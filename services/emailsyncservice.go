package services

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// EmailSyncService manages periodic email syncing for all accounts based on
// each account's SyncInterval setting. It runs a background loop that checks
// every 30 seconds and syncs accounts whose sync interval has elapsed.
//
// It is the fallback path, not the primary one: accounts with a live IDLE
// connection (see emailidle.go) are skipped here, because the server already
// tells us when their inbox changes. Everything below stays as the safety net for
// servers that refuse IDLE, for accounts that are reconnecting, and for the
// periods when no watcher is up.
type EmailSyncService struct {
	config   *ConfigService
	email    *EmailService
	idle     *EmailIdleService
	mu       sync.Mutex
	lastSync map[string]time.Time // accountID → last sync time
	stopCh   chan struct{}

	// unread tracks the per-account unseen counts behind the tray badge.
	unread     unreadTracker
	unreadMu   sync.Mutex
	lastUnread bool
	onUnread   func(hasUnread bool)
}

func NewEmailSyncService(cfg *ConfigService, email *EmailService) *EmailSyncService {
	svc := &EmailSyncService{
		config:   cfg,
		email:    email,
		lastSync: make(map[string]time.Time),
		stopCh:   make(chan struct{}),
		unread:   newUnreadTracker(),
	}
	// Every INBOX fetch reports its unseen count on the connection it already has
	// open, which is what keeps the tray badge current without extra sessions.
	if email != nil {
		email.SetUnseenListener(func(accountID string, unseen int) {
			svc.publishUnread(svc.unread.observe(accountID, unseen))
		})
	}
	return svc
}

// SetUnreadListener registers the callback that swaps the tray icon. It is called
// only when the aggregate flips, and from whichever goroutine finished the sync,
// so the callback must be safe to call off the UI thread (SystemTray.SetIcon is).
func (s *EmailSyncService) SetUnreadListener(listener func(hasUnread bool)) {
	s.unreadMu.Lock()
	defer s.unreadMu.Unlock()
	s.onUnread = listener
}

// unreadTracker remembers the last unseen count per account. An account that
// fails to answer keeps its previous value (see refreshUnread), and accounts that
// disappear from the config are dropped, so the badge cannot get stuck on.
type unreadTracker struct {
	mu     sync.Mutex
	counts map[string]int
}

func newUnreadTracker() unreadTracker {
	return unreadTracker{counts: make(map[string]int)}
}

// observe records an account's unseen count and reports whether anything is still
// unread across all accounts.
func (t *unreadTracker) observe(accountID string, unseen int) bool {
	if unseen < 0 {
		unseen = 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[accountID] = unseen
	return t.hasUnreadLocked()
}

// drop forgets one account and reports the aggregate that is left.
func (t *unreadTracker) drop(accountID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.counts, accountID)
	return t.hasUnreadLocked()
}

