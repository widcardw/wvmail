package services

import (
	"testing"
	"time"
)

// A cached date keeps the sender's own UTC offset, so ordering the column as a
// string orders clock faces instead of instants. The reported symptom: a mail
// sent at 21:32 +08:00 (13:32 UTC) sat above two sent at 22:49 and 22:51 local
// (14:49 and 14:51 UTC) because "21:32" > "14:49" as text.

// The pair that exposed it, and a third row to prove the whole list is ordered.
func TestLoadEmailsOrdersByInstantNotByClockFace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{
		// Sent from China at 21:32 local time — the mail that used to stay on top.
		{UID: 3802, Subject: "login alert", Date: "2026-10-24T21:32:01+08:00"},
		{UID: 3803, Subject: "reset password", Date: "2026-10-24T14:49:33Z"},
		{UID: 3804, Subject: "cancel withdrawal", Date: "2026-10-24T14:51:46Z"},
	}); err != nil {
		t.Fatalf("SaveEmails: %v", err)
	}

	got, err := db.LoadEmails("acct", "INBOX")
	if err != nil {
		t.Fatalf("LoadEmails: %v", err)
	}
	want := []uint32{3804, 3803, 3802}
	if len(got) != len(want) {
		t.Fatalf("got %d emails, want %d", len(got), len(want))
	}
	for i, uid := range want {
		if got[i].UID != uid {
			t.Fatalf("position %d is uid %d (%s), want uid %d — order: %v",
				i, got[i].UID, got[i].Date, uid, uidsOf(got))
		}
	}
}

// Search results are shown in the same list, so they need the same ordering.
func TestSearchEmailsOrdersByInstant(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{
		{UID: 1, Subject: "digest", FromAddr: "bot@example.com", Date: "2026-10-24T21:32:01+08:00"},
		{UID: 2, Subject: "digest", FromAddr: "bot@example.com", Date: "2026-10-24T14:51:46Z"},
	}); err != nil {
		t.Fatalf("SaveEmails: %v", err)
	}

	got, err := db.SearchEmails(EmailSearchQuery{
		Filters: []EmailSearchFilter{{Field: "content", Value: "digest"}},
	})
	if err != nil {
		t.Fatalf("SearchEmails: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
	if got[0].UID != 2 {
		t.Fatalf("first result is uid %d, want uid 2 (14:51Z is later than 21:32+08:00)", got[0].UID)
	}
}

// Contact suggestions keep the first `limit` rows, so "recent senders first" has
// to mean recent by the clock — otherwise a Chinese sender shadows a newer one.
func TestSuggestEmailContactsPrefersTheRecentSender(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{
		{UID: 1, FromAddr: "older@example.com", Date: "2026-10-24T21:32:01+08:00"},
		{UID: 2, FromAddr: "newer@example.com", Date: "2026-10-24T14:51:46Z"},
	}); err != nil {
		t.Fatalf("SaveEmails: %v", err)
	}

	got, err := db.SuggestEmailContacts("from", "example.com", "acct", 1)
	if err != nil {
		t.Fatalf("SuggestEmailContacts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(got))
	}
	if got[0].Address != "newer@example.com" {
		t.Fatalf("suggested %q, want the newer sender", got[0].Address)
	}
}

// A batch of notifications can share one Date header to the second; within it
// the later arrival is the newer message.
func TestSortEmailsNewestFirstTieBreaksOnArrival(t *testing.T) {
	emails := []EmailSummary{
		{UID: 10, Date: "2026-10-18T04:04:05Z"},
		{UID: 12, Date: "2026-10-18T04:04:05Z"},
		{UID: 11, Date: "2026-10-18T04:04:05Z"},
	}
	sortEmailsNewestFirst(emails)

	want := []uint32{12, 11, 10}
	for i, uid := range want {
		if emails[i].UID != uid {
			t.Fatalf("position %d is uid %d, want %d — order: %v", i, emails[i].UID, uid, uidsOf(emails))
		}
	}
}

// A message without a usable Date header must sink, not jump to the top as the
// zero time would if the comparison ignored the "has a date" flag.
func TestSortEmailsNewestFirstKeepsUndatedAtTheBottom(t *testing.T) {
	emails := []EmailSummary{
		{UID: 1, Date: ""},
		{UID: 2, Date: "2026-10-18T04:04:05Z"},
		{UID: 3, Date: "not a date"},
	}
	sortEmailsNewestFirst(emails)

	if emails[0].UID != 2 {
		t.Fatalf("first is uid %d, want the dated one — order: %v", emails[0].UID, uidsOf(emails))
	}
	if emails[len(emails)-1].UID != 1 && emails[len(emails)-1].UID != 3 {
		t.Fatalf("undated rows must sink, order: %v", uidsOf(emails))
	}
}

func TestParseEmailTimeReadsOffsets(t *testing.T) {
	china, ok := parseEmailTime("2026-10-24T21:32:01+08:00")
	if !ok {
		t.Fatal("a +08:00 timestamp must parse")
	}
	utc, ok := parseEmailTime("2026-10-24T14:51:46Z")
	if !ok {
		t.Fatal("a Z timestamp must parse")
	}
	if !utc.After(china) {
		t.Fatalf("14:51Z must be later than 21:32+08:00 (got %v vs %v)", utc, china)
	}
	if got := utc.Sub(china).Minutes(); got != 79.75 {
		t.Fatalf("gap is %.2f minutes, want 79.75", got)
	}

	if _, ok := parseEmailTime(""); ok {
		t.Error("an empty date must report no timestamp")
	}
	if _, ok := parseEmailTime("2026-10-24 21:32"); ok {
		t.Error("a non-RFC3339 string must report no timestamp")
	}
}

func TestResolveEmailDateFallsBackFromImplausibleFutureDate(t *testing.T) {
	now := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)
	envelopeDate := time.Date(2612, time.January, 18, 15, 55, 30, 0, time.UTC)
	internalDate := time.Date(2026, time.October, 31, 8, 42, 31, 0, time.UTC)

	got := resolveEmailDate(envelopeDate, internalDate, now)
	const want = "2026-10-31T08:42:31Z"
	if got != want {
		t.Fatalf("resolved date = %q, want InternalDate %q", got, want)
	}
}

