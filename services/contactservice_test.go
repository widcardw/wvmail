package services

import "testing"

func TestContactServiceRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := NewContactsService()
	zoe, err := svc.SaveContact(Contact{Name: "Zoe", Email: "ZOE@example.com"})
	if err != nil {
		t.Fatalf("SaveContact(zoe): %v", err)
	}
	if zoe.ID == "" {
		t.Error("SaveContact must assign an ID when none was given")
	}
	if zoe.Email != "zoe@example.com" {
		t.Errorf("email = %q, want normalized lower case", zoe.Email)
	}

	alice, err := svc.SaveContact(Contact{Email: "Alice <ALICE@example.com>"})
	if err != nil {
		t.Fatalf("SaveContact(alice): %v", err)
	}
	if alice.Name != "Alice" {
		t.Errorf("name = %q, want the display name from the address", alice.Name)
	}
	aliceCreatedAt := alice.CreatedAt

	// Adding the same address again is idempotent and can fill in a missing name.
	duplicate, err := svc.SaveContact(Contact{Name: "Alice Chen", Email: "alice@example.com"})
	if err != nil {
		t.Fatalf("SaveContact(duplicate): %v", err)
	}
	if duplicate.ID != alice.ID {
		t.Errorf("duplicate email created a new contact: got %q want %q", duplicate.ID, alice.ID)
	}
	if duplicate.Name != "Alice" {
		t.Errorf("duplicate add overwrote the saved name: %q", duplicate.Name)
	}

	got, err := svc.LoadContacts()
	if err != nil {
		t.Fatalf("LoadContacts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(contacts) = %d, want 2", len(got))
	}
	if got[0].Name != "Alice" || got[1].Name != "Zoe" {
		t.Errorf("contacts are not in display order: %+v", got)
	}

	if _, err := svc.SaveContact(Contact{ID: alice.ID, Name: "Alice Chen", Email: "alice@example.com"}); err != nil {
		t.Fatalf("SaveContact(update): %v", err)
	}
	got, err = svc.LoadContacts()
	if err != nil {
		t.Fatalf("LoadContacts after update: %v", err)
	}
	if got[0].Name != "Alice Chen" {
		t.Errorf("updated name = %q, want Alice Chen", got[0].Name)
	}
	if got[0].CreatedAt != aliceCreatedAt {
		t.Errorf("updated contact changed created_at: got %d want %d", got[0].CreatedAt, aliceCreatedAt)
	}

	if err := svc.DeleteContact(alice.ID); err != nil {
		t.Fatalf("DeleteContact: %v", err)
	}
	got, err = svc.LoadContacts()
	if err != nil {
		t.Fatalf("LoadContacts after delete: %v", err)
	}
	if len(got) != 1 || got[0].ID != zoe.ID {
		t.Errorf("unexpected contacts after delete: %+v", got)
	}
}

func TestContactServiceValidation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := NewContactsService()

	if _, err := svc.SaveContact(Contact{Name: "Nobody"}); err == nil {
		t.Error("a contact without an email must be rejected")
	}
	if _, err := svc.SaveContact(Contact{Email: "not-an-email"}); err == nil {
		t.Error("an invalid email must be rejected")
	}
	if err := svc.DeleteContact(""); err == nil {
		t.Error("DeleteContact without an ID must be rejected")
	}
}

func TestAutoCollectContactsConfigField(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &ConfigService{}
	config, err := cfg.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if config.AutoCollectContacts {
		t.Fatal("automatic contact collection must default to off")
	}
	if err := cfg.SaveConfigField("auto_collect_contacts", "true"); err != nil {
		t.Fatalf("SaveConfigField: %v", err)
	}
	config, err = cfg.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig after save: %v", err)
	}
	if !config.AutoCollectContacts {
		t.Fatal("automatic contact collection was not persisted")
	}
}

