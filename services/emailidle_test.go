package services

import (
	"errors"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
)

// The reconnect ladder is the difference between "recovers when the lid opens"
// and "hammers a server that is already throttling us", so the steps are pinned.
func TestNextIdleBackoff(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want time.Duration
	}{
		{30 * time.Second, time.Minute},
		{time.Minute, 2 * time.Minute},
		{4 * time.Minute, 8 * time.Minute},
		{8 * time.Minute, 15 * time.Minute},
		{15 * time.Minute, 15 * time.Minute},
		{time.Hour, 15 * time.Minute},
	}
	for _, tc := range cases {
		if got := nextIdleBackoff(tc.in); got != tc.want {
			t.Errorf("nextIdleBackoff(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// Only a NO/BAD answer means "this server will not IDLE". Treating a timeout as
// unsupported would silently downgrade an account to polling for the rest of the
// session after one bad Wi-Fi moment.
func TestIsIdleUnsupported(t *testing.T) {
	no := &imap.Error{Type: imap.StatusResponseTypeNo, Text: "IDLE not supported"}
	bad := &imap.Error{Type: imap.StatusResponseTypeBad, Text: "unknown command"}
	bye := &imap.Error{Type: imap.StatusResponseTypeBye, Text: "too many connections"}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"no", no, true},
		{"bad", bad, true},
		{"bye is transient", bye, false},
		{"plain timeout", errors.New("i/o timeout"), false},
		{"wrapped no", errors.Join(errors.New("idle failed"), no), true},
	}
	for _, tc := range cases {
		if got := isIdleUnsupported(tc.err); got != tc.want {
			t.Errorf("%s: isIdleUnsupported = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Editing an avatar or a display name must not tear down the connection; editing
// the credentials must.
func TestIdleFingerprint(t *testing.T) {
	base := EmailAccount{ID: "a", IMAPHost: "imap.qq.com", IMAPPort: 993, IMAPUser: "me", IMAPPass: "p", UseTLS: true}

	same := base
	same.Name = "Renamed"
	same.Avatar = "new.png"
	same.SyncInterval = 30
	if fingerprint(base) != fingerprint(same) {
		t.Error("cosmetic changes must not change the fingerprint")
	}

	moved := base
	moved.IMAPHost = "imap.exmail.qq.com"
	if fingerprint(base) == fingerprint(moved) {
		t.Error("a different host must change the fingerprint")
	}

	rotated := base
	rotated.IMAPPass = "q"
	if fingerprint(base) == fingerprint(rotated) {
		t.Error("a new password must change the fingerprint")
	}
}

// Deleting an account must close its connection right away: waiting for the 30s
// reconcile would leave a session open for a mailbox the user just removed, and
// mail arriving in that window would still be fetched and announced.
func TestStopWatchingClosesTheConnection(t *testing.T) {
	idle := NewEmailIdleService(nil, nil)
	w := &idleWatcher{account: EmailAccount{ID: "a", Email: "a@example.com"}, stop: make(chan struct{})}
	idle.watchers["a"] = w
	idle.unsupported["a"] = true

	idle.StopWatching("a")

	select {
	case <-w.stop:
	default:
		t.Fatal("stop channel was not closed")
	}
	if _, ok := idle.watchers["a"]; ok {
		t.Error("watcher must be forgotten, otherwise reconcile would close it again")
	}
	if _, ok := idle.unsupported["a"]; ok {
		t.Error("the cannot-IDLE mark must not outlive the account")
	}
	if idle.Watching("a") {
		t.Error("Watching must report false once the account is gone")
	}

	// A second delete (or reconcile racing this one) must not close w.stop twice.
	idle.StopWatching("a")
	idle.StopWatching("never-watched")
}

// The sidebar's "disconnected" marker is built from this map, so the three states
// it must tell apart are pinned here: live, present-but-reconnecting, and "do not
// draw anything" for an account whose watcher cannot be trusted as a signal.
func TestIdleStatesReportsLiveAndSkipsUnsupported(t *testing.T) {
	idle := NewEmailIdleService(nil, nil)

	live := &idleWatcher{account: EmailAccount{ID: "live"}, stop: make(chan struct{})}
	live.live.Store(true)
	reconnecting := &idleWatcher{account: EmailAccount{ID: "reconnecting"}, stop: make(chan struct{})}
	refused := &idleWatcher{account: EmailAccount{ID: "refused"}, stop: make(chan struct{})}

	idle.watchers["live"] = live
	idle.watchers["reconnecting"] = reconnecting
	idle.watchers["refused"] = refused
	// The server answered NO to IDLE: the watcher goroutine is gone and the account
	// is on interval polling, so reporting it as disconnected would be permanent
	// noise about an account that is working.
	idle.unsupported["refused"] = true

	states := idle.IdleStates()

	if got, ok := states["live"]; !ok || !got {
		t.Errorf("live watcher: got %v (present %v), want true", got, ok)
	}
	if got, ok := states["reconnecting"]; !ok || got {
		t.Errorf("reconnecting watcher: got %v (present %v), want a present false", got, ok)
	}
	if _, ok := states["refused"]; ok {
		t.Error("a watcher that cannot IDLE must be absent, not false")
	}
	if len(states) != 2 {
		t.Errorf("IdleStates() returned %d entries, want 2: %v", len(states), states)
	}
}