func TestResolveEmailDateKeepsPlausibleSenderDate(t *testing.T) {
	now := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)
	envelopeDate := time.Date(2026, time.October, 31, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	internalDate := time.Date(2026, time.October, 31, 2, 0, 0, 0, time.UTC)

	got := resolveEmailDate(envelopeDate, internalDate, now)
	const want = "2026-10-31T01:30:00Z"
	if got != want {
		t.Fatalf("resolved date = %q, want sender Date %q", got, want)
	}
}

func TestResolveEmailDateRejectsImplausibleCandidates(t *testing.T) {
	now := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)
	got := resolveEmailDate(
		time.Date(2612, time.January, 18, 15, 55, 30, 0, time.UTC),
		time.Date(1969, time.December, 31, 23, 59, 59, 0, time.UTC),
		now,
	)
	if got != "" {
		t.Fatalf("resolved date = %q, want an empty undated value", got)
	}
}

func TestSanitizeCachedEmailDateHidesImplausibleFutureDate(t *testing.T) {
	now := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)
	if got := sanitizeCachedEmailDate("2612-01-18T15:55:30Z", now); got != "" {
		t.Fatalf("cached future date = %q, want empty", got)
	}
	if got := sanitizeCachedEmailDate("2026-10-31T08:42:31Z", now); got != "2026-10-31T08:42:31Z" {
		t.Fatalf("cached plausible date = %q, want unchanged", got)
	}
}

func uidsOf(emails []EmailSummary) []uint32 {
	uids := make([]uint32, len(emails))
	for i, email := range emails {
		uids[i] = email.UID
	}
	return uids
}
