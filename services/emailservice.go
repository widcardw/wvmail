package services

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// The service itself and the message-list path: the cache-first paging and the
// full refresh behind the mail list. The rest of this service lives in the
// sibling files of this package — emailfolders.go (mailbox tree, unseen counts),
// emailbody.go (one message), emailattachments.go, emailsearch.go,
// emaildecode.go and imapdial.go (the connection) — and the model types it
// exposes are declared here.

// EmailService provides methods for connecting to IMAP/SMTP email servers
// and managing email operations.
type EmailService struct {
	db     *DBManager
	health *accountHealthReporter

	// onUnseen, when set, receives the unseen count of every INBOX this service
	// fetches, so the tray badge follows real mail without a second connection.
	onUnseenMu sync.Mutex
	onUnseen   func(accountID string, unseen int)
}

func NewEmailService() *EmailService {
	return &EmailService{
		db:     NewDBManager(),
		health: newAccountHealthReporter(),
	}
}

// SetAccountHealthReporter replaces the health reporter. It is wired by main()
// so every mail operation emits through the same sequence and snapshot source.
//
//wails:ignore
func (s *EmailService) SetAccountHealthReporter(reporter *accountHealthReporter) {
	if reporter == nil {
		reporter = newAccountHealthReporter()
	}
	s.health = reporter
}

// AccountHealthEvents returns the bounded recent health-event history, oldest
// first. It is exposed on the service because Wails binds methods, not fields.
func (s *EmailService) AccountHealthEvents() []AccountHealthEvent {
	return s.health.events()
}

// setUnseenListener registers the callback that receives INBOX unseen counts.
// It is called from the goroutine doing the fetch and is package-private: it is
// Go wiring between EmailService and EmailSyncService, not a frontend API.
func (s *EmailService) setUnseenListener(listener func(accountID string, unseen int)) {
	s.onUnseenMu.Lock()
	defer s.onUnseenMu.Unlock()
	s.onUnseen = listener
}

func (s *EmailService) reportUnseen(accountID string, unseen int) {
	s.onUnseenMu.Lock()
	listener := s.onUnseen
	s.onUnseenMu.Unlock()
	if listener != nil {
		listener(accountID, unseen)
	}
}

// refreshUnseen updates the tray badge on an already-selected inbox. It is
// deliberately best-effort: the mail operation that triggered it has already
// succeeded, and a failed STATUS must not turn that into a user-facing error.
func (s *EmailService) refreshUnseen(client *imapclient.Client, accountID, folder string) {
	if !isInbox(folder) {
		return
	}
	status, err := client.Status(folder, &imap.StatusOptions{NumUnseen: true}).Wait()
	if err != nil {
		slog.Warn("sync: unseen count failed",
			"account_id", accountID, "folder", folder, "error", err)
		return
	}
	if status.NumUnseen != nil {
		s.reportUnseen(accountID, int(*status.NumUnseen))
	}
}

// EmailSummary is a lightweight representation of an email for list views.
type EmailSummary struct {
	UID        uint32   `json:"uid"`
	MessageID  string   `json:"message_id,omitempty"`
	Subject    string   `json:"subject"`
	From       string   `json:"from"`
	FromName   string   `json:"from_name"`
	FromAddr   string   `json:"from_addr"`
	To         []string `json:"to"`
	Date       string   `json:"date"`
	Flags      []string `json:"flags"`
	IsRead     bool     `json:"is_read"`
	TagIDs     []string `json:"tag_ids,omitempty"`
	AccountID  string   `json:"account_id,omitempty"`
	FolderName string   `json:"folder_name,omitempty"`
}

// EmailSearchFilter is one structured search tag. Supported fields are
// "from", "to", and "content".
type EmailSearchFilter struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// EmailContactSuggestion is a distinct sender or recipient found in the
// local email cache and used to complete a structured search tag.
type EmailContactSuggestion struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

// EmailSearchQuery searches the local email cache. Empty AccountID and
// FolderName values leave those dimensions unfiltered.
type EmailSearchQuery struct {
	AccountID  string              `json:"account_id"`
	FolderName string              `json:"folder_name"`
	Filters    []EmailSearchFilter `json:"filters"`
}

// AttachmentInfo describes a single email attachment discovered in the MIME
// body structure.
type AttachmentInfo struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     uint32 `json:"size"`
	PartPath []int  `json:"part_path"`
	IsInline bool   `json:"is_inline"`
}

