package services

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
)

// providerNeedsIDCommand checks whether the given email provider requires an
// RFC 2971 ID command before LOGIN. Coremail-based Chinese mail servers
// (163.com, 126.com, qq.com, etc.) reject connections with "Unsafe Login" if
// this command is not sent before authentication.
func providerNeedsIDCommand(provider string) bool {
	switch provider {
	case "163", "qq":
		return true
	default:
		return false
	}
}

// EmailService provides methods for connecting to IMAP/SMTP email servers
// and managing email operations.
type EmailService struct {
	db *DBManager
}

func NewEmailService() *EmailService {
	return &EmailService{
		db: NewDBManager(),
	}
}

// EmailSummary is a lightweight representation of an email for list views.
type EmailSummary struct {
	UID       uint32   `json:"uid"`
	Subject   string   `json:"subject"`
	From      string   `json:"from"`
	FromName  string   `json:"from_name"`
	FromAddr  string   `json:"from_addr"`
	Date      string   `json:"date"`
	Flags     []string `json:"flags"`
	IsRead    bool     `json:"is_read"`
}

// EmailDetail is the full representation of an email including body content.
type EmailDetail struct {
	UID       uint32   `json:"uid"`
	Subject   string   `json:"subject"`
	From      string   `json:"from"`
	FromName  string   `json:"from_name"`
	FromAddr  string   `json:"from_addr"`
	To        []string `json:"to"`
	Cc        []string `json:"cc"`
	Date      string   `json:"date"`
	BodyText  string   `json:"body_text"`
	BodyHTML  string   `json:"body_html"`
	Flags     []string `json:"flags"`
}

// FolderInfo represents a mail folder with its metadata.
type FolderInfo struct {
	Name   string   `json:"name"`
	Delim  string   `json:"delim"`
	Attrs  []string `json:"attrs"`
}

// TestConnection attempts to connect and authenticate to the IMAP server.
// Returns success status and a human-readable message. Times out after 15 seconds.
func (s *EmailService) TestConnection(account EmailAccount) (bool, string) {
	client, err := dialIMAP(account, 15*time.Second)
	if err != nil {
		return false, fmt.Sprintf("Connection failed: %v", err)
	}
	defer client.Logout()

	return true, fmt.Sprintf("Successfully connected to %s as %s", account.IMAPHost, account.IMAPUser)
}

// ListFolders retrieves all available IMAP mailboxes/folders.
// Returns cached data if available and no force refresh requested.
func (s *EmailService) ListFolders(account EmailAccount) ([]FolderInfo, error) {
	// Try cache first
	cached, err := s.db.LoadFolders(account.ID)
	if err == nil && len(cached) > 0 {
		return cached, nil
	}

	return s.RefreshFolders(account)
}

