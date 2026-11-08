package services

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// Real-time arrival: IMAP IDLE instead of polling.
//
// Why this exists: the interval sync in emailsyncservice.go answers "has anything
// arrived?" by connecting, logging in and fetching a page — on every tick,
// whether or not there is anything new. IDLE turns that around. One connection
// per account is opened once and then parked; the server pushes "* N EXISTS" the
// moment a message lands, so nothing is polled and mail appears in about a second
// instead of up to a sync interval later.
//
// The price is a connection that has to be kept alive. Sleep/wake, a Wi-Fi change
// and a provider's idle timeout all kill it without telling us, so every account
// has a watchdog (idleOnce selects on Client.Closed) and exponential reconnect.
// A server that refuses IDLE is handed back to interval polling, which stays in
// place as the fallback whenever no watcher is live.
//
// Scope: INBOX only, like the polling path. Other folders are refreshed when the
// user opens them (see stores/accounts.ts); idling every folder would mean one
// connection per folder per account, which the Chinese providers in particular do
// not allow.

const (
	// idleDebounce coalesces a burst: a mailing list that delivers five messages
	// in one second should cost one fetch, not five.
	idleDebounce = 1500 * time.Millisecond
	// idleFetchTimeout bounds one fetch made on the IDLE connection. Without it a
	// connection that died mid-command (the classic sleep/wake case) would park
	// the watcher forever, since no other goroutine can interrupt that read.
	idleFetchTimeout = 90 * time.Second
	// Reconnect backoff: 30s, 1m, 2m, 4m, 8m, then 15m. Starting short keeps the
	// first recovery quick (a laptop lid opening again); the cap stops a broken
	// account from hammering a server that is already rate-limiting us.
	idleRetryInitial = 30 * time.Second
	idleRetryMax     = 15 * time.Minute
	// idleReconcile is how often the account list is re-read, so a new account or
	// a switched-off auto-sync takes effect without restarting the app.
	idleReconcile = 30 * time.Second
	// idleHeartbeat is how often the idle session is proved alive.
	//
	// Client.Closed only fires when the socket errors, and a sleeping laptop does
	// not error anything: the kernel keeps the connection looking open until the
	// first write after wake. Without a heartbeat, closing the lid for an hour
	// would leave the watcher parked on a session the server has already dropped.
	// NOOP and STATUS UIDNEXT (after stopping IDLE, since no command may run while
	// idling) prove both the session and mailbox are responding.
	idleHeartbeat = 3 * time.Minute
	// idleProbeTimeout bounds that question. A server that cannot answer a NOOP in
	// 30 seconds is not going to push us anything either.
	idleProbeTimeout = 30 * time.Second
	// idleStartDelay keeps launch from opening three connections per account at
	// once: the tray seed (checkUnreadOnce) and the sidebar badges
	// (FolderStatuses) both dial in the first second, and QQ/163 count concurrent
	// sessions. Waiting costs nothing — the first IDLE only reports mail that
	// arrives after it starts, and the startup fetches cover everything before.
	idleStartDelay = 5 * time.Second
	// idleFetchPageSize matches the polling path: only the first page is fetched,
	// because the point is to have new mail in the list, not to mirror the mailbox.
	idleFetchPageSize = 20
)

// errIdleUnsupported marks a server that will not idle at all (no IDLE
// capability, or NO/BAD on the command). The account goes back to interval
// polling and is not retried until the next launch.
var errIdleUnsupported = errors.New("server does not support IDLE")

// EmailIdleService keeps one IDLE connection per auto-sync account.
type EmailIdleService struct {
	config *ConfigService
	email  *EmailService
	health *accountHealthReporter

	mu          sync.Mutex
	watchers    map[string]*idleWatcher
	unsupported map[string]bool // accountID → server refused IDLE this session

	// onMail runs after the watcher has fetched and cached the new page. It must
	// be quick and non-blocking: it runs on the watcher goroutine, the one holding
	// the account's only connection.
	onMail func(account EmailAccount, folder string)

	stopCh   chan struct{}
	stopOnce sync.Once
}

type idleWatcher struct {
	// account is the snapshot this connection was opened for. It is kept so the
	// stop path can name the mailbox in the log even after the account has been
	// removed from the config (LogValue renders it as account.email, see
	// configservice.go).
	account     EmailAccount
	fingerprint string // host/user/auth/password; a change restarts the watcher
	stop        chan struct{}

	// live is true only while the connection is actually idling. The polling loop
	// reads it: a reconnecting account is still covered by interval syncs, so a
	// flaky connection degrades to the old behaviour instead of going dark.
	live atomic.Bool
}

