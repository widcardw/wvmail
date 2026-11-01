package services

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// Flagged mail is an IMAP-wide concept, not a provider API: \Flagged is one of
// the standard system flags (RFC 9051), and SEARCH FLAGGED works on Gmail,
// Outlook, QQ, 163 and generic IMAP servers. The aggregate Starred view is built
// from that standard search rather than a provider-specific "Starred" folder.

const flaggedFetchLimitPerAccount = 500

const flaggedSnapshotFreshFor = 2 * time.Minute

// FlaggedEmailSnapshot is the local All Starred result for one account.
// FetchedAt is Unix milliseconds; zero means the rows were invalidated by a
// star change and need a refresh before they can be trusted.
type FlaggedEmailSnapshot struct {
	Emails    []EmailSummary `json:"emails"`
	FetchedAt int64          `json:"fetched_at"`
	FromCache bool           `json:"from_cache"`
	Stale     bool           `json:"stale"`
}

// ListFlaggedEmails returns an account's All Starred snapshot. A normal open
// reads SQLite only and never waits for IMAP; callers pass refresh=true when the
// snapshot is stale or the user explicitly asks for a server refresh.
//
// A server that exposes a special-use \Flagged mailbox is searched there only:
// Gmail's Starred folder already aggregates every label, so walking every
// mailbox would return the same message several times. A \All virtual mailbox is
// the next best choice. Otherwise every selectable mailbox except Junk and Trash
// is searched. The limit bounds the metadata fetch on accounts with years of
// starred mail; the newest 500 are what a first version can render without
// turning one click into hundreds of envelopes.
func (s *EmailService) ListFlaggedEmails(account EmailAccount, refresh bool) (*FlaggedEmailSnapshot, error) {
	if !refresh {
		emails, fetchedAt, err := s.db.LoadFlaggedSnapshot(account.ID)
		if err != nil {
			return nil, fmt.Errorf("loading flagged snapshot failed: %w", err)
		}
		if emails == nil {
			emails = []EmailSummary{}
		}
		return &FlaggedEmailSnapshot{
			Emails:    emails,
			FetchedAt: fetchedAt,
			FromCache: true,
			Stale:     fetchedAt == 0 || time.Since(time.UnixMilli(fetchedAt)) >= flaggedSnapshotFreshFor,
		}, nil
	}

	folders, err := s.db.LoadFolders(account.ID)
	if err != nil || len(folders) == 0 {
		// RefreshFolders already opens its own connection. Reuse that answer
		// instead of dialing a second time just to read what it cached.
		if folders, err = s.RefreshFolders(account); err != nil {
			return nil, fmt.Errorf("loading folders failed: %w", err)
		}
	}

	client, err := connectIMAP(account, 60*time.Second)
	if err != nil {
		return nil, err
	}
	defer client.Logout()

	targets := flaggedSearchFolders(folders)
	if len(targets) == 0 {
		return s.saveFlaggedSnapshot(account.ID, []EmailSummary{})
	}

	byKey := make(map[string]EmailSummary)
	var firstErr error
	for _, folder := range targets {
		results, err := searchFlaggedFolder(client, account, folder.Name)
		if err != nil {
			slog.Warn("flagged: search failed",
				"account", account, "folder", folder.Name, "error", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, summary := range results {
			summary.AccountID = account.ID
			summary.FolderName = folder.Name
			key := flaggedMessageKey(summary)
			if existing, ok := byKey[key]; ok {
				if newestFirst(summary, existing) {
					byKey[key] = summary
				}
				continue
			}
			byKey[key] = summary
		}
	}
	if len(byKey) == 0 && firstErr != nil {
		return nil, firstErr
	}

	emails := make([]EmailSummary, 0, len(byKey))
	for _, summary := range byKey {
		emails = append(emails, summary)
	}
	sortEmailsNewestFirst(emails)
	if len(emails) > flaggedFetchLimitPerAccount {
		emails = emails[:flaggedFetchLimitPerAccount]
	}
	return s.saveFlaggedSnapshot(account.ID, emails)
}

func (s *EmailService) saveFlaggedSnapshot(accountID string, emails []EmailSummary) (*FlaggedEmailSnapshot, error) {
	fetchedAt := time.Now().UnixMilli()
	if err := s.db.SaveFlaggedSnapshot(accountID, emails, fetchedAt); err != nil {
		// The server search succeeded, so a cache write failure must not turn the
		// refresh into an error. The next open simply searches again.
		slog.Warn("cache: failed to save flagged snapshot", "account", accountID, "error", err)
	}
	return &FlaggedEmailSnapshot{
		Emails:    emails,
		FetchedAt: fetchedAt,
		FromCache: false,
		Stale:     false,
	}, nil
}

func flaggedMessageKey(summary EmailSummary) string {
	if summary.MessageID != "" {
		return "message-id\x00" + strings.ToLower(summary.MessageID)
	}
	return fmt.Sprintf("folder\x00%s\x00%d", summary.FolderName, summary.UID)
}

// SetEmailFlagged adds or removes \Flagged on the server, then mirrors the
// accepted result into the summary and body caches.
func (s *EmailService) SetEmailFlagged(account EmailAccount, folder string, uid uint32, flagged bool) error {
	client, err := connectIMAP(account, 60*time.Second)
	if err != nil {
		return err
	}
	defer client.Logout()

	if _, err := selectMailbox(client, folder, false); err != nil {
		return err
	}
	op := imap.StoreFlagsDel
	if flagged {
		op = imap.StoreFlagsAdd
	}
	store := &imap.StoreFlags{
		Op:     op,
		Silent: true,
		Flags:  []imap.Flag{imap.FlagFlagged},
	}
	if err := client.Store(imap.UIDSetNum(imap.UID(uid)), store, nil).Close(); err != nil {
		return wrapIMAPError("updating the star failed", err)
	}
	if err := s.db.SetEmailFlagged(account.ID, folder, uid, flagged); err != nil {
		// The server is authoritative and the next fetch repairs the cache, so a
		// cache write failure must not turn a successful STORE into an error.
		slog.Warn("cache: failed to update flagged state",
			"account", account, "folder", folder, "uid", uid, "error", err)
	}
	if err := s.db.InvalidateFlaggedSnapshot(account.ID); err != nil {
		slog.Warn("cache: failed to invalidate flagged snapshot",
			"account", account, "error", err)
	}
	return nil
}

func searchFlaggedFolder(client *imapclient.Client, account EmailAccount, folder string) ([]EmailSummary, error) {
	if _, err := selectMailbox(client, folder, true); err != nil {
		return nil, err
	}
	data, err := client.UIDSearch(&imap.SearchCriteria{
		Flag: []imap.Flag{imap.FlagFlagged},
	}, nil).Wait()
	if err != nil {
		return nil, wrapIMAPError("search flagged mail failed", err)
	}
	uids := data.AllUIDs()
	if len(uids) == 0 {
		return []EmailSummary{}, nil
	}
	sort.Slice(uids, func(i, j int) bool { return uids[i] > uids[j] })
	if len(uids) > flaggedFetchLimitPerAccount {
		uids = uids[:flaggedFetchLimitPerAccount]
	}

	var uidSet imap.UIDSet
	uidSet.AddNum(uids...)
	fetchCmd := client.Fetch(uidSet, &imap.FetchOptions{
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		UID:          true,
	})
	defer fetchCmd.Close()
	buffers, err := fetchCmd.Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch flagged mail failed: %w", err)
	}

	summaries := make([]EmailSummary, 0, len(buffers))
	for _, buf := range buffers {
		summary := emailSummaryFromBuffer(buf)
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func flaggedSearchFolders(folders []FolderInfo) []FolderInfo {
	flagged := make([]FolderInfo, 0, 1)
	all := make([]FolderInfo, 0, 1)
	for _, folder := range folders {
		if !isSelectableMailbox(folder.Attrs) || isJunkOrTrashFolder(folder) {
			continue
		}
		switch folderRole(folder.Attrs) {
		case string(imap.MailboxAttrFlagged):
			flagged = append(flagged, folder)
		case string(imap.MailboxAttrAll):
			all = append(all, folder)
		}
	}
	if len(flagged) > 0 {
		return flagged
	}
	if len(all) > 0 {
		return all
	}

	targets := make([]FolderInfo, 0, len(folders))
	for _, folder := range folders {
		if isSelectableMailbox(folder.Attrs) && !isJunkOrTrashFolder(folder) {
			targets = append(targets, folder)
		}
	}
	return targets
}

func folderRole(attrs []string) string {
	for _, attr := range attrs {
		switch strings.ToLower(attr) {
		case strings.ToLower(string(imap.MailboxAttrFlagged)):
			return string(imap.MailboxAttrFlagged)
		case strings.ToLower(string(imap.MailboxAttrAll)):
			return string(imap.MailboxAttrAll)
		case strings.ToLower(string(imap.MailboxAttrJunk)):
			return string(imap.MailboxAttrJunk)
		case strings.ToLower(string(imap.MailboxAttrTrash)):
			return string(imap.MailboxAttrTrash)
		}
	}
	return ""
}

func isJunkOrTrashFolder(folder FolderInfo) bool {
	switch folderRole(folder.Attrs) {
	case string(imap.MailboxAttrJunk), string(imap.MailboxAttrTrash):
		return true
	}
	name := strings.ToLower(strings.TrimSpace(folder.Name))
	return strings.Contains(name, "junk") ||
		strings.Contains(name, "spam") ||
		strings.Contains(name, "trash") ||
		strings.Contains(name, "deleted")
}
