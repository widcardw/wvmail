package services

import (
	"bytes"
	"net"
	"strconv"
	"testing"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

func TestFindTrashFolderPrefersSpecialUse(t *testing.T) {
	folders := []FolderInfo{
		{Name: "INBOX", Delim: "/"},
		{Name: "Archive/Deleted", Delim: "/"},
		{Name: "[Gmail]/Trash", Delim: "/", Attrs: []string{string(imap.MailboxAttrTrash)}},
	}

	got, ok := findTrashFolder(folders)
	if !ok {
		t.Fatal("expected a Trash folder")
	}
	if got.Name != "[Gmail]/Trash" {
		t.Errorf("got %q, want the \\Trash mailbox", got.Name)
	}
}

func TestFindTrashFolderFallsBackToLocalizedLeafName(t *testing.T) {
	folders := []FolderInfo{
		{Name: "INBOX", Delim: "/"},
		{Name: "Archive/已删除邮件", Delim: "/"},
		{Name: "Junk", Delim: "/", Attrs: []string{string(imap.MailboxAttrJunk)}},
	}

	got, ok := findTrashFolder(folders)
	if !ok {
		t.Fatal("expected a Trash folder")
	}
	if got.Name != "Archive/已删除邮件" {
		t.Errorf("got %q, want the localized Trash leaf", got.Name)
	}
}

func TestFindTrashFolderIgnoresJunkAndNonSelectableMatches(t *testing.T) {
	folders := []FolderInfo{
		{Name: "Deleted", Delim: "/", Attrs: []string{string(imap.MailboxAttrNoSelect)}},
		{Name: "Spam Trash", Delim: "/", Attrs: []string{string(imap.MailboxAttrJunk)}},
		{Name: "Archive", Delim: "/"},
	}

	if got, ok := findTrashFolder(folders); ok {
		t.Errorf("unexpected Trash candidate %+v", got)
	}
}

func TestDeleteEmailClearsCacheAndUpdatesFolderTotal(t *testing.T) {
	// Keep the test away from the real application cache.
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{
		{UID: 1, Subject: "keep"},
		{UID: 2, Subject: "delete"},
	}); err != nil {
		t.Fatalf("SaveEmails: %v", err)
	}
	if err := db.SaveEmailBody("acct", "INBOX", &EmailDetail{
		UID: 2, Subject: "delete", BodyText: "body",
	}); err != nil {
		t.Fatalf("SaveEmailBody: %v", err)
	}
	if err := db.SaveFolderMeta("acct", "INBOX", FolderMeta{Total: 2, UIDNext: 3}); err != nil {
		t.Fatalf("SaveFolderMeta: %v", err)
	}

	if err := db.DeleteEmail("acct", "INBOX", 2); err != nil {
		t.Fatalf("DeleteEmail: %v", err)
	}

	emails, err := db.LoadEmails("acct", "INBOX")
	if err != nil {
		t.Fatalf("LoadEmails: %v", err)
	}
	if len(emails) != 1 || emails[0].UID != 1 {
		t.Errorf("remaining emails = %+v, want only uid 1", emails)
	}
	if _, err := db.LoadEmailBody("acct", "INBOX", 2); err == nil {
		t.Error("deleted message body survived")
	}
	meta, err := db.LoadFolderMeta("acct", "INBOX")
	if err != nil {
		t.Fatalf("LoadFolderMeta: %v", err)
	}
	if meta.Total != 1 {
		t.Errorf("folder total = %d, want 1", meta.Total)
	}
	if meta.UIDNext != 3 {
		t.Errorf("UIDNext = %d, want it unchanged at 3", meta.UIDNext)
	}
}

func TestDeleteEmailTreatsMissingServerUIDAsStaleCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, account := startTestIMAP(t, func(user *imapmemserver.User) {
		if err := user.Create("INBOX", nil); err != nil {
			t.Fatalf("Create(INBOX): %v", err)
		}
		if err := user.Create("Trash", nil); err != nil {
			t.Fatalf("Create(Trash): %v", err)
		}
	})

	db := NewDBManager()
	if err := db.SaveEmails(account.ID, "INBOX", []EmailSummary{{UID: 99, Subject: "already moved"}}); err != nil {
		t.Fatalf("SaveEmails: %v", err)
	}
	if err := db.SaveFolderMeta(account.ID, "INBOX", FolderMeta{Total: 1, UIDNext: 100}); err != nil {
		t.Fatalf("SaveFolderMeta: %v", err)
	}

	service := NewEmailService()
	if err := service.DeleteEmail(account, "INBOX", 99); err != nil {
		t.Fatalf("DeleteEmail: %v", err)
	}

	emails, err := db.LoadEmails(account.ID, "INBOX")
	if err != nil {
		t.Fatalf("LoadEmails: %v", err)
	}
	if len(emails) != 0 {
		t.Errorf("remaining emails = %+v, want the stale row removed", emails)
	}
	meta, err := db.LoadFolderMeta(account.ID, "INBOX")
	if err != nil {
		t.Fatalf("LoadFolderMeta: %v", err)
	}
	if meta.Total != 1 {
		t.Errorf("folder total = %d, want it unchanged because the server was already current", meta.Total)
	}
}