// EmailDetail is the full representation of an email including body content.
type EmailDetail struct {
	UID       uint32   `json:"uid"`
	MessageID string   `json:"message_id,omitempty"`
	Subject   string   `json:"subject"`
	From      string   `json:"from"`
	FromName  string   `json:"from_name"`
	FromAddr  string   `json:"from_addr"`
	To        []string `json:"to"`
	Cc        []string `json:"cc"`
	Date      string   `json:"date"`
	BodyText  string   `json:"body_text"`
	BodyHTML  string   `json:"body_html"`
	// BodyIsPlainText records that the MIME body was text/plain (and no HTML
	// alternative was used). The frontend renders these bodies as Markdown.
	BodyIsPlainText bool             `json:"body_is_plain_text"`
	Flags           []string         `json:"flags"`
	Attachments     []AttachmentInfo `json:"attachments"`
	IsSuspicious    bool             `json:"is_suspicious"`
}

// FolderInfo represents a mail folder with its metadata.
type FolderInfo struct {
	Name  string   `json:"name"`
	Delim string   `json:"delim"`
	Attrs []string `json:"attrs"`
}

// EmailPageResponse is returned by ListEmails / RefreshEmails and contains
// the page of email summaries along with the total message count in the folder.
type EmailPageResponse struct {
	Emails []EmailSummary `json:"emails"`
	Total  int            `json:"total"` // total number of messages in the folder
	// FromCache tells the caller whether this page came from the local cache
	// without talking to the server. The frontend uses it to decide whether a
	// folder it just opened still needs revalidating (see openFolder in
	// stores/accounts.ts): a page that already came off the server must not be
	// fetched a second time in the same breath.
	FromCache bool `json:"from_cache"`
	// UIDNext is the mailbox's next UID at fetch time, 0 when the page came from
	// the cache. UIDs only ever grow, so the frontend can compare it against a
	// later STATUS to tell "there is new mail" from "nothing moved" without
	// fetching the list again (see revalidateOpenFolder in stores/accounts.ts).
	UIDNext uint32 `json:"uid_next"`
}

// ListEmails retrieves a page of email summaries from the specified folder.
// It serves the page from the local cache whenever the cache holds all of its
// rows; only an uncached page costs an IMAP round trip.
func (s *EmailService) ListEmails(account EmailAccount, folder string, page, pageSize int) (*EmailPageResponse, error) {
	page, pageSize = normalizePage(page, pageSize)

	cached, err := s.db.LoadEmails(account.ID, folder)
	if err == nil && len(cached) > 0 {
		meta, metaErr := s.db.LoadFolderMeta(account.ID, folder)
		// If no meta is stored yet (e.g. old cache before this feature), force a
		// refresh to get the real total from the server.
		if metaErr == nil && meta.Total > 0 {
			// Page 1 keeps its old behaviour (show whatever is cached), and later
			// pages are served from cache as soon as it can fill them: paging used
			// to always dial the server, which cost a full connect+login+fetch
			// (~0.5s) per page flip even when the rows were already stored.
			if page <= 1 || cacheCoversPage(page, pageSize, meta.Total, len(cached)) {
				return &EmailPageResponse{
					Emails:    paginateSlices(cached, page, pageSize),
					Total:     meta.Total,
					FromCache: true,
					// The UIDNEXT that fetch saw travels with the cached page: it is
					// what lets a folder click tell "nothing arrived" from "there is
					// new mail" without fetching again (see revalidateOpenFolder).
					UIDNext: meta.UIDNext,
				}, nil
			}
		}
	}

	return s.RefreshEmails(account, folder, page, pageSize)
}

// RefreshEmails fetches email summaries from the IMAP server and updates the
// cache.
func (s *EmailService) RefreshEmails(account EmailAccount, folder string, page, pageSize int) (*EmailPageResponse, error) {
	client, err := connectIMAP(account, 60*time.Second)
	if err != nil {
		return nil, err
	}
	defer client.Logout()

	return s.fetchEmailsOn(client, account, folder, page, pageSize)
}

