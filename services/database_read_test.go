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