// retain drops accounts that are no longer configured and reports the aggregate.
func (t *unreadTracker) retain(accountIDs []string) bool {
	keep := make(map[string]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		keep[id] = struct{}{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for id := range t.counts {
		if _, ok := keep[id]; !ok {
			delete(t.counts, id)
		}
	}
	return t.hasUnreadLocked()
}

func (t *unreadTracker) hasUnreadLocked() bool {
	for _, unseen := range t.counts {
		if unseen > 0 {
			return true
		}
	}
	return false
}

// refreshUnread re-reads one account's unseen count. It is used at startup, where
// no folder fetch has happened yet; during normal operation the fetch itself
// reports the count (see SetUnseenListener).
func (s *EmailSyncService) refreshUnread(account EmailAccount, inbox string) {
	unseen, err := s.email.UnseenCount(account, inbox)
	if err != nil {
		// Keep the previous value: a transient IMAP error must not clear the badge.
		slog.Warn("tray: unseen count failed, keeping previous value",
			"account", account, "error", err)
		return
	}
	slog.Info("tray: inbox unseen count", "account", account, "unseen", unseen)
	s.publishUnread(s.unread.observe(account.ID, unseen))
}

// checkUnreadOnce seeds the badge with each account's real unseen count. It runs
// once at launch even for accounts with sync_interval = 0 (background syncing
// disabled): a STATUS carries no message bodies, and without it the badge would
// stay dark until the user happened to refresh an inbox.
func (s *EmailSyncService) checkUnreadOnce() {
	accounts, err := s.config.GetAccounts()
	if err != nil {
		slog.Error("tray: loading accounts failed", "error", err)
		return
	}
	for _, acct := range accounts {
		acct := acct
		go func() {
			folders, err := s.email.ListFolders(acct)
			if err != nil {
				slog.Warn("tray: listing folders failed", "account", acct, "error", err)
				return
			}
			for _, f := range folders {
				if isInbox(f.Name) {
					s.refreshUnread(acct, f.Name)
					return
				}
			}
		}()
	}
}

// publishUnread forwards the aggregate to the tray, but only when it flips:
// SetIcon crosses to the UI thread and there is no point doing that every sync.
func (s *EmailSyncService) publishUnread(hasUnread bool) {
	s.unreadMu.Lock()
	changed := hasUnread != s.lastUnread
	s.lastUnread = hasUnread
	listener := s.onUnread
	s.unreadMu.Unlock()

	if changed && listener != nil {
		slog.Info("tray: unread badge updated", "has_unread", hasUnread)
		listener(hasUnread)
	}
}

// Start begins the periodic sync loop. It checks every 30 seconds.
func (s *EmailSyncService) Start() {
	go func() {
		// The badge is seeded once at launch, independently of the sync interval.
		s.checkUnreadOnce()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.syncLoop()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop stops the background sync loop.
func (s *EmailSyncService) Stop() {
	close(s.stopCh)
}

// SetIdleWatcher links the real-time watcher. Accounts it is currently watching
// are skipped by the polling loop: fetching them on a timer as well would defeat
// the point of IDLE and double the connections.
func (s *EmailSyncService) SetIdleWatcher(idle *EmailIdleService) {
	s.idle = idle
}

// ForgetAccount drops the bookkeeping of a deleted account: its last-sync stamp
// and its share of the tray badge.
//
// syncLoop's unread.retain already does the badge part, and lastSync entries are
// unreachable once the account is gone, but both only catch up on the next tick —
// deleting the account that happened to hold the unread mail would leave the tray
// icon lit for up to 30s. RemoveAccount calls this through the listener wired in
// main.go so the UI settles immediately.
func (s *EmailSyncService) ForgetAccount(accountID string) {
	s.mu.Lock()
	delete(s.lastSync, accountID)
	s.mu.Unlock()

	s.publishUnread(s.unread.drop(accountID))
}

// OnMailArrived is the IDLE watcher's callback. The watcher has already fetched
// and cached the new page on its own connection, so this only records the sync and
// tells the UI; fetching again here would be a second full round trip for the same
// messages.
func (s *EmailSyncService) OnMailArrived(account EmailAccount, folder string) {
	s.mu.Lock()
	s.lastSync[account.ID] = time.Now()
	s.mu.Unlock()

	slog.Info("sync: new mail arrived (idle)", "account", account, "folder", folder)

	app := application.Get()
	if app != nil {
		app.Event.Emit("emails-synced", account.ID)
	}
	// The unseen count came off the same connection during the fetch (see
	// EmailService.reportUnseen), so no extra STATUS is needed here.
}

// ForceSyncNow immediately syncs the INBOX for a specific account and returns
// the refreshed email summaries. It is called from the frontend's force-refresh
// button.
func (s *EmailSyncService) ForceSyncNow(account EmailAccount, folder string, page, pageSize int) (*EmailPageResponse, error) {
	resp, err := s.email.RefreshEmails(account, folder, page, pageSize)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.lastSync[account.ID] = time.Now()
	s.mu.Unlock()

	// Notify frontend
	app := application.Get()
	if app != nil {
		app.Event.Emit("emails-synced", account.ID)
	}
	// A manual refresh is the moment the user expects the badge to be re-checked.
	if isInbox(folder) {
		s.refreshUnread(account, folder)
	}
	return resp, nil
}

// isInbox matches the folder names this app uses for the inbox across providers
// ("INBOX", "Inbox", "Folders/Inbox", "INBOX/", …).
func isInbox(folder string) bool {
	name := strings.ToLower(strings.TrimRight(folder, "/"))
	return name == "inbox" || strings.HasSuffix(name, "/inbox")
}

func (s *EmailSyncService) syncLoop() {
	accounts, err := s.config.GetAccounts()
	if err != nil {
		slog.Error("sync: loading accounts failed", "error", err)
		return
	}

	ids := make([]string, 0, len(accounts))
	for _, acct := range accounts {
		ids = append(ids, acct.ID)
	}
	// A removed account must not keep the tray badge lit.
	s.publishUnread(s.unread.retain(ids))

	now := time.Now()

	for _, acct := range accounts {
		if acct.SyncInterval <= 0 {
			continue
		}

		// IDLE is doing the job for this account; polling would only add
		// connections. Accounts that are reconnecting are NOT skipped: Watching is
		// false while the watcher is backing off, so the gap is covered here.
		if s.idle != nil && s.idle.Watching(acct.ID) {
			continue
		}

		s.mu.Lock()
		last, exists := s.lastSync[acct.ID]
		s.mu.Unlock()

		interval := time.Duration(acct.SyncInterval) * time.Minute
		if exists && now.Sub(last) < interval {
			continue
		}

		go s.syncAccount(acct)
	}
}

func (s *EmailSyncService) syncAccount(acct EmailAccount) {
	folders, err := s.email.ListFolders(acct)
	if err != nil {
		slog.Error("sync: listing folders failed", "account", acct, "error", err)
		return
	}

	// Find the INBOX folder and sync it
	for _, f := range folders {
		if !isInbox(f.Name) {
			continue
		}

		_, err := s.email.RefreshEmails(acct, f.Name, 1, 20)
		if err != nil {
			// Without this the whole background sync failed silently: the
			// UI keeps showing stale mail with no hint of why.
			slog.Error("sync: refreshing inbox failed",
				"account", acct, "folder", f.Name, "error", err)
			s.email.health.report(acct.ID, AccountHealthSourceSync, AccountHealthOutcomeFailure)
			return
		}

		s.mu.Lock()
		s.lastSync[acct.ID] = time.Now()
		s.mu.Unlock()

		app := application.Get()
		if app != nil {
			app.Event.Emit("emails-synced", acct.ID)
		}
		s.email.health.report(acct.ID, AccountHealthSourceSync, AccountHealthOutcomeOK)
		s.refreshUnread(acct, f.Name)
		return
	}
}
