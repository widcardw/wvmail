package services

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// EmailService provides methods for connecting to IMAP/SMTP email servers
// and managing email operations.
type EmailService struct{}

// EmailSummary is a lightweight representation of an email for list views.
type EmailSummary struct {
	UID       uint32   `json:"uid"`
	Subject   string   `json:"subject"`
	From      string   `json:"from"`
	Date      string   `json:"date"`
	Flags     []string `json:"flags"`
	IsRead    bool     `json:"is_read"`
}

// EmailDetail is the full representation of an email including body content.
type EmailDetail struct {
	UID       uint32   `json:"uid"`
	Subject   string   `json:"subject"`
	From      string   `json:"from"`
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
func (s *EmailService) ListFolders(account EmailAccount) ([]FolderInfo, error) {
	client, err := dialIMAP(account, 30*time.Second)
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
		fi := FolderInfo{
			Name:  ld.Mailbox,
			Delim: string(ld.Delim),
			Attrs: attrs,
		}
		folders = append(folders, fi)
	}

	return folders, nil
}

// ListEmails retrieves a page of email summaries from the specified folder.
func (s *EmailService) ListEmails(account EmailAccount, folder string, page, pageSize int) ([]EmailSummary, error) {
	client, err := dialIMAP(account, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer client.Logout()

	// Select the mailbox (read-only)
	selectCmd := client.Select(folder, &imap.SelectOptions{ReadOnly: true})
	mbox, err := selectCmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("select failed: %w", err)
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
		// Build From string from the envelope
		if len(buf.Envelope.From) > 0 {
			addr := buf.Envelope.From[0]
			if addr.Host != "" {
				summary.From = addr.Mailbox + "@" + addr.Host
			} else {
				summary.From = addr.Mailbox
			}
		}
		summaries = append(summaries, summary)
	}

	// Reverse to get newest first
	for i, j := 0, len(summaries)-1; i < j; i, j = i+1, j-1 {
		summaries[i], summaries[j] = summaries[j], summaries[i]
	}

	return summaries, nil
}

// FetchEmail retrieves the full content of a single email by its UID.
func (s *EmailService) FetchEmail(account EmailAccount, folder string, uid uint32) (*EmailDetail, error) {
	client, err := dialIMAP(account, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer client.Logout()

	// Select the mailbox (read-only)
	selectCmd := client.Select(folder, &imap.SelectOptions{ReadOnly: true})
	_, err = selectCmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("select failed: %w", err)
	}

	// Build the UID set
	var uidSet imap.UIDSet
	uidSet.AddNum(imap.UID(uid))

	// Fetch envelope and body section (text and HTML)
	fetchCmd := client.Fetch(uidSet, &imap.FetchOptions{
		Envelope: true,
		Flags:    true,
		UID:      true,
		BodySection: []*imap.FetchItemBodySection{
			{Peek: true},                              // full body
			{Specifier: imap.PartSpecifierText, Peek: true}, // text part
		},
	})
	defer fetchCmd.Close()

	buffers, err := fetchCmd.Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	if len(buffers) == 0 {
		return nil, fmt.Errorf("email not found: uid=%d", uid)
	}

	buf := buffers[0]
	detail := &EmailDetail{
		UID:     uint32(buf.UID),
		Subject: buf.Envelope.Subject,
		Date:    buf.Envelope.Date.Format(time.RFC3339),
		Flags:   make([]string, len(buf.Flags)),
	}

	// Decode body sections
	bodyBytes := buf.FindBodySection(&imap.FetchItemBodySection{Peek: true})
	if bodyBytes != nil {
		detail.BodyText = string(bodyBytes)
	}

	// Build addresses
	for _, addr := range buf.Envelope.From {
		if addr.Host != "" {
			detail.From = addr.Mailbox + "@" + addr.Host
		} else {
			detail.From = addr.Mailbox
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

	return detail, nil
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
			return nil, fmt.Errorf("login failed: %w", res.err)
		}
		return client, nil
	case <-time.After(timeout):
		client.Close()
		return nil, fmt.Errorf("login timed out after %v", timeout)
	}
}