func TestRefreshEmailsPrunesMessagesDeletedElsewhere(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, account := startTestIMAP(t, func(user *imapmemserver.User) {
		if err := user.Create("INBOX", nil); err != nil {
			t.Fatalf("Create(INBOX): %v", err)
		}
		message := []byte("From: sender@example.com\r\nTo: user@example.com\r\nSubject: still here\r\n\r\nbody")
		if _, err := user.Append("INBOX", bytes.NewReader(message), &imap.AppendOptions{}); err != nil {
			t.Fatalf("Append(INBOX): %v", err)
		}
	})

	db := NewDBManager()
	if err := db.SaveEmails(account.ID, "INBOX", []EmailSummary{{UID: 99, Subject: "deleted elsewhere"}}); err != nil {
		t.Fatalf("SaveEmails: %v", err)
	}
	if err := db.SaveEmailBody(account.ID, "INBOX", &EmailDetail{
		UID: 99, Subject: "deleted elsewhere", BodyText: "stale body",
	}); err != nil {
		t.Fatalf("SaveEmailBody: %v", err)
	}

	service := NewEmailService()
	resp, err := service.RefreshEmails(account, "INBOX", 1, 20)
	if err != nil {
		t.Fatalf("RefreshEmails: %v", err)
	}
	if len(resp.Emails) != 1 || resp.Emails[0].Subject != "still here" {
		t.Errorf("refresh returned %+v, want only the server message", resp.Emails)
	}

	emails, err := db.LoadEmails(account.ID, "INBOX")
	if err != nil {
		t.Fatalf("LoadEmails: %v", err)
	}
	if len(emails) != 1 || emails[0].Subject != "still here" {
		t.Errorf("cached emails = %+v, want the stale row pruned", emails)
	}
	if _, err := db.LoadEmailBody(account.ID, "INBOX", 99); err == nil {
		t.Error("stale cached body survived the refresh")
	}
}

func TestDeleteEmailMovesMessageToTrashOverIMAP(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	user, account := startTestIMAP(t, func(user *imapmemserver.User) {
		if err := user.Create("INBOX", nil); err != nil {
			t.Fatalf("Create(INBOX): %v", err)
		}
		if err := user.Create("Trash", nil); err != nil {
			t.Fatalf("Create(Trash): %v", err)
		}
		message := []byte("From: sender@example.com\r\nTo: user@example.com\r\nSubject: delete me\r\n\r\nbody")
		if _, err := user.Append("INBOX", bytes.NewReader(message), &imap.AppendOptions{}); err != nil {
			t.Fatalf("Append(INBOX): %v", err)
		}
	})

	service := NewEmailService()
	if err := service.DeleteEmail(account, "INBOX", 1); err != nil {
		t.Fatalf("DeleteEmail: %v", err)
	}

	inbox, err := user.Status("INBOX", &imap.StatusOptions{NumMessages: true})
	if err != nil {
		t.Fatalf("Status(INBOX): %v", err)
	}
	if inbox.NumMessages == nil || *inbox.NumMessages != 0 {
		t.Errorf("INBOX messages = %v, want 0", inbox.NumMessages)
	}
	trash, err := user.Status("Trash", &imap.StatusOptions{NumMessages: true})
	if err != nil {
		t.Fatalf("Status(Trash): %v", err)
	}
	if trash.NumMessages == nil || *trash.NumMessages != 1 {
		t.Errorf("Trash messages = %v, want 1", trash.NumMessages)
	}

	// Deleting from Trash must expunge the message rather than moving it onto
	// itself. The destination UID is 1 because Trash started empty.
	if err := service.DeleteEmail(account, "Trash", 1); err != nil {
		t.Fatalf("DeleteEmail(Trash): %v", err)
	}
	trash, err = user.Status("Trash", &imap.StatusOptions{NumMessages: true})
	if err != nil {
		t.Fatalf("Status(Trash after permanent delete): %v", err)
	}
	if trash.NumMessages == nil || *trash.NumMessages != 0 {
		t.Errorf("Trash messages after permanent delete = %v, want 0", trash.NumMessages)
	}
}

func startTestIMAP(
	t *testing.T,
	setup func(user *imapmemserver.User),
) (*imapmemserver.User, EmailAccount) {
	t.Helper()

	memServer := imapmemserver.New()
	user := imapmemserver.NewUser("user", "pass")
	setup(user)
	memServer.AddUser(user)

	server := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memServer.NewSession(), nil, nil
		},
		InsecureAuth: true,
		Caps: imap.CapSet{
			imap.CapIMAP4rev1: {},
			imap.CapMove:      {},
			imap.CapUIDPlus:   {},
		},
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	go func() {
		_ = server.Serve(listener)
	}()

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("Atoi(port): %v", err)
	}
	account := EmailAccount{
		ID:       "test-account",
		Provider: "custom",
		IMAPHost: host,
		IMAPPort: port,
		IMAPUser: "user",
		IMAPPass: "pass",
	}
	return user, account
}
