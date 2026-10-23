package services

import "testing"

// Opening a message flips the cached read state: the list styling and the unread
// badge both read this column, so it has to change without waiting for a refetch.
func TestMarkEmailReadUpdatesCache(t *testing.T) {
	// Keep the test away from the real ~/Library/Application Support/wvmail cache.
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{
		{UID: 1, Subject: "unread", IsRead: false},
		{UID: 2, Subject: "read", IsRead: true},
	}); err != nil {
		t.Fatalf("SaveEmails: %v", err)
	}

	if err := db.MarkEmailRead("acct", "INBOX", 1); err != nil {
		t.Fatalf("MarkEmailRead: %v", err)
	}

	got, err := db.LoadEmails("acct", "INBOX")
	if err != nil {
		t.Fatalf("LoadEmails: %v", err)
	}
	read := make(map[uint32]bool, len(got))
	for _, email := range got {
		read[email.UID] = email.IsRead
	}
	if !read[1] {
		t.Error("uid 1 must be read after MarkEmailRead")
	}
	if !read[2] {
		t.Error("uid 2 must stay read")
	}

	// A message that is not cached (aggregate/search views reach the same call) is a
	// no-op, not an error: the server still gets the flag.
	if err := db.MarkEmailRead("acct", "INBOX", 999); err != nil {
		t.Errorf("marking an uncached message must not fail: %v", err)
	}
}

// The cache is per account+folder, so marking a message read in one folder must not
// touch the same UID elsewhere.
func TestMarkEmailReadIsScopedToFolder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	for _, folder := range []string{"INBOX", "Archive"} {
		if err := db.SaveEmails("acct", folder, []EmailSummary{{UID: 7, IsRead: false}}); err != nil {
			t.Fatalf("SaveEmails(%s): %v", folder, err)
		}
	}

	if err := db.MarkEmailRead("acct", "INBOX", 7); err != nil {
		t.Fatalf("MarkEmailRead: %v", err)
	}

	archive, err := db.LoadEmails("acct", "Archive")
	if err != nil {
		t.Fatalf("LoadEmails: %v", err)
	}
	if len(archive) != 1 || archive[0].IsRead {
		t.Errorf("Archive copy must stay unread, got %+v", archive)
	}
}

// UIDNEXT has to survive the cache round trip: the frontend compares it against a
// later STATUS to decide whether opening a folder needs a fetch at all, so a value
// that resets to 0 on restart would make every folder open dial IMAP once more.
func TestFolderMetaKeepsUIDNext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	if err := db.SaveFolderMeta("acct", "INBOX", FolderMeta{Total: 42, UIDNext: 1087}); err != nil {
		t.Fatalf("SaveFolderMeta: %v", err)
	}

	meta, err := db.LoadFolderMeta("acct", "INBOX")
	if err != nil {
		t.Fatalf("LoadFolderMeta: %v", err)
	}
	if meta.Total != 42 || meta.UIDNext != 1087 {
		t.Errorf("got %+v, want {Total:42 UIDNext:1087}", meta)
	}

	// A second fetch replaces the row rather than stacking a new one.
	if err := db.SaveFolderMeta("acct", "INBOX", FolderMeta{Total: 43, UIDNext: 1088}); err != nil {
		t.Fatalf("SaveFolderMeta (2nd): %v", err)
	}
	meta, err = db.LoadFolderMeta("acct", "INBOX")
	if err != nil {
		t.Fatalf("LoadFolderMeta (2nd): %v", err)
	}
	if meta.Total != 43 || meta.UIDNext != 1088 {
		t.Errorf("got %+v, want {Total:43 UIDNext:1088}", meta)
	}

	// Nothing cached yet reads as the zero value, which callers treat as "unknown".
	empty, err := db.LoadFolderMeta("acct", "Archive")
	if err != nil {
		t.Fatalf("LoadFolderMeta (missing): %v", err)
	}
	if empty != (FolderMeta{}) {
		t.Errorf("missing folder must read as zero meta, got %+v", empty)
	}
}
