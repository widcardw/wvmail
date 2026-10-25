package services

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AccountHealthEvent is the structured signal behind the sidebar's per-account
// connection marker. It is emitted at the operation that already knows the
// outcome, rather than parsed from log text: logs are for people, their wording
// and level are not a stable interface.
type AccountHealthEvent struct {
	Seq     int    `json:"seq"`
	At      int64  `json:"at"`
	Account string `json:"account"`
	Source  string `json:"source"`  // idle | sync | status
	Outcome string `json:"outcome"` // ok | failure
}

const (
	AccountHealthSourceIdle   = "idle"
	AccountHealthSourceSync   = "sync"
	AccountHealthSourceStatus = "status"

	AccountHealthOutcomeOK      = "ok"
	AccountHealthOutcomeFailure = "failure"
)

// accountHealthReporter owns the event sequence and the latest event per
// account. The snapshot lets a frontend that mounts after a failure catch up;
// the sequence number lets it drop a late duplicate once it is subscribed.
type accountHealthReporter struct {
	seq      atomic.Int64
	mu       sync.RWMutex
	latest   map[string]AccountHealthEvent
	history  []AccountHealthEvent
	capacity int
}

func newAccountHealthReporter() *accountHealthReporter {
	return &accountHealthReporter{
		latest:   make(map[string]AccountHealthEvent),
		history:  make([]AccountHealthEvent, 0, 32),
		capacity: 64,
	}
}

func (r *accountHealthReporter) report(accountID, source, outcome string) {
	if r == nil {
		return
	}
	if accountID == "" {
		return
	}

	event := AccountHealthEvent{
		Seq:     int(r.seq.Add(1)),
		At:      time.Now().UnixMilli(),
		Account: accountID,
		Source:  source,
		Outcome: outcome,
	}

	r.mu.Lock()
	r.latest[accountID] = event
	r.history = append(r.history, event)
	if len(r.history) > r.capacity {
		r.history = r.history[len(r.history)-r.capacity:]
	}
	r.mu.Unlock()

	// Emitting is best-effort: no webview exists during tests and the server
	// build, and a failed delivery must not fail the IMAP operation that saw it.
	if app := application.Get(); app != nil {
		app.Event.Emit("account-health", event)
	}
}

// forget clears one account after it is removed from the config. The event is
// kept out of the latest map, but is still emitted so a mounted UI can drop its
// health state immediately.
func (r *accountHealthReporter) forget(accountID string) {
	if accountID == "" {
		return
	}
	r.mu.Lock()
	delete(r.latest, accountID)
	r.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit("account-health", AccountHealthEvent{
			Seq:     int(r.seq.Add(1)),
			At:      time.Now().UnixMilli(),
			Account: accountID,
			Source:  AccountHealthSourceStatus,
			Outcome: "forgot",
		})
	}
}

// NewAccountHealthReporter is the wiring point used by main; tests can create
// their own reporter directly.
func NewAccountHealthReporter() *accountHealthReporter {
	return newAccountHealthReporter()
}

// AccountHealthEvents returns the bounded recent history, oldest first.
func (r *accountHealthReporter) events() []AccountHealthEvent {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]AccountHealthEvent(nil), r.history...)
}
