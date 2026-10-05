package services

import "testing"

// The spam gate must not depend on which code path served the message: a body that is
// already cached carries its flags but not the verdict, and a spam message whose gate
// silently disappeared on the second open is exactly the bug this pins down.
func TestIsSuspiciousMail(t *testing.T) {
	cases := []struct {
		name   string
		folder string
		flags  []string
		want   bool
	}{
		{"junk folder", "Junk", nil, true},
		{"lowercase folder", "junk", nil, true},
		{"spam folder", "INBOX.Spam", nil, true},
		{"localised folder", "垃圾邮件", nil, true},
		{"junk flag", "INBOX", []string{`\Seen`, `\Junk`}, true},
		{"junk flag any case", "INBOX", []string{`\junk`}, true},
		{"ordinary inbox mail", "INBOX", []string{`\Seen`}, false},
		{"ordinary folder", "Sent Messages", nil, false},
		// "Junk"-lookalikes must not trip it: only the folder name is matched.
		{"folder substring is not enough", "INBOX", []string{`\Junked`}, false},
	}
	for _, tc := range cases {
		if got := isSuspiciousMail(tc.folder, tc.flags); got != tc.want {
			t.Errorf("%s: isSuspiciousMail(%q, %v) = %v, want %v", tc.name, tc.folder, tc.flags, got, tc.want)
		}
	}
}

// FetchEmail prefers the cache, so the verdict has to be recomputed there too. The cache
// is pre-filled, which also proves the cache hit needs no network connection.
func TestFetchEmailKeepsJunkVerdictFromCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	service := &EmailService{db: db}
	account := EmailAccount{ID: "acct", Email: "user@example.com"}

	const folder = "INBOX"
	const uid = 42

	// Cached from an earlier fetch of a message the server flagged as junk.
	if err := db.SaveEmailBody(account.ID, folder, &EmailDetail{
		UID:      uid,
		Subject:  "cheap pills",
		BodyHTML: "<p>buy now</p>",
		Flags:    []string{`\Seen`, `\Junk`},
	}); err != nil {
		t.Fatalf("SaveEmailBody: %v", err)
	}

	detail, err := service.FetchEmail(account, folder, uid)
	if err != nil {
		t.Fatalf("FetchEmail: %v", err)
	}
	if !detail.IsSuspicious {
		t.Error("a cached body with the \\Junk flag must still be marked suspicious")
	}

	// Same for the folder rule when the message lives in a Junk mailbox.
	if err := db.SaveEmailBody(account.ID, "Junk", &EmailDetail{
		UID:      uid + 1,
		Subject:  "lottery",
		BodyHTML: "<p>you won</p>",
	}); err != nil {
		t.Fatalf("SaveEmailBody: %v", err)
	}
	junk, err := service.FetchEmail(account, "Junk", uid+1)
	if err != nil {
		t.Fatalf("FetchEmail (Junk): %v", err)
	}
	if !junk.IsSuspicious {
		t.Error("a cached body opened from Junk must be marked suspicious")
	}

	// And an ordinary cached message stays ordinary.
	if err := db.SaveEmailBody(account.ID, folder, &EmailDetail{
		UID:      uid + 2,
		Subject:  "meeting",
		BodyHTML: "<p>agenda</p>",
		Flags:    []string{`\Seen`},
	}); err != nil {
		t.Fatalf("SaveEmailBody: %v", err)
	}
	normal, err := service.FetchEmail(account, folder, uid+2)
	if err != nil {
		t.Fatalf("FetchEmail (normal): %v", err)
	}
	if normal.IsSuspicious {
		t.Error("an ordinary message must not be marked suspicious")
	}
}