func NewEmailIdleService(cfg *ConfigService, email *EmailService) *EmailIdleService {
	return &EmailIdleService{
		config:      cfg,
		email:       email,
		health:      newAccountHealthReporter(),
		watchers:    make(map[string]*idleWatcher),
		unsupported: make(map[string]bool),
		stopCh:      make(chan struct{}),
	}
}

// SetAccountHealthReporter replaces the health reporter. It is wired by main()
// so the IDLE watcher and the mail services emit through one sequence.
//
//wails:ignore
func (s *EmailIdleService) SetAccountHealthReporter(reporter *accountHealthReporter) {
	if reporter == nil {
		reporter = newAccountHealthReporter()
	}
	s.health = reporter
}

// SetMailHandler registers the callback that runs after new mail has been cached.
func (s *EmailIdleService) SetMailHandler(handler func(account EmailAccount, folder string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMail = handler
}

// Watching reports whether an account currently has a live IDLE connection. The
// polling loop skips those accounts.
func (s *EmailIdleService) Watching(accountID string) bool {
	s.mu.Lock()
	w, ok := s.watchers[accountID]
	s.mu.Unlock()
	return ok && w.live.Load()
}

// IdleStates reports every account that has an IDLE watcher, and whether that
// watcher's connection is live right now. This is the per-account signal the
// sidebar's "disconnected" marker reads.
//
// Three things are deliberately *absent* from the map, because the caller must
// not read "no entry" as "broken":
//   - an account with auto-sync off has no watcher at all;
//   - a watcher the server refused to IDLE (unsupported) has fallen back to
//     interval polling, so it will never go live again this session;
//   - an account that was just added and has not been reconciled yet.
//
// A present-but-false entry is the interesting one: the watcher exists and its
// connection is currently down, i.e. it is sitting in the reconnect backoff.
//
// One call for the whole set rather than one Watching() per account: the sidebar
// asks about every account on each refresh.
func (s *EmailIdleService) IdleStates() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	states := make(map[string]bool, len(s.watchers))
	for id, w := range s.watchers {
		if s.unsupported[id] {
			continue
		}
		states[id] = w.live.Load()
	}
	return states
}

// StopWatching closes one account's IDLE connection and forgets it.
//
// reconcile() would reach the same state on its own within idleReconcile, but
// until it does the session is still open and still delivering: a deleted account
// that receives mail in that window gets fetched, cached and announced to the UI,
// and its server-side connection slot stays taken. ConfigService.RemoveAccount
// calls this (via the listener wired in main.go) so the connection goes away with
// the account instead of a tick later.
//
// Safe to call for an account that is not being watched, and safe to call twice:
// the map entry is the only handle, and both this and reconcile() delete it while
// holding s.mu, so w.stop is closed exactly once.
func (s *EmailIdleService) StopWatching(accountID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// A re-added account must get a fresh chance to IDLE, so the "this server
	// cannot IDLE" mark must not outlive the account.
	delete(s.unsupported, accountID)
	w, ok := s.watchers[accountID]
	if !ok {
		return
	}
	slog.Info("idle: stopping watcher", "account", w.account, "reason", "account removed")
	close(w.stop)
	delete(s.watchers, accountID)
}

// Start begins watching every account that has auto-sync enabled.
func (s *EmailIdleService) Start() {
	go func() {
		// Let the launch-time fetches finish first (see idleStartDelay).
		select {
		case <-time.After(idleStartDelay):
		case <-s.stopCh:
			return
		}
		s.reconcile()

		ticker := time.NewTicker(idleReconcile)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.reconcile()
			case <-s.stopCh:
				s.stopAll()
				return
			}
		}
	}()
}

// Stop closes every IDLE connection. Each watcher logs out on its way out, so the
// servers see a normal session end rather than a dropped socket.
func (s *EmailIdleService) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

