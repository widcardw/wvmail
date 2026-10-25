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

// With no cached folder list the folder refresh is part of the health sweep: its
// dial failure must reach the caller instead of being flattened into an empty,
// apparently successful map.
func TestFolderStatusesWithoutCachedFolders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	service := &EmailService{db: NewDBManager(), health: newAccountHealthReporter()}
	_, err := service.FolderStatuses(EmailAccount{ID: "acct"})
	if err == nil {
		t.Fatal("FolderStatuses must report the folder refresh failure")
	}
	if events := service.health.events(); len(events) != 1 {
		t.Fatalf("expected one failure event, got %v", events)
	} else if events[0].Outcome != AccountHealthOutcomeFailure {
		t.Fatalf("expected a failure event, got %+v", events[0])
	}
}
