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
