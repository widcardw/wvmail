package services

import "testing"

// The optional "also delete cached mail" checkbox on the delete dialog: it has to
// clear every table that is keyed by account_id, and leave other accounts alone.
func TestDeleteAccountCacheRemovesOnlyThatAccount(t *testing.T) {
	// Keep the test away from the real ~/Library/Application Support/wvmail cache.
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	for _, account := range []string{"gone", "kept"} {
		if err := db.SaveFolders(account, []FolderInfo{{Name: "INBOX", Delim: "/"}}); err != nil {
			t.Fatalf("SaveFolders(%s): %v", account, err)
		}
		if err := db.SaveEmails(account, "INBOX", []EmailSummary{{UID: 1, Subject: "hello"}}); err != nil {
			t.Fatalf("SaveEmails(%s): %v", account, err)
		}
		if err := db.SaveEmailBody(account, "INBOX", &EmailDetail{UID: 1, Subject: "hello", BodyText: "body"}); err != nil {
			t.Fatalf("SaveEmailBody(%s): %v", account, err)
		}
		if err := db.SaveFolderMeta(account, "INBOX", FolderMeta{Total: 1, UIDNext: 2}); err != nil {
			t.Fatalf("SaveFolderMeta(%s): %v", account, err)
		}
		if err := db.SaveFlaggedSnapshot(account, []EmailSummary{{UID: 1}}, 1); err != nil {
			t.Fatalf("SaveFlaggedSnapshot(%s): %v", account, err)
		}
	}

	deleted, err := db.DeleteAccountCache("gone")
	if err != nil {
		t.Fatalf("DeleteAccountCache: %v", err)
	}
	// One folder + one email + one body + one meta + one starred snapshot.
	if deleted != 5 {
		t.Errorf("deleted rows = %d, want 5", deleted)
	}

	if folders, err := db.LoadFolders("gone"); err != nil {
		t.Fatalf("LoadFolders(gone): %v", err)
	} else if len(folders) != 0 {
		t.Errorf("folders left for the deleted account: %v", folders)
	}
	if emails, err := db.LoadEmails("gone", "INBOX"); err != nil {
		t.Fatalf("LoadEmails(gone): %v", err)
	} else if len(emails) != 0 {
		t.Errorf("emails left for the deleted account: %v", emails)
	}
	if _, err := db.LoadEmailBody("gone", "INBOX", 1); err == nil {
		t.Error("cached body survived the purge")
	}
	if meta, err := db.LoadFolderMeta("gone", "INBOX"); err != nil {
		t.Fatalf("LoadFolderMeta(gone): %v", err)
	} else if meta != (FolderMeta{}) {
		t.Errorf("folder meta left for the deleted account: %+v", meta)
	}

	// The other account is untouched: purging is a delete-account action, not a
	// "clear the cache" button.
	if emails, err := db.LoadEmails("kept", "INBOX"); err != nil {
		t.Fatalf("LoadEmails(kept): %v", err)
	} else if len(emails) != 1 {
		t.Errorf("kept account lost its mail: %v", emails)
	}
	if folders, err := db.LoadFolders("kept"); err != nil {
		t.Fatalf("LoadFolders(kept): %v", err)
	} else if len(folders) != 1 {
		t.Errorf("kept account lost its folders: %v", folders)
	}
}

// Deleting an account that was never cached must succeed with nothing to do.
func TestDeleteAccountCacheUnknownAccount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	deleted, err := db.DeleteAccountCache("never-cached")
	if err != nil {
		t.Fatalf("DeleteAccountCache: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted rows = %d, want 0", deleted)
	}
}
