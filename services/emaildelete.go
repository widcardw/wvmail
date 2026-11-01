package services

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// DeleteEmail moves a message to the account's Trash mailbox. Deleting a message
// that is already in Trash is permanent, which matches the usual mail-client
// behaviour and avoids leaving a message forever in the folder meant for removal.
//
// go-imap's MOVE implementation already falls back to COPY + \Deleted + EXPUNGE
// on servers without the MOVE extension, so providers only need to expose their
// standard Trash mailbox.
func (s *EmailService) DeleteEmail(account EmailAccount, folder string, uid uint32) error {
	folders, err := s.db.LoadFolders(account.ID)
	if err != nil {
		return fmt.Errorf("loading folders failed: %w", err)
	}
	refreshed := false
	if len(folders) == 0 {
		if folders, err = s.RefreshFolders(account); err != nil {
			return fmt.Errorf("loading folders failed: %w", err)
		}
		refreshed = true
	}
	trash, ok := findTrashFolder(folders)
	if !ok && !refreshed {
		// The folder cache can predate the mailbox list gaining a Trash folder
		// (or can have been written by an older provider response). Refresh once
		// before concluding that the account has nowhere to move the message.
		if folders, err = s.RefreshFolders(account); err != nil {
			return fmt.Errorf("loading folders failed: %w", err)
		}
		trash, ok = findTrashFolder(folders)
	}
	if !ok {
		return fmt.Errorf("delete failed: no Trash mailbox found for this account")
	}

	client, err := connectIMAP(account, 60*time.Second)
	if err != nil {
		return err
	}
	defer client.Logout()

	if _, err := selectMailbox(client, folder, false); err != nil {
		return err
	}

	uidSet := imap.UIDSetNum(imap.UID(uid))
	found, err := client.UIDSearch(&imap.SearchCriteria{
		UID: []imap.UIDSet{uidSet},
	}, nil).Wait()
	if err != nil {
		return wrapIMAPError("checking the message failed", err)
	}
	if len(found.AllUIDs()) == 0 {
		// The cache can outlive a message deleted or moved by another client.
		// Treat that as a successful cleanup of the stale local row instead of
		// sending MOVE/STORE for a UID the server no longer has.
		s.finalizeEmailRemoval(client, account, folder, uid, false)
		return nil
	}

	if strings.EqualFold(folder, trash.Name) {
		if err := deletePermanently(client, uidSet); err != nil {
			return wrapIMAPError("permanently deleting the message failed", err)
		}
	} else if _, err := client.Move(uidSet, trash.Name).Wait(); err != nil {
		return wrapIMAPError("moving the message to Trash failed", err)
	}

	s.finalizeEmailRemoval(client, account, folder, uid, true)
	return nil
}

// finalizeEmailRemoval mirrors an accepted removal into the local cache. A
// stale UID that was already absent on the server must not decrement the cached
// folder total, because the server total was already correct.
func (s *EmailService) finalizeEmailRemoval(
	client *imapclient.Client,
	account EmailAccount,
	folder string,
	uid uint32,
	serverChanged bool,
) {
	var err error
	if serverChanged {
		err = s.db.DeleteEmail(account.ID, folder, uid)
	} else {
		err = s.db.PruneEmails(account.ID, folder, []uint32{uid})
	}
	if err != nil {
		// The server is authoritative; a stale cache row is preferable to telling
		// the user the delete failed after it already happened.
		slog.Warn("cache: failed to remove deleted email",
			"account", account, "folder", folder, "uid", uid, "error", err)
	}
	if err := s.db.InvalidateFlaggedSnapshot(account.ID); err != nil {
		slog.Warn("cache: failed to invalidate flagged snapshot after delete",
			"account", account, "error", err)
	}

	// Deleting an unread inbox message changes the tray badge. The mailbox is
	// already selected, so one STATUS is enough.
	s.refreshUnseen(client, account.ID, folder)
}

func deletePermanently(client *imapclient.Client, uidSet imap.UIDSet) error {
	store := &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Silent: true,
		Flags:  []imap.Flag{imap.FlagDeleted},
	}
	if err := client.Store(uidSet, store, nil).Close(); err != nil {
		return err
	}
	if client.Caps().Has(imap.CapUIDPlus) {
		return client.UIDExpunge(uidSet).Close()
	}
	return client.Expunge().Close()
}

func findTrashFolder(folders []FolderInfo) (FolderInfo, bool) {
	for _, folder := range folders {
		if isSelectableMailbox(folder.Attrs) && folderRole(folder.Attrs) == string(imap.MailboxAttrTrash) {
			return folder, true
		}
	}

	for _, folder := range folders {
		if !isSelectableMailbox(folder.Attrs) || folderRole(folder.Attrs) == string(imap.MailboxAttrJunk) {
			continue
		}
		name := folder.Name
		if folder.Delim != "" {
			if index := strings.LastIndex(name, folder.Delim); index >= 0 {
				name = name[index+len(folder.Delim):]
			}
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "trash", "deleted", "deleted items", "deleted messages", "bin",
			"已删除", "已删除邮件", "垃圾箱", "废纸篓":
			return folder, true
		}
	}
	return FolderInfo{}, false
}
