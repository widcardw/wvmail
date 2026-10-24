package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedMoveAccounts writes three accounts and returns the service.
func seedMoveAccounts(t *testing.T) *ConfigService {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	svc := &ConfigService{}
	accounts := []EmailAccount{
		{ID: "a", Name: "A", Email: "a@example.com", IMAPPass: "pass-a", SMTPPass: "smtp-a"},
		{ID: "b", Name: "B", Email: "b@example.com"},
		{ID: "c", Name: "C", Email: "c@example.com"},
	}
	if err := svc.SaveAccounts(accounts); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return svc
}

func accountIDs(t *testing.T, svc *ConfigService) []string {
	t.Helper()
	accounts, err := svc.GetAccounts()
	if err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	ids := make([]string, len(accounts))
	for i, a := range accounts {
		ids[i] = a.ID
	}
	return ids
}

func assertIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// Moving up and down is the whole feature: the list order is what the sidebar and
// the settings page render, and it has to survive the round trip through YAML.
func TestMoveAccountReordersAndPersists(t *testing.T) {
	svc := seedMoveAccounts(t)

	if err := svc.MoveAccount("c", -1); err != nil {
		t.Fatalf("move c up: %v", err)
	}
	assertIDs(t, accountIDs(t, svc), "a", "c", "b")

	if err := svc.MoveAccount("a", 1); err != nil {
		t.Fatalf("move a down: %v", err)
	}
	assertIDs(t, accountIDs(t, svc), "c", "a", "b")
}

// The UI hides "move up" on the first row and "move down" on the last, but a
// stale click can still arrive: it must be a no-op, not an error and not a
// corrupted order.
func TestMoveAccountPastTheEndIsNoOp(t *testing.T) {
	svc := seedMoveAccounts(t)

	if err := svc.MoveAccount("a", -1); err != nil {
		t.Fatalf("move first up: %v", err)
	}
	assertIDs(t, accountIDs(t, svc), "a", "b", "c")

	if err := svc.MoveAccount("c", 1); err != nil {
		t.Fatalf("move last down: %v", err)
	}
	assertIDs(t, accountIDs(t, svc), "a", "b", "c")
}

func TestMoveAccountUnknownID(t *testing.T) {
	svc := seedMoveAccounts(t)

	if err := svc.MoveAccount("nope", 1); err == nil {
		t.Fatal("expected an error for an unknown account id")
	}
	assertIDs(t, accountIDs(t, svc), "a", "b", "c")
}

// Reordering rewrites the whole config file, so it must not disturb the secrets
// sitting in it: the passwords have to come back decrypted and stay encrypted on
// disk (readConfig/writeConfig do the cipher work, MoveAccount only shifts slots).
func TestMoveAccountKeepsPasswords(t *testing.T) {
	svc := seedMoveAccounts(t)

	if err := svc.MoveAccount("c", -1); err != nil {
		t.Fatalf("move c up: %v", err)
	}

	accounts, err := svc.GetAccounts()
	if err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	var moved *EmailAccount
	for i := range accounts {
		if accounts[i].ID == "a" {
			moved = &accounts[i]
		}
	}
	if moved == nil {
		t.Fatal("account a disappeared")
	}
	if moved.IMAPPass != "pass-a" || moved.SMTPPass != "smtp-a" {
		t.Fatalf("passwords = %q/%q, want pass-a/smtp-a", moved.IMAPPass, moved.SMTPPass)
	}

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(raw), "pass-a") {
		t.Errorf("IMAP password written in clear text: %s", filepath.Base(path))
	}
}
