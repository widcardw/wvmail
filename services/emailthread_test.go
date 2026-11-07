package services

import (
	"testing"
)

func TestApplyEmailThreadsUsesReferencesAndMergesOrphans(t *testing.T) {
	emails := []EmailSummary{
		{UID: 3, MessageID: "<reply@example.com>", References: []string{"<root@example.com>", "<second@example.com>"}, Subject: "Re: Project", FromAddr: "b@example.com", To: []string{"a@example.com"}, Date: "2026-11-03T10:00:00Z", IsRead: true},
		{UID: 2, MessageID: "<second@example.com>", InReplyTo: []string{"<root@example.com>"}, Subject: "Re: Project", FromAddr: "a@example.com", To: []string{"b@example.com"}, Date: "2026-11-02T10:00:00Z"},
		{UID: 1, MessageID: "<root@example.com>", Subject: "Project", FromAddr: "a@example.com", To: []string{"b@example.com"}, Date: "2026-11-01T10:00:00Z", IsRead: true},
	}

	applyEmailThreads(emails)

	if emails[0].ThreadID == "" || emails[0].ThreadID != emails[1].ThreadID || emails[1].ThreadID != emails[2].ThreadID {
		t.Fatalf("references did not merge the thread: %+v", emails)
	}
	for _, email := range emails {
		if email.ThreadCount != 3 {
			t.Errorf("UID %d thread count = %d, want 3", email.UID, email.ThreadCount)
		}
		if email.ThreadUnread != 1 {
			t.Errorf("UID %d thread unread = %d, want 1", email.UID, email.ThreadUnread)
		}
	}
}

func TestApplyEmailThreadsKeepsStrongLinksOutsideWeakWindow(t *testing.T) {
	emails := []EmailSummary{
		{UID: 2, MessageID: "<late@example.com>", InReplyTo: []string{"<root@example.com>"}, Subject: "Re: Quarterly", FromAddr: "b@example.com", Date: "2027-03-01T10:00:00Z"},
		{UID: 1, MessageID: "<root@example.com>", Subject: "Quarterly", FromAddr: "a@example.com", Date: "2026-01-01T10:00:00Z"},
	}

	applyEmailThreads(emails)

	if emails[0].ThreadID != emails[1].ThreadID {
		t.Fatalf("a real reply must merge even months later: %+v", emails)
	}
}

func TestApplyEmailThreadsMergesRepliesWhoseParentIsNotCached(t *testing.T) {
	emails := []EmailSummary{
		{UID: 2, MessageID: "<second-reply@example.com>", References: []string{"<root@example.com>", "<first-reply@example.com>"}, Subject: "Re: Offline root", Date: "2026-11-02T10:00:00Z"},
		{UID: 1, MessageID: "<first-reply@example.com>", References: []string{"<root@example.com>"}, Subject: "Re: Offline root", Date: "2026-11-01T10:00:00Z"},
	}

	applyEmailThreads(emails)

	if emails[0].ThreadID != emails[1].ThreadID {
		t.Fatalf("shared missing root did not merge the replies: %+v", emails)
	}
	if emails[0].ThreadID != "mid:root@example.com" {
		t.Fatalf("thread ID = %q, want the shared root", emails[0].ThreadID)
	}
}

func TestWeakThreadMatchHonoursWindowAndSubjectRules(t *testing.T) {
	base := EmailSummary{
		MessageID: "<one@example.com>",
		Subject:   "Re: Status update",
		FromAddr:  "alice@example.com",
		To:        []string{"me@example.com"},
		Date:      "2026-11-01T10:00:00Z",
	}
	within := EmailSummary{
		MessageID: "<two@example.com>",
		Subject:   "Status update",
		FromAddr:  "me@example.com",
		To:        []string{"alice@example.com"},
		Date:      "2026-11-15T10:00:00Z",
	}
	outside := within
	outside.Date = "2026-11-15T10:00:01Z"

	if !weakThreadMatch(base, within) {
		t.Fatal("same subject and participants within 14 days should match")
	}
	if weakThreadMatch(base, outside) {
		t.Fatal("a subject-only match outside 14 days must not merge")
	}

	empty := base
	empty.Subject = "Re:"
	if weakThreadMatch(empty, within) {
		t.Fatal("an empty normalized subject must not merge")
	}

	bulk := within
	bulk.IsBulk = true
	if weakThreadMatch(base, bulk) {
		t.Fatal("bulk mail must not be merged by subject alone")
	}
}