// fetchEmailsOn is RefreshEmails on a connection the caller already owns. The
// IDLE watcher needs this: an idling connection cannot run another command, so
// the watcher stops IDLE and reuses that same session for the fetch instead of
// opening a second one for every arriving message (which is exactly the
// connection pressure IDLE is meant to remove).
func (s *EmailService) fetchEmailsOn(client *imapclient.Client, account EmailAccount, folder string, page, pageSize int) (*EmailPageResponse, error) {
	// Select the mailbox (read-only)
	mbox, err := selectMailbox(client, folder, true)
	if err != nil {
		return nil, err
	}

	// A successful SELECT reports NumMessages for the mailbox. QQ/Foxmail IMAP can
	// transiently report 0 for an otherwise non-empty Sent folder right after a
	// change; when we have a prior non-zero total for this exact folder, retry the
	// select once before trusting an empty result.
	priorMeta, _ := s.db.LoadFolderMeta(account.ID, folder)
	priorTotal := priorMeta.Total
	for retry := 0; mbox.NumMessages == 0 && priorTotal > 0 && retry < 1; retry++ {
		time.Sleep(300 * time.Millisecond)
		if rs, e2 := selectMailbox(client, folder, true); e2 == nil {
			mbox = rs
		}
	}

	// One extra command on the connection that is already open: the tray badge
	// needs the unseen count of the inbox, and a second IMAP session just for
	// STATUS would double both the connection cost and the provider's throttling.
	s.refreshUnseen(client, account.ID, folder)

	if mbox.NumMessages == 0 {
		if priorTotal > 0 {
			return nil, fmt.Errorf(
				"refresh reported 0 messages in %q while %d were cached — keeping previous list",
				folder, priorTotal,
			)
		}
		return &EmailPageResponse{Emails: []EmailSummary{}, Total: 0, UIDNext: uint32(mbox.UIDNext)}, nil
	}

	// Calculate the range for pagination
	total := int(mbox.NumMessages)
	page, pageSize = normalizePage(page, pageSize)
	start := total - (page-1)*pageSize
	end := total - page*pageSize + 1
	if end < 1 {
		end = 1
	}
	if start < 1 {
		return &EmailPageResponse{Emails: []EmailSummary{}, Total: total, UIDNext: uint32(mbox.UIDNext)}, nil
	}

	var seqSet imap.SeqSet
	if start != end {
		seqSet.AddRange(uint32(end), uint32(start))
	} else {
		seqSet.AddNum(uint32(end))
	}

	// Fetch envelope and flags
	fetchCmd := client.Fetch(seqSet, &imap.FetchOptions{
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		UID:          true,
	})

	buffers, err := fetchCmd.Collect()
	if err != nil {
		// Some servers (e.g. Tencent WeCom) return malformed ENVELOPE data.
		// Retry without Envelope and use default values for subject/from/date.
		slog.Warn("sync: envelope fetch failed, retrying without envelope",
			"account", account, "folder", folder, "error", err)
		fetchCmd.Close()
		fetchCmd = client.Fetch(seqSet, &imap.FetchOptions{
			Flags:        true,
			InternalDate: true,
			RFC822Size:   true,
			UID:          true,
		})
		buffers, err = fetchCmd.Collect()
		if err != nil {
			return nil, fmt.Errorf("fetch failed (even without envelope): %w", err)
		}
	}
	defer fetchCmd.Close()

	// Results come in ascending sequence order (oldest arrival first); the sort
	// after the loop turns them into newest-first.
	summaries := make([]EmailSummary, 0, len(buffers))
	for _, buf := range buffers {
		summaries = append(summaries, emailSummaryFromBuffer(buf))
	}

	// Newest first. The sequence range above selected the newest arrivals, but
	// arrival order is not date order (a forwarded message keeps its original
	// Date header), so the page is ordered on the parsed instant.
	sortEmailsNewestFirst(summaries)

	// Save meta + cache emails on every fetch. SaveEmails merges by UID without
	// removing older rows, so paging to an earlier page keeps those in the cache.
	if err := s.db.SaveFolderMeta(account.ID, folder, FolderMeta{
		Total:   total,
		UIDNext: uint32(mbox.UIDNext),
	}); err != nil {
		slog.Warn("cache: failed to cache folder meta", "account", account, "folder", folder, "error", err)
	}
	if err := s.db.SaveEmails(account.ID, folder, summaries); err != nil {
		slog.Warn("cache: failed to cache emails", "account", account, "folder", folder, "error", err)
	}

	// The IMAP request above only fetched the requested page. Return the page
	// from the merged cache for page 1, so callers get newly fetched messages
	// and previously cached older messages in one response. For later pages,
	// keep returning the IMAP page unless the cache has enough rows to rebuild
	// that page completely.
	responseEmails := summaries
	if cached, cacheErr := s.db.LoadEmails(account.ID, folder); cacheErr == nil && len(cached) > 0 {
		// SaveEmails intentionally upserts instead of replacing the folder, so
		// messages deleted in another client would otherwise stay in the cache
		// forever. Check only the cached UIDs: the cache is much smaller than a
		// large mailbox and this keeps reconciliation proportional to it.
		if pruned, pruneErr := s.pruneStaleCachedEmails(client, account, folder, cached); pruneErr != nil {
			slog.Warn("cache: failed to prune stale emails",
				"account", account, "folder", folder, "error", pruneErr)
		} else {
			cached = pruned
		}
		if page <= 1 || cacheCoversPage(page, pageSize, total, len(cached)) {
			responseEmails = paginateSlices(cached, page, pageSize)
		}
	}

	return &EmailPageResponse{
		Emails:  responseEmails,
		Total:   total,
		UIDNext: uint32(mbox.UIDNext),
	}, nil
}

