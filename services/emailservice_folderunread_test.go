package services

import "testing"

// The sidebar badges every folder, so the count list must not fall over the container
// folders a server reports: a \Noselect branch has no messages, and a STATUS for it is an
// error that would otherwise mask the folders that do have unread mail.
func TestIsSelectableMailbox(t *testing.T) {
	cases := []struct {
		name  string
		attrs []string
		want  bool
	}{
		{"plain folder", nil, true},
		{"regular attributes", []string{`\HasChildren`, `\HasNoChildren`}, true},
		{"noselect branch", []string{`\Noselect`, `\HasChildren`}, false},
		{"case insensitive", []string{`\noselect`}, false},
		{"non-existent", []string{`\NonExistent`}, false},
		{"marker inside a longer attribute", []string{`\NoSelectContainer`}, false},
	}
	for _, tc := range cases {
		if got := isSelectableMailbox(tc.attrs); got != tc.want {
			t.Errorf("%s: isSelectableMailbox(%v) = %v, want %v", tc.name, tc.attrs, got, tc.want)
		}
	}
}

// With no cached folder list there is nothing to ask about, so the call must come back
// empty without opening a connection (the account below has no host to dial).
func TestFolderUnreadWithoutCachedFolders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	service := &EmailService{db: NewDBManager()}
	counts, err := service.FolderUnread(EmailAccount{ID: "acct"})
	if err != nil {
		t.Fatalf("FolderUnread: %v", err)
	}
	if counts == nil {
		t.Fatal("FolderUnread must return a map, not nil: the caller indexes into it")
	}
	if len(counts) != 0 {
		t.Errorf("expected no counts, got %v", counts)
	}
}