// RefreshFolders fetches folders from the IMAP server and updates the cache.
func (s *EmailService) RefreshFolders(account EmailAccount) ([]FolderInfo, error) {
	client, err := dialIMAP(account, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer client.Logout()

	listCmd := client.List("", "*", nil)
	defer listCmd.Close()

	listData, err := listCmd.Collect()
	if err != nil {
		return nil, fmt.Errorf("list failed: %w", err)
	}

	folders := make([]FolderInfo, 0, len(listData))
	for _, ld := range listData {
		attrs := make([]string, len(ld.Attrs))
		for i, a := range ld.Attrs {
			attrs[i] = string(a)
		}
		folders = append(folders, FolderInfo{
			Name:  ld.Mailbox,
			Delim: string(ld.Delim),
			Attrs: attrs,
		})
	}

	// Update cache
	if err := s.db.SaveFolders(account.ID, folders); err != nil {
		// Log but don't fail — cache is best-effort
		fmt.Printf("failed to cache folders: %v\n", err)
	}

	return folders, nil
}

// ListEmails retrieves a page of email summaries from the specified folder.
// Returns cached data if available.
func (s *EmailService) ListEmails(account EmailAccount, folder string, page, pageSize int) ([]EmailSummary, error) {
	// Try cache first — always return page 1 from cache; pagination over
	// cached data is disabled for simplicity (we cache the full folder).
	if page <= 1 {
		cached, err := s.db.LoadEmails(account.ID, folder)
		if err == nil && len(cached) > 0 {
			return s.paginate(cached, page, pageSize), nil
		}
	}

	return s.RefreshEmails(account, folder, page, pageSize)
}

// RefreshEmails fetches email summaries from the IMAP server and updates the
// cache.
func (s *EmailService) RefreshEmails(account EmailAccount, folder string, page, pageSize int) ([]EmailSummary, error) {
	client, err := dialIMAP(account, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer client.Logout()

	// Select the mailbox (read-only)
	selectCmd := client.Select(folder, &imap.SelectOptions{ReadOnly: true})
	mbox, err := selectCmd.Wait()
	if err != nil {
		return nil, wrapIMAPError("select folder failed", err)
	}

	if mbox.NumMessages == 0 {
		return []EmailSummary{}, nil
	}

	// Calculate the range for pagination
	total := int(mbox.NumMessages)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := total - (page-1)*pageSize
	end := total - page*pageSize + 1
	if end < 1 {
		end = 1
	}
	if start < 1 {
		return []EmailSummary{}, nil
	}

	var seqSet imap.SeqSet
	if start != end {
		seqSet.AddRange(uint32(end), uint32(start))
	} else {
		seqSet.AddNum(uint32(end))
	}

	// Fetch envelope and flags
	fetchCmd := client.Fetch(seqSet, &imap.FetchOptions{
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		UID:          true,
	})
	defer fetchCmd.Close()

	buffers, err := fetchCmd.Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	// Results come in ascending order; reverse for newest-first
	summaries := make([]EmailSummary, 0, len(buffers))
	for _, buf := range buffers {
		summary := EmailSummary{
			UID:     uint32(buf.UID),
			Subject: buf.Envelope.Subject,
			Date:    buf.Envelope.Date.Format(time.RFC3339),
			Flags:   make([]string, len(buf.Flags)),
		}
		for i, f := range buf.Flags {
			summary.Flags[i] = string(f)
		}
		// Check if the message is read
		for _, f := range buf.Flags {
			if f == imap.FlagSeen {
				summary.IsRead = true
				break
			}
		}
		// Build sender name + address from envelope
		if len(buf.Envelope.From) > 0 {
			addr := buf.Envelope.From[0]
			if addr.Host != "" {
				summary.FromAddr = addr.Mailbox + "@" + addr.Host
			} else {
				summary.FromAddr = addr.Mailbox
			}
			summary.FromName = addr.Name
			if summary.FromName != "" {
				summary.From = summary.FromName + " <" + summary.FromAddr + ">"
			} else {
				summary.From = summary.FromAddr
			}
		}
		summaries = append(summaries, summary)
	}

	// Reverse to get newest first
	for i, j := 0, len(summaries)-1; i < j; i, j = i+1, j-1 {
		summaries[i], summaries[j] = summaries[j], summaries[i]
	}

	// Update cache with all fetched summaries (best-effort)
	if err := s.db.SaveEmails(account.ID, folder, summaries); err != nil {
		fmt.Printf("failed to cache emails: %v\n", err)
	}

	return s.paginate(summaries, page, pageSize), nil
}

// FetchEmail retrieves the full content of a single email by its UID.
// Returns cached data if available.
func (s *EmailService) FetchEmail(account EmailAccount, folder string, uid uint32) (*EmailDetail, error) {
	// Try cache first
	cached, err := s.db.LoadEmailBody(account.ID, folder, uid)
	if err == nil && cached != nil {
		return cached, nil
	}

	return s.RefreshEmailBody(account, folder, uid)
}

// RefreshEmailBody fetches the full email content from the IMAP server and
// updates the cache. It first fetches the body structure (MIME tree) to
// discover text/plain and text/html part paths, then fetches those sections.
func (s *EmailService) RefreshEmailBody(account EmailAccount, folder string, uid uint32) (*EmailDetail, error) {
	client, err := dialIMAP(account, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer client.Logout()

	// Select the mailbox (read-only)
	selectCmd := client.Select(folder, &imap.SelectOptions{ReadOnly: true})
	_, err = selectCmd.Wait()
	if err != nil {
		return nil, wrapIMAPError("select folder failed", err)
	}

	// Build the UID set
	var uidSet imap.UIDSet
	uidSet.AddNum(imap.UID(uid))

	// --- Pass 1: Fetch BodyStructure + metadata ---
	fetchCmd1 := client.Fetch(uidSet, &imap.FetchOptions{
		BodyStructure: &imap.FetchItemBodyStructure{},
		Envelope:      true,
		Flags:         true,
		UID:           true,
	})
	defer fetchCmd1.Close()

	bufs1, err := fetchCmd1.Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch structure failed: %w", err)
	}
	if len(bufs1) == 0 {
		return nil, fmt.Errorf("email not found: uid=%d", uid)
	}

	buf := bufs1[0]

	detail := &EmailDetail{
		UID:     uint32(buf.UID),
		Subject: buf.Envelope.Subject,
		Date:    buf.Envelope.Date.Format(time.RFC3339),
		Flags:   make([]string, len(buf.Flags)),
	}

	// Build sender name + address from envelope
	for _, addr := range buf.Envelope.From {
		if addr.Host != "" {
			detail.FromAddr = addr.Mailbox + "@" + addr.Host
		} else {
			detail.FromAddr = addr.Mailbox
		}
		detail.FromName = addr.Name
		if detail.FromName != "" {
			detail.From = detail.FromName + " <" + detail.FromAddr + ">"
		} else {
			detail.From = detail.FromAddr
		}
	}
	for _, addr := range buf.Envelope.To {
		if addr.Host != "" {
			detail.To = append(detail.To, addr.Mailbox+"@"+addr.Host)
		} else {
			detail.To = append(detail.To, addr.Mailbox)
		}
	}
	for _, addr := range buf.Envelope.Cc {
		if addr.Host != "" {
			detail.Cc = append(detail.Cc, addr.Mailbox+"@"+addr.Host)
		} else {
			detail.Cc = append(detail.Cc, addr.Mailbox)
		}
	}

	// --- Walk body structure to find text/plain and text/html parts ---
	var parts []bodyPartInfo
	if buf.BodyStructure != nil {
		buf.BodyStructure.Walk(func(path []int, part imap.BodyStructure) bool {
			mt := part.MediaType()
			isTarget := mt == "text/plain" || mt == "text/html"
			if sp, ok := part.(*imap.BodyStructureSinglePart); ok && isTarget {
				parts = append(parts, bodyPartInfo{
					path:     path,
					isHTML:   mt == "text/html",
					encoding: sp.Encoding,
					charset:  sp.Params["charset"],
				})
			}
			if _, ok := part.(*imap.BodyStructureMultiPart); ok {
				return true
			}
			return false
		})
	}

	// --- Pass 2: Fetch the discovered body sections ---
	var sections []*imap.FetchItemBodySection
	if len(parts) == 0 {
		sections = append(sections, &imap.FetchItemBodySection{Peek: true})
	}
	for _, p := range parts {
		sections = append(sections, &imap.FetchItemBodySection{Part: p.path, Peek: true})
	}

	if len(sections) > 0 {
		fetchCmd2 := client.Fetch(uidSet, &imap.FetchOptions{
			BodySection: sections,
		})
		defer fetchCmd2.Close()

		bufs2, err := fetchCmd2.Collect()
		if err != nil {
			return nil, fmt.Errorf("fetch body failed: %w", err)
		}
		if len(bufs2) > 0 {
			buf2 := bufs2[0]
			for _, sec := range sections {
				raw := buf2.FindBodySection(sec)
				if raw == nil {
					continue
				}
				// Find matching part info for decoding
				var info *bodyPartInfo
				for i, p := range parts {
					if sectionPathEqual(sec.Part, p.path) {
						info = &parts[i]
						break
					}
				}
				if info != nil {
					decoded := decodeBodyPart(raw, info.encoding, info.charset)
					if info.isHTML {
						detail.BodyHTML = decoded
					} else {
						detail.BodyText = decoded
					}
				} else {
					// Fallback section (no part info) — try as plain text
					detail.BodyText = string(raw)
				}
			}
		}
	}

	// Update cache (best-effort)
	if err := s.db.SaveEmailBody(account.ID, folder, detail); err != nil {
		fmt.Printf("failed to cache email body: %v\n", err)
	}

	return detail, nil
}

// bodyPartInfo captures metadata for a specific MIME part discovered during
// body structure walk.
type bodyPartInfo struct {
	path     []int
	isHTML   bool
	encoding string
	charset  string
}

// decodeBodyPart decodes raw IMAP body section bytes based on
// Content-Transfer-Encoding and charset.
func decodeBodyPart(raw []byte, enc, cs string) string {
	if len(raw) == 0 {
		return ""
	}
	// Step 1: decode Content-Transfer-Encoding
	var decoded []byte
	var err error
	switch strings.ToLower(enc) {
	case "base64":
		decoded, err = base64.StdEncoding.DecodeString(string(raw))
		if err != nil {
			// Try with padded input
			s := strings.TrimRight(string(raw), "\n\r\t ")
			decoded, err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				decoded = raw
			}
		}
	case "quoted-printable":
		decoded, err = io.ReadAll(quotedprintable.NewReader(strings.NewReader(string(raw))))
		if err != nil {
			decoded = raw
		}
	default:
		// 7bit, 8bit, binary — no decoding needed
		decoded = raw
	}

	// Step 2: decode charset to UTF-8
	if cs != "" && !strings.EqualFold(cs, "utf-8") && !strings.EqualFold(cs, "us-ascii") {
		reader, err := charset.Reader(cs, strings.NewReader(string(decoded)))
		if err == nil {
			utf8bytes, err := io.ReadAll(reader)
			if err == nil {
				return string(utf8bytes)
			}
		}
	}

	return string(decoded)
}

// sectionPathEqual compares two IMAP part paths for equality.
func sectionPathEqual(a, b []int) bool {
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

// paginate returns a slice of email summaries for the given page.
func (s *EmailService) paginate(summaries []EmailSummary, page, pageSize int) []EmailSummary {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(summaries) {
		return []EmailSummary{}
	}
	end := start + pageSize
	if end > len(summaries) {
		end = len(summaries)
	}
	return summaries[start:end]
}

// dialIMAP connects to the IMAP server with the given timeout for the entire
// connection + login operation. The timeout covers TCP dial, TLS handshake,
// and IMAP login.
func dialIMAP(account EmailAccount, timeout time.Duration) (*imapclient.Client, error) {
	addr := fmt.Sprintf("%s:%d", account.IMAPHost, account.IMAPPort)

	dialer := &net.Dialer{
		Timeout: timeout,
	}
	opts := &imapclient.Options{
		Dialer: dialer,
		TLSConfig: &tls.Config{
			ServerName: account.IMAPHost,
		},
	}

	var client *imapclient.Client
	var dialErr error

	if account.UseTLS || account.IMAPPort == 993 {
		client, dialErr = imapclient.DialTLS(addr, opts)
	} else {
		client, dialErr = imapclient.DialStartTLS(addr, opts)
		if dialErr != nil {
			// Fallback to insecure plain connection
			client, dialErr = imapclient.DialInsecure(addr, &imapclient.Options{Dialer: dialer})
		}
	}
	if dialErr != nil {
		return nil, fmt.Errorf("dial failed: %w", dialErr)
	}

	// Some Coremail-based providers (163.com, qq.com) require an RFC 2971 ID
	// command before LOGIN. Without it they reject with "Unsafe Login".
	if providerNeedsIDCommand(account.Provider) {
		// Wait for the server greeting first (consumes the greeting from the
		// connection so the client is ready for the next command).
		if err := client.WaitGreeting(); err != nil {
			client.Close()
			return nil, fmt.Errorf("greeting failed: %w", err)
		}
		// Send the ID command with client identification.
		idCmd := client.ID(&imap.IDData{
			Name:    "wvmail",
			Version: "1.0",
			Vendor:  "wvmail",
		})
		if _, err := idCmd.Wait(); err != nil {
			client.Close()
			return nil, fmt.Errorf("id command failed: %w", err)
		}
	}

	// Login with timeout — use a goroutine + select so login itself can't hang
	// beyond the configured timeout.
	type loginResult struct {
		err error
	}
	done := make(chan loginResult, 1)
	go func() {
		cmd := client.Login(account.IMAPUser, account.IMAPPass)
		err := cmd.Wait()
		done <- loginResult{err}
	}()

	select {
	case res := <-done:
		if res.err != nil {
			client.Close()
			return nil, wrapIMAPError("login failed", res.err)
		}
		// Send CAPABILITY after login — some servers (e.g., 163) require this
		// handshake before they'll allow SELECT/EXAMINE.
		if capCmd := client.Capability(); capCmd != nil {
			_, capErr := capCmd.Wait()
			if capErr != nil {
				client.Close()
				return nil, fmt.Errorf("capability failed: %w", capErr)
			}
		}
		return client, nil
	case <-time.After(timeout):
		client.Close()
		return nil, fmt.Errorf("login timed out after %v", timeout)
	}
}

// wrapIMAPError wraps common IMAP errors with user-friendly context.
func wrapIMAPError(context string, err error) error {
	msg := err.Error()
	if strings.Contains(msg, "Unsafe Login") {
		return fmt.Errorf("%s: %w\n\nThis server rejected the connection as unsafe. This may happen with 163/126 mail even when using the correct authorization code. Try: (1) ensure IMAP/SMTP service is enabled in webmail settings, (2) generate a fresh authorization code, (3) contacting customer service (kefu@188.com).", context, err)
	}
	if strings.Contains(msg, "authentication failed") || strings.Contains(msg, "login failed") || strings.Contains(msg, "LOGIN failed") || strings.Contains(msg, "Authentication failed") {
		return fmt.Errorf("%s: %w\n\nHint: Check your username/password. Some providers (163, QQ) require an authorization code (授权码) instead of your login password.", context, err)
	}
	return fmt.Errorf("%s: %w", context, err)
}
