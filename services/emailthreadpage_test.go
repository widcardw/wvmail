package services

import "testing"

func TestThreadPageResponsePaginatesConversationsNotMessages(t *testing.T) {
	emails := []EmailSummary{
		{UID: 5, ThreadID: "a", ThreadCount: 3},
		{UID: 4, ThreadID: "a", ThreadCount: 3},
		{UID: 3, ThreadID: "a", ThreadCount: 3},
		{UID: 2, ThreadID: "b", ThreadCount: 1},
		{UID: 1, ThreadID: "c", ThreadCount: 1},
	}

	page1 := threadPageResponse(emails, 5, 6, 1, 2, true)
	if page1.ThreadTotal != 3 {
		t.Fatalf("thread total = %d, want 3", page1.ThreadTotal)
	}
	if len(page1.Emails) != 2 || page1.Emails[0].ThreadID != "a" || page1.Emails[1].ThreadID != "b" {
		t.Fatalf("page 1 = %+v, want threads a then b", page1.Emails)
	}
	if page1.Emails[0].UID != 5 {
		t.Fatalf("thread lead UID = %d, want the newest message 5", page1.Emails[0].UID)
	}

	page2 := threadPageResponse(emails, 5, 6, 2, 2, true)
	if len(page2.Emails) != 1 || page2.Emails[0].ThreadID != "c" {
		t.Fatalf("page 2 = %+v, want only thread c", page2.Emails)
	}
}

func TestThreadPageIsReadyRequiresLookaheadWhenCacheIsPartial(t *testing.T) {
	emails := []EmailSummary{
		{UID: 3, ThreadID: "a"},
		{UID: 2, ThreadID: "b"},
		{UID: 1, ThreadID: "c"},
	}

	if threadPageIsReady(emails, 10, 1, 3) {
		t.Fatal("exactly one page of rows in a partial cache must still fetch to prove whether a next page exists")
	}
	if !threadPageIsReady(emails, 3, 1, 3) {
		t.Fatal("a complete cache is always ready")
	}
	if !threadPageIsReady(emails, 10, 1, 2) {
		t.Fatal("three rows exceed the one-page boundary and prove a next page exists")
	}
}