// reconcile brings the running watchers in line with the config: one per account
// with SyncInterval > 0, none for the rest.
func (s *EmailIdleService) reconcile() {
	accounts, err := s.config.GetAccounts()
	if err != nil {
		slog.Error("idle: loading accounts failed", "error", err)
		return
	}

	wanted := make(map[string]EmailAccount, len(accounts))
	for _, acct := range accounts {
		if acct.SyncInterval > 0 {
			wanted[acct.ID] = acct
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, w := range s.watchers {
		acct, stillWanted := wanted[id]
		if stillWanted && fingerprint(acct) == w.fingerprint {
			continue
		}
		// Auto-sync switched off, the account removed, or its connection settings
		// edited: this connection is watching the wrong thing.
		reason := "auto-sync off"
		if stillWanted {
			reason = "settings changed"
		}
		slog.Info("idle: stopping watcher", "account", w.account, "reason", reason)
		close(w.stop)
		delete(s.watchers, id)
	}

	for id, acct := range wanted {
		if _, running := s.watchers[id]; running {
			continue
		}
		if s.unsupported[id] {
			continue
		}
		w := &idleWatcher{
			account:     acct,
			fingerprint: fingerprint(acct),
			stop:        make(chan struct{}),
		}
		s.watchers[id] = w
		go s.watch(w, acct)
	}

	// An account that was removed must not stay marked unsupported if it comes back.
	for id := range s.unsupported {
		if _, ok := wanted[id]; !ok {
			delete(s.unsupported, id)
		}
	}
}

func (s *EmailIdleService) stopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, w := range s.watchers {
		close(w.stop)
		delete(s.watchers, id)
	}
}

// watch is one account's reconnect loop. It exits only when the watcher is
// stopped or the server proves it cannot IDLE.
func (s *EmailIdleService) watch(w *idleWatcher, account EmailAccount) {
	backoff := idleRetryInitial
	for {
		select {
		case <-w.stop:
			return
		default:
		}

		connected := false
		err := s.idleOnce(w, account, func() { connected = true })
		// The watcher only reaches this line when the connection is no longer
		// usable (or it cannot establish one at all). The existing 30s IDLE poll
		// still decides when that becomes worth drawing; this event just makes
		// the transition available immediately.
		s.health.report(account.ID, AccountHealthSourceIdle, AccountHealthOutcomeFailure)
		if errors.Is(err, errIdleUnsupported) {
			slog.Info("idle: falling back to interval polling", "account", account)
			s.mu.Lock()
			s.unsupported[account.ID] = true
			s.mu.Unlock()
			return
		}
		if err == nil {
			return // stopped on purpose
		}
		if connected {
			s.health.report(account.ID, AccountHealthSourceIdle, AccountHealthOutcomeOK)
		}
		// A successful IDLE start means the server and the network were fine, so
		// the next drop deserves a fresh, short backoff.
		if connected {
			backoff = idleRetryInitial
		}

		slog.Warn("idle: connection lost, reconnecting",
			"account", account,
			"retry_in", backoff, "error", err)

		select {
		case <-time.After(backoff):
		case <-w.stop:
			return
		}
		backoff = nextIdleBackoff(backoff)
	}
}

// nextIdleBackoff doubles the delay up to the cap.
func nextIdleBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > idleRetryMax {
		return idleRetryMax
	}
	return next
}

