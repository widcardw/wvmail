package services

import "testing"

// The tray badge follows the aggregate across accounts, and an account that stops
// answering keeps its last value.
func TestUnreadTrackerAggregates(t *testing.T) {
	tracker := newUnreadTracker()
	if tracker.observe("a", 0) {
		t.Error("no unread mail must leave the badge off")
	}
	if !tracker.observe("a", 3) {
		t.Error("unread mail must light the badge")
	}
	if !tracker.observe("b", 0) {
		t.Error("a second, read account must not clear the first account's unread")
	}
	if tracker.observe("a", 0) {
		t.Error("badge must clear once every account is read")
	}
}

func TestUnreadTrackerClampsNegativeCounts(t *testing.T) {
	tracker := newUnreadTracker()
	if tracker.observe("a", -5) {
		t.Error("a negative count must not light the badge")
	}
}

func TestUnreadTrackerRetainDropsRemovedAccounts(t *testing.T) {
	tracker := newUnreadTracker()
	tracker.observe("kept", 0)
	tracker.observe("removed", 4)

	if !tracker.retain([]string{"kept", "removed"}) {
		t.Fatal("both accounts are still configured, so the badge stays on")
	}
	// Dropping the account that had unread mail has to clear the badge, otherwise
	// deleting an account leaves the tray icon lit forever.
	if tracker.retain([]string{"kept"}) {
		t.Error("badge must clear when the unread account is removed")
	}
}

// SetIcon crosses to the UI thread, so the listener must only fire when the
// aggregate actually flips.
func TestPublishUnreadOnlyFiresOnChange(t *testing.T) {
	svc := NewEmailSyncService(nil, nil)
	var calls []bool
	svc.SetUnreadListener(func(hasUnread bool) {
		calls = append(calls, hasUnread)
	})

	svc.publishUnread(false) // initial state, no listener call yet
	svc.publishUnread(true)
	svc.publishUnread(true)
	svc.publishUnread(false)
	svc.publishUnread(false)

	want := []bool{true, false}
	if len(calls) != len(want) {
		t.Fatalf("listener calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("listener calls = %v, want %v", calls, want)
		}
	}
}

func TestIsInboxMatchesProviderSpellings(t *testing.T) {
	for _, folder := range []string{"INBOX", "Inbox", "inbox", "Folders/Inbox", "INBOX/"} {
		if !isInbox(folder) {
			t.Errorf("isInbox(%q) = false, want true", folder)
		}
	}
	for _, folder := range []string{"Sent", "Archive", "InboxOld"} {
		if isInbox(folder) {
			t.Errorf("isInbox(%q) = true, want false", folder)
		}
	}
}
