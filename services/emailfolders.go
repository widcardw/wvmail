package services

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// Folder listing and per-folder state: the mailbox tree, unseen counts (the
// sidebar badges and the tray badge), and the read/unread flag writes.

// ListFolders retrieves all available IMAP mailboxes/folders.
// Returns cached data if available and no force refresh requested.
func (s *EmailService) ListFolders(account EmailAccount) ([]FolderInfo, error) {
	folders, _, err := s.loadFolderList(account)
	return folders, err
}

// loadFolderList returns the cached folder list, refreshing from the server only
// when the cache is empty. It reports whether it refreshed so a caller that
// still needs a Trash lookup can avoid a second dial.
//
// DeleteEmail, FolderStatuses, the sync loop and the IDLE watcher all used to
// repeat this read-cache-then-refresh block inline.
func (s *EmailService) loadFolderList(account EmailAccount) ([]FolderInfo, bool, error) {
	if cached, err := s.db.LoadFolders(account.ID); err == nil && len(cached) > 0 {
		return cached, false, nil
	}
	folders, err := s.RefreshFolders(account)
	if err != nil {
		return nil, false, err
	}
	return folders, true, nil
}

// RefreshFolders fetches folders from the IMAP server and updates the cache.
func (s *EmailService) RefreshFolders(account EmailAccount) ([]FolderInfo, error) {
	client, err := connectIMAP(account, 60*time.Second)
	if err != nil {
		return nil, err
	}
	defer client.Logout()

	listCmd := client.List("", "*", nil)
	defer listCmd.Close()

	listData, err := listCmd.Collect()
	if err != nil {
		return nil, fmt.Errorf("list failed: %w", err)
	}

	folders := make([]FolderInfo, 0, len(listData))
	for _, ld := range listData {
		attrs := make([]string, len(ld.Attrs))
		for i, a := range ld.Attrs {
			attrs[i] = string(a)
		}
		folders = append(folders, FolderInfo{
			Name:  ld.Mailbox,
			Delim: string(ld.Delim),
			Attrs: attrs,
		})
	}

	// Update cache
	if err := s.db.SaveFolders(account.ID, folders); err != nil {
		// Log but don't fail — cache is best-effort
		slog.Warn("cache: failed to cache folders", "account", account, "error", err)
	}

	return folders, nil
}

// UnseenCount asks the server how many messages in folder are still unseen
// (STATUS … UNSEEN). The tray badge must be right while the window is hidden, so
// it cannot be derived from whichever page the UI happens to have loaded; a
// STATUS round trip is one cheap command that does not fetch any message.
func (s *EmailService) UnseenCount(account EmailAccount, folder string) (int, error) {
	client, err := connectIMAP(account, 60*time.Second)
	if err != nil {
		return 0, err
	}
	defer client.Logout()

	data, err := client.Status(folder, &imap.StatusOptions{NumUnseen: true}).Wait()
	if err != nil {
		return 0, fmt.Errorf("status failed: %w", err)
	}
	if data.NumUnseen == nil {
		// A server that answers without UNSEEN is treated as "nothing to badge".
		return 0, nil
	}
	return int(*data.NumUnseen), nil
}

// isSelectableMailbox reports whether a mailbox can be opened, and therefore STATUSed.
// A \Noselect / \NonExistent branch only exists to hold child folders: it has no
// messages of its own, and asking for its status is an error rather than a zero.
func isSelectableMailbox(attrs []string) bool {
	for _, a := range attrs {
		lower := strings.ToLower(a)
		if strings.Contains(lower, "noselect") || strings.Contains(lower, "nonexistent") {
			return false
		}
	}
	return true
}

// FolderStatus is one folder's counters and special-use attributes as the
// server reported them. The frontend uses Attrs to avoid double-counting Gmail's
// overlapping system views (INBOX, Important, All Mail).
//
// Unseen drives the sidebar badge. UIDNext and Messages are the change signal: a
// message arriving moves UIDNext whether or not it is unread, which UNSEEN alone
// cannot tell you (a mailbox with three permanently unread newsletters keeps
// UNSEEN at 3 forever). The frontend compares them against what the last fetch
// saw before deciding to refresh a folder at all (see revalidateOpenFolder in
// stores/accounts.ts).
type FolderStatus struct {
	Name     string   `json:"name"`
	Attrs    []string `json:"attrs"`
	Messages int      `json:"messages"`
	UIDNext  uint32   `json:"uid_next"`
	Unseen   int      `json:"unseen"`
}