// idleOnce holds one connection idling until something arrives or the connection
// dies. It returns nil when the watcher was stopped.
func (s *EmailIdleService) idleOnce(w *idleWatcher, account EmailAccount, connected func()) error {
	// Buffered and coalescing: the handler runs on the client's reader goroutine
	// and blocks it, so it must never wait for the consumer.
	arrivals := make(chan uint32, 1)
	handler := &imapclient.UnilateralDataHandler{
		Mailbox: func(data *imapclient.UnilateralDataMailbox) {
			if data.NumMessages == nil {
				// Flags-only updates (including the ones our own STORE causes) are
				// not worth a fetch; the list re-reads them when it is opened.
				return
			}
			select {
			case arrivals <- *data.NumMessages:
			default:
			}
		},
	}

	client, err := dialIMAPWith(account, 60*time.Second, handler)
	if err != nil {
		return err
	}
	defer client.Logout()

	// RFC 2177 is an extension: ask before assuming. One CAPABILITY is cheaper
	// than a failed IDLE, and it turns "keeps failing with BAD" into one clear log
	// line plus a clean fallback to polling.
	caps, err := client.Capability().Wait()
	if err != nil {
		return err
	}
	if !caps.Has(imap.CapIdle) {
		return errIdleUnsupported
	}

	folder := s.inboxFolder(account)
	mbox, err := selectMailbox(client, folder, true)
	if err != nil {
		return err
	}
	meta, metaErr := s.email.db.LoadFolderMeta(account.ID, folder)
	if metaErr != nil {
		slog.Warn("idle: failed to load inbox notification baseline",
			"account", account, "folder", folder, "error", metaErr)
	} else if meta.UIDNext == 0 {
		if err := s.email.db.SaveFolderMeta(account.ID, folder, FolderMeta{
			Total:   int(mbox.NumMessages),
			UIDNext: uint32(mbox.UIDNext),
		}); err != nil {
			slog.Warn("idle: failed to save inbox notification baseline",
				"account", account, "folder", folder, "error", err)
		}
	}

	total := int64(mbox.NumMessages)
	uidNext := uint32(mbox.UIDNext)
	if metaErr == nil && meta.UIDNext > 0 && uidNext > meta.UIDNext {
		slog.Info("idle: reconciling inbox changes since last run",
			"account", account, "folder", folder, "previous_uid_next", meta.UIDNext, "uid_next", uidNext)
		resp, fetchErr := s.fetchOnConnection(client, account, folder)
		if fetchErr != nil {
			return fetchErr
		}
		if resp != nil {
			total = int64(resp.Total)
			uidNext = resp.UIDNext
		}
		if handler := s.mailHandler(); handler != nil && resp != nil {
			handler(account, folder)
		}
	}

	idle, err := client.Idle()
	if err != nil {
		if isIdleUnsupported(err) {
			return errIdleUnsupported
		}
		return err
	}

	// total is what the server last told us about the mailbox. It starts at the
	// SELECT's count and moves after every fetch, which is what makes a repeated
	// EXISTS (some servers re-send it when IDLE restarts) a no-op instead of an
	// endless fetch loop.
	w.live.Store(true)
	s.health.report(account.ID, AccountHealthSourceIdle, AccountHealthOutcomeOK)
	defer w.live.Store(false)
	connected()
	slog.Info("idle: watching inbox",
		"account", account, "folder", folder, "messages", total)

	heartbeat := time.NewTicker(idleHeartbeat)
	defer heartbeat.Stop()

	for {
		select {
		case <-w.stop:
			closeIdle(idle)
			return nil

		case <-client.Closed():
			// The reader goroutine is gone: sleep, a network change, or the server
			// dropping an idle session. Closing IDLE only unblocks it.
			closeIdle(idle)
			return errors.New("connection closed")

		case <-heartbeat.C:
			// Prove the session is still there (see idleHeartbeat). This also
			// re-arms IDLE against middleboxes that drop long-silent connections.
			// STATUS UIDNEXT also catches providers that advertise IDLE but fail to
			// push EXISTS reliably, without opening another IMAP connection.
			if err := closeIdle(idle); err != nil {
				return err
			}
			if err := s.probe(client); err != nil {
				return err
			}
			status, statusErr := probeUIDNext(client, folder)
			if statusErr != nil {
				slog.Warn("idle: inbox UIDNEXT probe failed",
					"account", account, "folder", folder, "error", statusErr)
				return statusErr
			} else if status.UIDNext > imap.UID(uidNext) {
				slog.Info("idle: inbox changed without an IDLE EXISTS event",
					"account", account, "folder", folder,
					"previous_uid_next", uidNext, "uid_next", status.UIDNext)
				resp, fetchErr := s.fetchOnConnection(client, account, folder)
				if fetchErr != nil {
					return fetchErr
				}
				if resp != nil {
					total = int64(resp.Total)
					uidNext = resp.UIDNext
				}
				if handler := s.mailHandler(); handler != nil && resp != nil {
					handler(account, folder)
				}
			}
			if idle, err = client.Idle(); err != nil {
				return err
			}

		case first := <-arrivals:
			// Give a burst a moment to finish arriving, then fetch once. The
			// returned count is the highest one seen, not the first.
			latest, ok := waitDebounce(first, arrivals, w.stop)
			if !ok {
				closeIdle(idle)
				return nil
			}
			if int64(latest) == total {
				continue // these messages are already in the cache
			}

			if err := closeIdle(idle); err != nil {
				return err
			}
			resp, err := s.fetchOnConnection(client, account, folder)
			if err != nil {
				return err
			}
			if resp != nil {
				total = int64(resp.Total)
				uidNext = resp.UIDNext
			}
			if handler := s.mailHandler(); handler != nil {
				handler(account, folder)
			}

			if idle, err = client.Idle(); err != nil {
				return err
			}
		}
	}
}

// probe sends one NOOP and waits for its answer, bounded by idleProbeTimeout.
// The socket has to be closed on timeout: nothing else can interrupt the read.
func (s *EmailIdleService) probe(client *imapclient.Client) error {
	done := make(chan error, 1)
	go func() { done <- client.Noop().Wait() }()

	select {
	case err := <-done:
		return err
	case <-time.After(idleProbeTimeout):
		client.Close()
		return fmt.Errorf("no answer to NOOP within %v", idleProbeTimeout)
	}
}