func TestContactAutoCollectUsesBaselineAndNewInboxUIDs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &ConfigService{}
	if err := cfg.SaveConfigField("auto_collect_contacts", "true"); err != nil {
		t.Fatalf("enable auto collect: %v", err)
	}
	svc := NewContactsService(cfg)
	account := EmailAccount{ID: "acct", Email: "me@example.com", IMAPUser: "me@example.com"}

	// The first observation establishes the baseline and must not import the
	// mailbox's existing messages.
	added, err := svc.CollectFromMail(account, "INBOX", []EmailSummary{
		{UID: 10, FromName: "Old Sender", FromAddr: "old@example.com"},
	})
	if err != nil {
		t.Fatalf("CollectFromMail(baseline): %v", err)
	}
	if added != 0 {
		t.Fatalf("baseline added %d contacts, want 0", added)
	}

	added, err = svc.CollectFromMail(account, "INBOX", []EmailSummary{
		{UID: 10, FromName: "Old Sender", FromAddr: "old@example.com"},
		{UID: 11, FromName: "Alice", FromAddr: "alice@example.com"},
		{UID: 12, FromName: "No Reply", FromAddr: "no-reply@example.com"},
		{UID: 13, FromName: "Newsletter", FromAddr: "news@example.com", IsBulk: true},
		{UID: 14, FromName: "Me", FromAddr: "me@example.com"},
	})
	if err != nil {
		t.Fatalf("CollectFromMail(new messages): %v", err)
	}
	if added != 1 {
		t.Fatalf("added %d contacts, want only Alice", added)
	}

	contacts, err := svc.LoadContacts()
	if err != nil {
		t.Fatalf("LoadContacts: %v", err)
	}
	if len(contacts) != 1 || contacts[0].Email != "alice@example.com" || contacts[0].Name != "Alice" {
		t.Fatalf("unexpected auto-collected contacts: %+v", contacts)
	}
}

func TestContactAutoCollectRespectsDeletionTombstone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &ConfigService{}
	if err := cfg.SaveConfigField("auto_collect_contacts", "true"); err != nil {
		t.Fatalf("enable auto collect: %v", err)
	}
	svc := NewContactsService(cfg)
	account := EmailAccount{ID: "acct", Email: "me@example.com"}

	if _, err := svc.CollectFromMail(account, "INBOX", nil); err != nil {
		t.Fatalf("initialize collection state: %v", err)
	}
	if added, err := svc.CollectFromMail(account, "INBOX", []EmailSummary{
		{UID: 1, FromName: "Alice", FromAddr: "alice@example.com"},
	}); err != nil || added != 1 {
		t.Fatalf("first collect = %d, %v; want 1 contact", added, err)
	}

	contacts, err := svc.LoadContacts()
	if err != nil || len(contacts) != 1 {
		t.Fatalf("LoadContacts = %+v, %v", contacts, err)
	}
	if err := svc.DeleteContact(contacts[0].ID); err != nil {
		t.Fatalf("DeleteContact: %v", err)
	}

	excluded, err := svc.db.LoadContactExclusionEmails()
	if err != nil {
		t.Fatalf("LoadContactExclusionEmails: %v", err)
	}
	if !excluded["alice@example.com"] {
		t.Fatal("deleting a contact must add a tombstone")
	}

	if added, err := svc.CollectFromMail(account, "INBOX", []EmailSummary{
		{UID: 2, FromName: "Alice", FromAddr: "alice@example.com"},
	}); err != nil || added != 0 {
		t.Fatalf("collect after delete = %d, %v; want 0 contacts", added, err)
	}

	if _, err := svc.SaveContact(Contact{Name: "Alice", Email: "alice@example.com"}); err != nil {
		t.Fatalf("manual SaveContact: %v", err)
	}
	excluded, err = svc.db.LoadContactExclusionEmails()
	if err != nil {
		t.Fatalf("LoadContactExclusionEmails after manual add: %v", err)
	}
	if excluded["alice@example.com"] {
		t.Fatal("manual add must clear the tombstone")
	}
}

func TestContactAutoCollectFiltersRoleAddresses(t *testing.T) {
	tests := []struct {
		email string
		ok    bool
	}{
		{"alice@example.com", true},
		{"notify-me@example.com", false},
		{"no-reply+project@example.com", false},
		{"notifications@example.com", false},
		{"mailer-daemon@example.com", false},
		{"newsletter@example.com", false},
		{"support@example.com", true},
	}
	for _, test := range tests {
		if got := collectableContactEmail(test.email); got != test.ok {
			t.Errorf("collectableContactEmail(%q) = %v, want %v", test.email, got, test.ok)
		}
	}
}

func TestContactAutoCollectOnlyRunsForInbox(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &ConfigService{}
	if err := cfg.SaveConfigField("auto_collect_contacts", "true"); err != nil {
		t.Fatalf("enable auto collect: %v", err)
	}
	svc := NewContactsService(cfg)
	account := EmailAccount{ID: "acct", Email: "me@example.com"}

	if added, err := svc.CollectFromMail(account, "Archive", []EmailSummary{
		{UID: 1, FromName: "Alice", FromAddr: "alice@example.com"},
	}); err != nil || added != 0 {
		t.Fatalf("Archive collection = %d, %v; want 0", added, err)
	}
	contacts, err := svc.LoadContacts()
	if err != nil {
		t.Fatalf("LoadContacts: %v", err)
	}
	if len(contacts) != 0 {
		t.Fatalf("non-INBOX mail added contacts: %+v", contacts)
	}
}