// pruneStaleCachedEmails checks the cached UIDs against the selected mailbox.
// It asks only about cached UIDs rather than fetching SEARCH ALL, so the cost is
// proportional to the pages this client has actually stored.
func (s *EmailService) pruneStaleCachedEmails(
	client *imapclient.Client,
	account EmailAccount,
	folder string,
	cached []EmailSummary,
) ([]EmailSummary, error) {
	if len(cached) == 0 {
		return cached, nil
	}

	var cachedUIDs imap.UIDSet
	for _, email := range cached {
		cachedUIDs.AddNum(imap.UID(email.UID))
	}
	data, err := client.UIDSearch(&imap.SearchCriteria{
		UID: []imap.UIDSet{cachedUIDs},
	}, nil).Wait()
	if err != nil {
		return cached, fmt.Errorf("searching cached UIDs failed: %w", err)
	}

	existing := make(map[uint32]struct{}, len(data.AllUIDs()))
	for _, uid := range data.AllUIDs() {
		existing[uint32(uid)] = struct{}{}
	}

	stale := make([]uint32, 0)
	kept := make([]EmailSummary, 0, len(cached))
	for _, email := range cached {
		if _, ok := existing[email.UID]; ok {
			kept = append(kept, email)
		} else {
			stale = append(stale, email.UID)
		}
	}
	if len(stale) == 0 {
		return cached, nil
	}
	if err := s.db.PruneEmails(account.ID, folder, stale); err != nil {
		return cached, fmt.Errorf("removing stale cached emails failed: %w", err)
	}
	slog.Debug("cache: pruned stale emails",
		"account", account, "folder", folder, "count", len(stale))
	return kept, nil
}

// emailSummaryFromBuffer is shared by the folder page fetch and the flagged-mail
// search. Keeping one conversion means the two paths cannot disagree about
// sender formatting, dates, or read state.
func emailSummaryFromBuffer(buf *imapclient.FetchMessageBuffer) EmailSummary {
	summary := EmailSummary{
		UID:   uint32(buf.UID),
		Flags: make([]string, len(buf.Flags)),
	}
	var envelopeDate time.Time
	if buf.Envelope != nil {
		summary.Subject = decodeMimeHeader(buf.Envelope.Subject)
		summary.MessageID = strings.TrimSpace(buf.Envelope.MessageID)
		envelopeDate = buf.Envelope.Date
	}
	// Stored in UTC so the column is homogeneous. Date and InternalDate are
	// validated together because malformed Date headers do occur in real mail.
	summary.Date = resolveEmailDate(envelopeDate, buf.InternalDate, time.Now())
	if summary.Subject == "" {
		summary.Subject = "(No Subject)"
	}
	for i, f := range buf.Flags {
		summary.Flags[i] = string(f)
		if f == imap.FlagSeen {
			summary.IsRead = true
		}
	}
	if buf.Envelope != nil {
		summary.From, summary.FromName, summary.FromAddr = envelopeSender(buf.Envelope.From)
		summary.To = envelopeAddresses(buf.Envelope.To)
	}
	return summary
}

// PurgeAccountCache drops everything cached for one account and reports how many
// rows went away.
//
// It is separate from ConfigService.RemoveAccount on purpose: the config write
// decides whether the account exists, and this only decides how much of it stays
// on disk. The settings page calls it after a successful removal, and only when
// the user ticked "also delete cached mail" — the cache is what makes the app work
// offline, so throwing it away is never implicit.
func (s *EmailService) PurgeAccountCache(accountID string) (int64, error) {
	deleted, err := s.db.DeleteAccountCache(accountID)
	if err != nil {
		slog.Error("cache: purging account failed", "account_id", accountID, "error", err)
		return 0, err
	}
	slog.Info("cache: purged account", "account_id", accountID, "rows", deleted)
	return deleted, nil
}