func probeUIDNext(client *imapclient.Client, folder string) (*imap.StatusData, error) {
	type result struct {
		status *imap.StatusData
		err    error
	}
	done := make(chan result, 1)
	go func() {
		status, err := client.Status(folder, &imap.StatusOptions{UIDNext: true}).Wait()
		done <- result{status: status, err: err}
	}()

	select {
	case res := <-done:
		return res.status, res.err
	case <-time.After(idleProbeTimeout):
		client.Close()
		return nil, fmt.Errorf("no answer to STATUS UIDNEXT within %v", idleProbeTimeout)
	}
}

// waitDebounce waits for the quiet edge of a burst, draining further arrivals. It
// returns the highest count seen and false when the watcher was stopped meanwhile.
func waitDebounce(first uint32, arrivals <-chan uint32, stop <-chan struct{}) (uint32, bool) {
	timer := time.NewTimer(idleDebounce)
	defer timer.Stop()

	latest := first
	for {
		select {
		case seen := <-arrivals:
			if seen > latest {
				latest = seen
			}
			// Another message in the same burst: keep waiting for the quiet edge.
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(idleDebounce)
		case <-timer.C:
			return latest, true
		case <-stop:
			return latest, false
		}
	}
}

// fetchOnConnection fetches page 1 on the caller's connection, bounded by
// idleFetchTimeout.
func (s *EmailIdleService) fetchOnConnection(client *imapclient.Client, account EmailAccount, folder string) (*EmailPageResponse, error) {
	type result struct {
		resp *EmailPageResponse
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := s.email.fetchEmailsOn(client, account, folder, 1, idleFetchPageSize)
		done <- result{resp: resp, err: err}
	}()

	select {
	case res := <-done:
		return res.resp, res.err
	case <-time.After(idleFetchTimeout):
		// Closing the socket is the only way to unblock the read above.
		client.Close()
		return nil, fmt.Errorf("fetch timed out after %v", idleFetchTimeout)
	}
}

// inboxFolder resolves the account's INBOX name. The cache is authoritative when
// it has one ("INBOX", "Inbox", "Folders/Inbox" are all in use across providers);
// a miss costs one LIST rather than guessing wrong and selecting a mailbox that
// does not exist.
func (s *EmailIdleService) inboxFolder(account EmailAccount) string {
	folders, err := s.email.ListFolders(account)
	if err == nil {
		for _, f := range folders {
			if isInbox(f.Name) {
				return f.Name
			}
		}
	}
	return "INBOX"
}

func (s *EmailIdleService) mailHandler() func(EmailAccount, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.onMail
}

// closeIdle stops IDLE and swallows the "already closed" error, so every shutdown
// path above can call it without tracking whether it already ran.
//
// The tagged OK that answers DONE is deliberately not waited for: go-imap buffers
// it and matches it by tag on the reader goroutine, while waiting here would block
// the watcher on a half-dead socket that never answers.
func closeIdle(cmd *imapclient.IdleCommand) error {
	if cmd == nil {
		return nil
	}
	err := cmd.Close()
	if err != nil && strings.Contains(err.Error(), "already closed") {
		return nil
	}
	return err
}

// isIdleUnsupported classifies a failed IDLE as "this server will not do it"
// rather than "the connection broke". Only NO/BAD counts: a timeout or a closed
// socket is a transient fault and must be retried.
func isIdleUnsupported(err error) bool {
	if err == nil {
		return false
	}
	var imapErr *imap.Error
	if errors.As(err, &imapErr) {
		if imapErr.Type == imap.StatusResponseTypeNo || imapErr.Type == imap.StatusResponseTypeBad {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "idle") &&
		(strings.Contains(msg, "unknown") || strings.Contains(msg, "not supported") || strings.Contains(msg, "unsupported"))
}

// fingerprint identifies the connection settings a watcher was started with.
// Editing the host, user, auth type or password must reconnect; anything else
// (avatar, display name, sync interval) must not.
func fingerprint(account EmailAccount) string {
	return strings.Join([]string{
		account.IMAPHost,
		fmt.Sprint(account.IMAPPort),
		account.IMAPUser,
		account.AuthType,
		account.IMAPPass,
		account.OAuthRefreshToken,
		fmt.Sprint(account.UseTLS),
	}, "\x00")
}
