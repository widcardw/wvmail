package services

import (
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
)

func folderNames(folders []FolderInfo) []string {
	names := make([]string, len(folders))
	for i, folder := range folders {
		names[i] = folder.Name
	}
	return names
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFlaggedSearchFoldersPrefersSpecialUseMailboxes(t *testing.T) {
	folders := []FolderInfo{
		{Name: "INBOX", Attrs: []string{}},
		{Name: "All Mail", Attrs: []string{string(imap.MailboxAttrAll)}},
		{Name: "Starred", Attrs: []string{string(imap.MailboxAttrFlagged)}},
	}
	got := folderNames(flaggedSearchFolders(folders))
	if !equalStrings(got, []string{"Starred"}) {
		t.Fatalf("flagged folders = %v, want [Starred]", got)
	}

	folders = folders[:2]
	got = folderNames(flaggedSearchFolders(folders))
	if !equalStrings(got, []string{"All Mail"}) {
		t.Fatalf("fallback folders = %v, want [All Mail]", got)
	}
}

func TestFlaggedSearchFoldersSkipsJunkAndTrash(t *testing.T) {
	folders := []FolderInfo{
		{Name: "INBOX"},
		{Name: "Archive"},
		{Name: "Junk", Attrs: []string{string(imap.MailboxAttrJunk)}},
		{Name: "Trash", Attrs: []string{string(imap.MailboxAttrTrash)}},
		{Name: "Spam"},
		{Name: "Deleted Items"},
		{Name: "Virtual", Attrs: []string{string(imap.MailboxAttrNoSelect)}},
	}
	got := folderNames(flaggedSearchFolders(folders))
	if !equalStrings(got, []string{"INBOX", "Archive"}) {
		t.Fatalf("search folders = %v, want [INBOX Archive]", got)
	}
}

func TestFlaggedMessageKeyDoesNotMergeMissingMessageIDs(t *testing.T) {
	first := EmailSummary{MessageID: "<same@example.com>", FolderName: "INBOX", UID: 1}
	copyInArchive := EmailSummary{MessageID: "<SAME@example.com>", FolderName: "Archive", UID: 8}
	if flaggedMessageKey(first) != flaggedMessageKey(copyInArchive) {
		t.Fatal("the same Message-ID in different folders must deduplicate")
	}

	first.MessageID = ""
	copyInArchive.MessageID = ""
	if flaggedMessageKey(first) == flaggedMessageKey(copyInArchive) {
		t.Fatal("messages without Message-ID must not collapse across folders")
	}
}

func TestFlagsWithAddsAndRemovesFlagged(t *testing.T) {
	got := flagsWith([]string{`\Seen`, `\FLAGGED`}, `\Flagged`, false)
	if !equalStrings(got, []string{`\Seen`}) {
		t.Fatalf("flags after removal = %v, want [\\Seen]", got)
	}

	got = flagsWith(got, `\Flagged`, true)
	if !equalStrings(got, []string{`\Seen`, `\Flagged`}) {
		t.Fatalf("flags after add = %v, want [\\Seen \\Flagged]", got)
	}
}

func TestFlaggedSnapshotRoundTripAndInvalidation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	fetchedAt := time.Now().UnixMilli()
	want := []EmailSummary{
		{UID: 7, MessageID: "starred@example.com", Subject: "starred", FolderName: "INBOX"},
	}
	if err := db.SaveFlaggedSnapshot("acct", want, fetchedAt); err != nil {
		t.Fatalf("SaveFlaggedSnapshot: %v", err)
	}

	got, gotFetchedAt, err := db.LoadFlaggedSnapshot("acct")
	if err != nil {
		t.Fatalf("LoadFlaggedSnapshot: %v", err)
	}
	if gotFetchedAt != fetchedAt || len(got) != 1 || got[0].Subject != "starred" {
		t.Fatalf("snapshot round trip = %+v, fetchedAt=%d", got, gotFetchedAt)
	}

	if err := db.InvalidateFlaggedSnapshot("acct"); err != nil {
		t.Fatalf("InvalidateFlaggedSnapshot: %v", err)
	}
	got, gotFetchedAt, err = db.LoadFlaggedSnapshot("acct")
	if err != nil {
		t.Fatalf("LoadFlaggedSnapshot (invalidated): %v", err)
	}
	if gotFetchedAt != 0 || len(got) != 1 {
		t.Fatalf("invalidated snapshot must keep rows and zero freshness: %+v, fetchedAt=%d", got, gotFetchedAt)
	}
}
