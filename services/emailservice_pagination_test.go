package services

import "testing"

func TestNormalizePageDefaultsAndKeepsValidInput(t *testing.T) {
	if page, size := normalizePage(0, 0); page != 1 || size != 20 {
		t.Fatalf("normalizePage(0, 0) = (%d, %d), want (1, 20)", page, size)
	}
	if page, size := normalizePage(-3, -1); page != 1 || size != 20 {
		t.Fatalf("normalizePage(-3, -1) = (%d, %d), want (1, 20)", page, size)
	}
	if page, size := normalizePage(2, 50); page != 2 || size != 50 {
		t.Fatalf("normalizePage(2, 50) = (%d, %d), want (2, 50)", page, size)
	}
}

// Paging beyond the first page must stay on the cache whenever it can fill the
// page: falling through to the server costs a full IMAP connect+login+fetch per
// flip (~0.5s), which is what made page 2 feel slow.
func TestCacheCoversPage(t *testing.T) {
	cases := []struct {
		name                  string
		page, pageSize, total int
		cached                int
		want                  bool
	}{
		{"page 2 fully cached", 2, 20, 100, 40, true},
		{"page 2 one row short", 2, 20, 100, 39, false},
		{"page 2 exactly at the boundary", 2, 20, 40, 40, true},
		{"page 3 fully cached", 3, 20, 100, 60, true},
		{"last page needs only the remaining rows", 3, 20, 55, 55, true},
		{"last page short by one", 3, 20, 55, 54, false},
		{"page past the end still needs the whole folder", 9, 20, 55, 55, true},
		{"empty cache never covers", 1, 20, 100, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cacheCoversPage(tc.page, tc.pageSize, tc.total, tc.cached)
			if got != tc.want {
				t.Errorf("cacheCoversPage(page=%d, size=%d, total=%d, cached=%d) = %v, want %v",
					tc.page, tc.pageSize, tc.total, tc.cached, got, tc.want)
			}
		})
	}
}

// The cache is ordered newest-first, so slicing it produces the same page the
// server would.
func TestPaginateSlicesCachedOrder(t *testing.T) {
	summaries := []EmailSummary{{UID: 5}, {UID: 4}, {UID: 3}, {UID: 2}, {UID: 1}}

	page2 := paginateSlices(summaries, 2, 2)
	if len(page2) != 2 || page2[0].UID != 3 || page2[1].UID != 2 {
		t.Fatalf("page 2 = %+v, want UID 3 then 2", page2)
	}
	last := paginateSlices(summaries, 3, 2)
	if len(last) != 1 || last[0].UID != 1 {
		t.Fatalf("last page = %+v, want UID 1", last)
	}
	if got := paginateSlices(summaries, 4, 2); len(got) != 0 {
		t.Fatalf("page past the end = %+v, want empty", got)
	}
}

func TestNewEmailsSinceUIDNext(t *testing.T) {
	emails := []EmailSummary{
		{UID: 42, Subject: "previous"},
		{UID: 43, Subject: "new"},
		{UID: 44, Subject: "newest"},
	}

	got := newEmailsSinceUIDNext(emails, 43)
	if len(got) != 2 || got[0].UID != 43 || got[1].UID != 44 {
		t.Fatalf("newEmailsSinceUIDNext = %+v, want UIDs 43 and 44", got)
	}
	if got := newEmailsSinceUIDNext(emails, 0); len(got) != 0 {
		t.Fatalf("zero UIDNEXT must not classify existing mail as new: %+v", got)
	}
}

func TestNewMailNotificationOptions(t *testing.T) {
	account := EmailAccount{ID: "account-1", Email: "me@example.com"}
	email := EmailSummary{
		UID:      43,
		FromName: "Sender",
		Subject:  "Project update",
	}

	got := newMailNotificationOptions("zh-CN", account, email)
	if got.Title != "新邮件" {
		t.Errorf("Title = %q, want Chinese title", got.Title)
	}
	if got.Body != "Sender\nProject update" {
		t.Errorf("Body = %q, want sender and subject", got.Body)
	}
	if got.ID != "mail-account-1-43" {
		t.Errorf("ID = %q, want stable account/UID ID", got.ID)
	}
}

func TestMailNotificationSuppressionPolicy(t *testing.T) {
	sender := NewMailNotificationSender(nil, nil)
	if sender.isSuppressed() {
		t.Fatal("notifications should not be suppressed without a policy")
	}

	focused := true
	sender.SetShouldNotify(func() bool { return focused })
	if !sender.isSuppressed() {
		t.Fatal("focused main window should suppress notifications")
	}

	focused = false
	if sender.isSuppressed() {
		t.Fatal("unfocused main window should allow notifications")
	}
}
