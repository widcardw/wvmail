package services

import (
	"log/slog"
	"time"

	"github.com/emersion/go-imap/v2/imapclient"
)

// Conversation-aware paging for the folder list.
//
// The folder list renders one row per conversation, while IMAP and the cache
// store one row per message. Paging the messages directly made a conversation
// that crossed a page boundary appear on both pages (and left the page count
// describing messages rather than rows). The functions here page the grouped
// cache instead and fetch older message pages only when the current page does
// not yet hold enough conversations.

// threadRows collapses newest-first summaries to one row per ThreadID. The
// first message of a group is its newest message, which is what the list shows.
func threadRows(emails []EmailSummary) []EmailSummary {
	rows := make([]EmailSummary, 0, len(emails))
	seen := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		id := email.ThreadID
		if id == "" {
			rows = append(rows, email)
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, email)
	}
	return rows
}

func threadPageResponse(
	emails []EmailSummary,
	total int,
	uidNext uint32,
	page, pageSize int,
	fromCache bool,
) *EmailPageResponse {
	rows := threadRows(emails)
	return &EmailPageResponse{
		Emails:      paginateSlices(rows, page, pageSize),
		Total:       total,
		ThreadTotal: len(rows),
		FromCache:   fromCache,
		UIDNext:     uidNext,
	}
}

// threadPageIsReady reports whether the cache can answer this conversation
// page without another IMAP fetch. When the cache is incomplete, one extra row
// beyond the page boundary is required: it proves that a next page exists and
// keeps the frontend from hiding it behind a stale ThreadTotal.
func threadPageIsReady(cached []EmailSummary, total, page, pageSize int) bool {
	if len(cached) >= total {
		return true
	}
	return len(threadRows(cached)) > page*pageSize
}

// ListThreadPage returns one page of conversations, newest first. It is the
// folder-list counterpart to ListEmails: the same cache, but one row per
// conversation and a ThreadTotal the pager can use.
func (s *EmailService) ListThreadPage(account EmailAccount, folder string, page, pageSize int) (*EmailPageResponse, error) {
	page, pageSize = normalizePage(page, pageSize)

	cached, err := s.db.LoadEmails(account.ID, folder)
	if err == nil && len(cached) > 0 {
		meta, metaErr := s.db.LoadFolderMeta(account.ID, folder)
		if metaErr == nil && meta.Total > 0 && threadPageIsReady(cached, meta.Total, page, pageSize) {
			return threadPageResponse(cached, meta.Total, meta.UIDNext, page, pageSize, true), nil
		}
	}

	return s.RefreshThreadPage(account, folder, page, pageSize)
}

// RefreshThreadPage refreshes the newest page and then fills enough cached
// messages to answer the requested conversation page.
func (s *EmailService) RefreshThreadPage(account EmailAccount, folder string, page, pageSize int) (*EmailPageResponse, error) {
	client, err := connectIMAP(account, 60*time.Second)
	if err != nil {
		return nil, err
	}
	defer client.Logout()

	return s.threadPageOn(client, account, folder, page, pageSize, true)
}

func (s *EmailService) threadPageOn(
	client *imapclient.Client,
	account EmailAccount,
	folder string,
	page, pageSize int,
	refreshNewest bool,
) (*EmailPageResponse, error) {
	page, pageSize = normalizePage(page, pageSize)

	nextPage := 1
	if refreshNewest {
		if _, err := s.fetchAndCacheMessagePage(client, account, folder, 1, pageSize); err != nil {
			return nil, err
		}
		// Match the message-oriented refresh: drop cached rows the server no
		// longer has before grouping, otherwise a message deleted elsewhere
		// would keep its conversation alive in this list.
		if cached, err := s.db.LoadEmails(account.ID, folder); err == nil && len(cached) > 0 {
			if _, pruneErr := s.pruneStaleCachedEmails(client, account, folder, cached); pruneErr != nil {
				slog.Warn("cache: failed to prune stale emails",
					"account", account, "folder", folder, "error", pruneErr)
			}
		}
		nextPage = 2
		// The folder pager normally fills the cache from newest to oldest. Skip
		// the pages that refresh just left there instead of re-fetching them;
		// `attempted` below still walks forward if a page was already cached.
		if cached, err := s.db.LoadEmails(account.ID, folder); err == nil && len(cached) >= pageSize {
			nextPage = len(cached)/pageSize + 1
			if nextPage < 2 {
				nextPage = 2
			}
		}
	}

	attempted := make(map[int]struct{})
	for {
		cached, err := s.db.LoadEmails(account.ID, folder)
		if err != nil {
			return nil, err
		}
		meta, err := s.db.LoadFolderMeta(account.ID, folder)
		if err != nil {
			return nil, err
		}

		rows := threadRows(cached)
		need := page * pageSize
		complete := meta.Total == 0 || len(cached) >= meta.Total
		if complete || len(rows) > need {
			return threadPageResponse(cached, meta.Total, meta.UIDNext, page, pageSize, false), nil
		}

		totalPages := 1
		if meta.Total > 0 {
			totalPages = (meta.Total + pageSize - 1) / pageSize
		}
		// The cache normally holds the newest pages in order, so the next page
		// after the cached prefix is the one to fetch. `attempted` also lets us
		// walk past a page that was already cached (a user can jump pages).
		if nextPage < 1 {
			nextPage = 1
		}
		for {
			if _, ok := attempted[nextPage]; !ok {
				break
			}
			nextPage++
		}
		if nextPage > totalPages {
			return threadPageResponse(cached, meta.Total, meta.UIDNext, page, pageSize, false), nil
		}

		attempted[nextPage] = struct{}{}
		if _, err := s.fetchAndCacheMessagePage(client, account, folder, nextPage, pageSize); err != nil {
			return nil, err
		}
		nextPage++
	}
}