func TestWeakThreadMatchDoesNotOverrideRealHeaders(t *testing.T) {
	a := EmailSummary{
		MessageID: "<one@example.com>",
		InReplyTo: []string{"<another-thread@example.com>"},
		Subject:   "Re: Same title",
		FromAddr:  "alice@example.com",
		To:        []string{"me@example.com"},
		Date:      "2026-11-01T10:00:00Z",
	}
	b := EmailSummary{
		MessageID: "<two@example.com>",
		Subject:   "Same title",
		FromAddr:  "me@example.com",
		To:        []string{"alice@example.com"},
		Date:      "2026-11-02T10:00:00Z",
	}
	if weakThreadMatch(a, b) {
		t.Fatal("weak matching must not override a message with real threading headers")
	}
}

func TestNormalizeThreadSubject(t *testing.T) {
	cases := map[string]string{
		"Re: Hello":       "hello",
		"RE[2]:  Hello  ": "hello",
		"Fwd: Re: 回复：你好":  "你好",
		"转发: 项目进展":        "项目进展",
		"  普通 主题  ":       "普通 主题",
	}
	for input, want := range cases {
		if got := normalizeThreadSubject(input); got != want {
			t.Errorf("normalizeThreadSubject(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWeakThreadIDsDoNotCollideAcrossDifferentParticipants(t *testing.T) {
	emails := []EmailSummary{
		{UID: 2, Subject: "Weekly note", FromAddr: "alice@example.com", To: []string{"carol@example.com"}, Date: "2026-11-02T10:00:00Z"},
		{UID: 1, Subject: "Weekly note", FromAddr: "alice@example.com", To: []string{"bob@example.com"}, Date: "2026-11-01T10:00:00Z"},
	}

	applyEmailThreads(emails)

	if emails[0].ThreadID == emails[1].ThreadID {
		t.Fatalf("separate subject-only conversations share thread ID %q", emails[0].ThreadID)
	}
}

func TestListThreadEmailsReturnsOldestFirst(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	db := NewDBManager()
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{
		{UID: 1, MessageID: "<root@example.com>", Subject: "Topic", Date: "2026-11-01T10:00:00Z"},
		{UID: 2, MessageID: "<reply@example.com>", InReplyTo: []string{"<root@example.com>"}, Subject: "Re: Topic", Date: "2026-11-02T10:00:00Z"},
		{UID: 3, MessageID: "<other@example.com>", Subject: "Other", Date: "2026-11-03T10:00:00Z"},
	}); err != nil {
		t.Fatalf("SaveEmails: %v", err)
	}

	service := &EmailService{db: db}
	thread, err := service.ListThreadEmails(EmailAccount{ID: "acct"}, "INBOX", "mid:root@example.com")
	if err != nil {
		t.Fatalf("ListThreadEmails: %v", err)
	}
	if len(thread) != 2 || thread[0].UID != 1 || thread[1].UID != 2 {
		t.Fatalf("thread = %+v, want UID 1 then 2", thread)
	}
}

func TestSaveEmailsKeepsCachedThreadingHeadersWhenRefreshOmitsThem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	db := NewDBManager()
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{{
		UID:        1,
		MessageID:  "<reply@example.com>",
		InReplyTo:  []string{"<root@example.com>"},
		References: []string{"<root@example.com>"},
		Subject:    "Re: Topic",
	}}); err != nil {
		t.Fatalf("first SaveEmails: %v", err)
	}
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{{
		UID:       1,
		MessageID: "<reply@example.com>",
		Subject:   "Re: Topic",
	}}); err != nil {
		t.Fatalf("second SaveEmails: %v", err)
	}

	emails, err := db.LoadEmails("acct", "INBOX")
	if err != nil {
		t.Fatalf("LoadEmails: %v", err)
	}
	if len(emails) != 1 || len(emails[0].References) != 1 || emails[0].References[0] != "<root@example.com>" {
		t.Fatalf("threading headers were erased: %+v", emails)
	}
}
