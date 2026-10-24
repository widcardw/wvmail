package services

import "testing"

// Deleting an account has to reach the background services, not just the config
// file: they only re-read the account list on their next tick, so without this
// callback a removed mailbox keeps an IDLE connection (and its tray badge) for up
// to 30 seconds.
func TestRemoveAccountNotifiesListener(t *testing.T) {
	svc := seedMoveAccounts(t)

	var removed []string
	svc.SetAccountRemovedListener(func(accountID string) {
		removed = append(removed, accountID)
	})

	if err := svc.RemoveAccount("b"); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}
	assertIDs(t, accountIDs(t, svc), "a", "c")

	if len(removed) != 1 || removed[0] != "b" {
		t.Fatalf("listener calls = %v, want [b]", removed)
	}
}

// A delete that did not happen must not stop anything: the account is still
// configured, and its connection is still the one that delivers its mail.
func TestRemoveAccountUnknownIDDoesNotNotify(t *testing.T) {
	svc := seedMoveAccounts(t)

	var removed []string
	svc.SetAccountRemovedListener(func(accountID string) {
		removed = append(removed, accountID)
	})

	if err := svc.RemoveAccount("nope"); err == nil {
		t.Fatal("expected an error for an unknown account id")
	}
	if len(removed) != 0 {
		t.Fatalf("listener calls = %v, want none", removed)
	}
	assertIDs(t, accountIDs(t, svc), "a", "b", "c")
}

// The listener is optional: the server build and the tests wire no background
// services, and RemoveAccount still has to work there.
func TestRemoveAccountWithoutListener(t *testing.T) {
	svc := seedMoveAccounts(t)

	if err := svc.RemoveAccount("a"); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}
	assertIDs(t, accountIDs(t, svc), "b", "c")
}