// FolderStatuses returns the per-folder counters (STATUS … MESSAGES UIDNEXT
// UNSEEN) so the sidebar can badge every folder and the list can tell whether a
// folder moved since it was last fetched.
//
// The counts cannot come from the local cache: only the pages the UI happened to
// load are stored, so anything derived from it would quietly undercount a large
// mailbox. One connection is opened and every STATUS is issued before any is
// awaited, so the whole folder list costs a single round trip instead of one per
// folder (go-imap pipelines commands on one connection).
//
// When no folder list is cached yet (a fresh account, or the startup tray seed has
// not finished), the list is fetched first so the caller's request still answers
// with real counts instead of an empty map.
//
// Folders that are not selectable, or that the server refuses to answer for, are
// simply absent from the map — the caller reads a missing key as "nothing to
// badge", and as "unknown" when deciding whether to refresh.
func (s *EmailService) FolderStatuses(account EmailAccount) (map[string]FolderStatus, error) {
	folders, _, err := s.loadFolderList(account)
	if err != nil {
		s.health.report(account.ID, AccountHealthSourceStatus, AccountHealthOutcomeFailure)
		return nil, err
	}

	client, err := connectIMAP(account, 60*time.Second)
	if err != nil {
		s.health.report(account.ID, AccountHealthSourceStatus, AccountHealthOutcomeFailure)
		return nil, err
	}
	defer client.Logout()

	type statusJob struct {
		name  string
		attrs []string
		cmd   *imapclient.StatusCommand
	}
	jobs := make([]statusJob, 0, len(folders))
	for _, f := range folders {
		if !isSelectableMailbox(f.Attrs) {
			continue
		}
		jobs = append(jobs, statusJob{
			name:  f.Name,
			attrs: f.Attrs,
			cmd: client.Status(f.Name, &imap.StatusOptions{
				NumMessages: true,
				UIDNext:     true,
				NumUnseen:   true,
			}),
		})
	}

	statuses := make(map[string]FolderStatus, len(jobs))
	for _, job := range jobs {
		data, err := job.cmd.Wait()
		if err != nil {
			// One mailbox the server will not talk about must not cost the others
			// their badges.
			slog.Debug("sync: folder status failed",
				"account", account, "folder", job.name, "error", err)
			continue
		}
		status := FolderStatus{
			Name:    job.name,
			Attrs:   job.attrs,
			UIDNext: uint32(data.UIDNext),
		}
		if data.NumMessages != nil {
			status.Messages = int(*data.NumMessages)
		}
		if data.NumUnseen != nil {
			status.Unseen = int(*data.NumUnseen)
		}
		statuses[job.name] = status
	}
	s.health.report(account.ID, AccountHealthSourceStatus, AccountHealthOutcomeOK)
	return statuses, nil
}

// MarkEmailRead flags one message as seen, in the local cache and on the server.
// Opening a message must not leave it unread forever: the list styling and the
// tray badge both follow this state, and providers that answer STATUS UNSEEN keep
// reporting the message until the flag is actually stored.
//
// The cache is updated first so the UI is right even if the server call fails; the
// error return then tells the caller whether the server agreed.
func (s *EmailService) MarkEmailRead(account EmailAccount, folder string, uid uint32) error {
	if err := s.db.MarkEmailRead(account.ID, folder, uid); err != nil {
		slog.Warn("cache: failed to mark email read in cache",
			"account", account, "folder", folder, "uid", uid, "error", err)
	}

	client, err := connectIMAP(account, 60*time.Second)
	if err != nil {
		return err
	}
	defer client.Logout()

	// STORE needs the mailbox opened read-write.
	if _, err := selectMailbox(client, folder, false); err != nil {
		return err
	}
	store := &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagSeen}}
	if err := client.Store(imap.UIDSetNum(imap.UID(uid)), store, nil).Close(); err != nil {
		return wrapIMAPError("marking the message read failed", err)
	}

	// Keep the tray badge honest without waiting for the next inbox fetch: the
	// connection is already open, so one STATUS is enough.
	s.refreshUnseen(client, account.ID, folder)
	return nil
}
