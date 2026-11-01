package services

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	netmail "net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-sasl"
)

const (
	maxOutgoingMessageBytes    = 25 << 20
	maxOutgoingAttachmentBytes = 20 << 20
	maxOutgoingAttachments     = 20
)

// EmailAddress is the address shape shared by the compose UI and the sender.
// Address is the actual mailbox; Name is optional display text.
type EmailAddress struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

// SendAttachment describes a normal file attachment. Path is a local file path
// selected through the desktop app; the sender reads it only after validating it.
type SendAttachment struct {
	Path        string `json:"Path"`
	Filename    string `json:"Filename"`
	ContentType string `json:"ContentType"`
}

// SendInlineImage describes an image referenced from the HTML body with a CID.
// ContentID is the value used in <img src="cid:...">.
type SendInlineImage struct {
	Path        string `json:"Path"`
	Filename    string `json:"Filename"`
	ContentType string `json:"ContentType"`
	ContentID   string `json:"ContentID"`
}

// SendEmailRequest is the complete message payload handed to the sender.
type SendEmailRequest struct {
	AccountID string         `json:"AccountID"`
	To        []EmailAddress `json:"To"`
	Cc        []EmailAddress `json:"Cc"`
	Bcc       []EmailAddress `json:"Bcc"`
	Subject   string         `json:"Subject"`
	// Text is the plain-text alternative. If it is empty, it is derived from HTML.
	Text string `json:"Text"`
	// HTML is the rich-text body. If it is empty, Text is escaped into paragraphs.
	HTML         string            `json:"HTML"`
	Attachments  []SendAttachment  `json:"Attachments"`
	InlineImages []SendInlineImage `json:"InlineImages"`
	InReplyTo    string            `json:"InReplyTo"`
	References   []string          `json:"References"`
}

// SendEmailResult reports the accepted envelope and the best-effort Sent copy.
type SendEmailResult struct {
	MessageID  string   `json:"MessageID"`
	Accepted   []string `json:"Accepted"`
	SentFolder string   `json:"SentFolder"`
	SentSaved  bool     `json:"SentSaved"`
	Warning    string   `json:"Warning"`
}

// EmailSendService owns outbound SMTP and the optional IMAP Sent-folder copy.
// Keeping it separate from EmailService means send failures cannot disturb the
// IMAP cache or the IDLE watcher.
type EmailSendService struct {
	cfg   *ConfigService
	email *EmailService
}

func NewEmailSendService(cfg *ConfigService, email *EmailService) *EmailSendService {
	return &EmailSendService{cfg: cfg, email: email}
}

// SendEmail sends one message. The message is built and validated before any
// SMTP connection is opened, so a bad attachment cannot leave a half-sent mail.
func (s *EmailSendService) SendEmail(req SendEmailRequest) (*SendEmailResult, error) {
	account, err := s.account(req.AccountID)
	if err != nil {
		return nil, err
	}
	if len(req.Attachments)+len(req.InlineImages) > maxOutgoingAttachments {
		return nil, fmt.Errorf("too many attachments: %d (limit %d)", len(req.Attachments)+len(req.InlineImages), maxOutgoingAttachments)
	}

	req.To, err = normalizeAddresses(req.To)
	if err != nil {
		return nil, fmt.Errorf("invalid To recipient: %w", err)
	}
	req.Cc, err = normalizeAddresses(req.Cc)
	if err != nil {
		return nil, fmt.Errorf("invalid Cc recipient: %w", err)
	}
	req.Bcc, err = normalizeAddresses(req.Bcc)
	if err != nil {
		return nil, fmt.Errorf("invalid Bcc recipient: %w", err)
	}
	recipients := envelopeRecipients(req.To, req.Cc, req.Bcc)
	if len(recipients) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}

	attachments := make([]preparedAttachment, 0, len(req.Attachments))
	for _, attachment := range req.Attachments {
		prepared, err := prepareOutgoingFile(attachment.Path, attachment.Filename, attachment.ContentType, false)
		if err != nil {
			return nil, fmt.Errorf("attachment %q: %w", attachment.Filename, err)
		}
		attachments = append(attachments, prepared)
	}

	inlineImages := make([]preparedAttachment, 0, len(req.InlineImages))
	seenContentIDs := make(map[string]struct{}, len(req.InlineImages))
	for _, image := range req.InlineImages {
		prepared, err := prepareOutgoingFile(image.Path, image.Filename, image.ContentType, true)
		if err != nil {
			return nil, fmt.Errorf("inline image %q: %w", image.Filename, err)
		}
		contentID := sanitizeContentID(image.ContentID)
		if contentID == "" {
			contentID = prepared.contentID
		}
		if _, exists := seenContentIDs[contentID]; exists {
			return nil, fmt.Errorf("duplicate inline image Content-ID %q", contentID)
		}
		seenContentIDs[contentID] = struct{}{}
		prepared.contentID = contentID
		inlineImages = append(inlineImages, prepared)
	}

	if strings.TrimSpace(req.Text) == "" && strings.TrimSpace(req.HTML) != "" {
		req.Text = htmlToPlainText(req.HTML)
	}
	if strings.TrimSpace(req.HTML) == "" && strings.TrimSpace(req.Text) != "" {
		req.HTML = textToHTML(req.Text)
	}

	built, err := buildEmailMessage(account, req, attachments, inlineImages)
	if err != nil {
		return nil, err
	}
	if err := s.smtpSend(account, recipients, built.raw); err != nil {
		return nil, err
	}

	result := &SendEmailResult{
		MessageID: built.messageID,
		Accepted:  recipients,
	}
	if s.email != nil {
		folder, saveErr := s.appendToSent(account, built.raw)
		if folder != "" {
			result.SentFolder = folder
		}
		if saveErr != nil {
			// SMTP already accepted the message. A missing Sent copy is useful to
			// surface, but it must not turn a successful send into a failed send.
			result.Warning = fmt.Sprintf("sent successfully, but saving a copy to Sent failed: %v", saveErr)
			slog.Warn("smtp: sent message but failed to append to Sent",
				"account", account, "folder", folder, "error", saveErr)
		} else if folder != "" {
			result.SentSaved = true
		}
	}
	return result, nil
}

func (s *EmailSendService) account(accountID string) (EmailAccount, error) {
	if s.cfg == nil {
		return EmailAccount{}, fmt.Errorf("config service is unavailable")
	}
	accounts, err := s.cfg.GetAccounts()
	if err != nil {
		return EmailAccount{}, fmt.Errorf("load accounts: %w", err)
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" && len(accounts) > 0 {
		return accounts[0], nil
	}
	for _, account := range accounts {
		if account.ID == accountID {
			return account, nil
		}
	}
	if accountID == "" {
		return EmailAccount{}, fmt.Errorf("no email accounts are configured")
	}
	return EmailAccount{}, fmt.Errorf("account %q not found", accountID)
}

type smtpSettings struct {
	host        string
	port        int
	user        string
	password    string
	implicitTLS bool
	startTLS    bool
}

func (s *EmailSendService) settings(account EmailAccount) (smtpSettings, error) {
	host := strings.TrimSpace(account.SMTPHost)
	if host == "" {
		return smtpSettings{}, fmt.Errorf("SMTP host is not configured for %s", account.Email)
	}
	port := account.SMTPPort
	if port == 0 {
		return smtpSettings{}, fmt.Errorf("SMTP port is not configured for %s", account.Email)
	}
	user := strings.TrimSpace(account.SMTPUser)
	if user == "" {
		user = strings.TrimSpace(account.Email)
	}
	password := account.SMTPPass
	if password == "" {
		password = account.IMAPPass
	}
	if account.AuthType != "oauth2" && password == "" {
		return smtpSettings{}, fmt.Errorf("SMTP password is not configured for %s", account.Email)
	}
	return smtpSettings{
		host:        host,
		port:        port,
		user:        user,
		password:    password,
		implicitTLS: port == 465,
		startTLS:    port != 465 && account.UseTLS,
	}, nil
}

func (s *EmailSendService) smtpSend(account EmailAccount, recipients []string, raw []byte) error {
	settings, err := s.settings(account)
	if err != nil {
		return err
	}
	client, conn, err := s.connect(account, settings)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer client.Close()

	if account.AuthType == "oauth2" {
		token, err := oauthAccessToken(account)
		if err != nil {
			return fmt.Errorf("SMTP OAuth: %w", err)
		}
		if err := client.Auth(smtpAuthFromSASL(xoauth2Client{username: settings.user, token: token})); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	} else if err := client.Auth(smtpAuthFromSASL(sasl.NewPlainClient("", settings.user, settings.password))); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	from := strings.TrimSpace(account.Email)
	if from == "" {
		from = settings.user
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("SMTP RCPT TO %s failed: %w", recipient, err)
		}
	}
	data, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if _, err := data.Write(raw); err != nil {
		_ = data.Close()
		return fmt.Errorf("SMTP message write failed: %w", err)
	}
	if err := data.Close(); err != nil {
		return fmt.Errorf("SMTP message finalization failed: %w", err)
	}
	if err := client.Quit(); err != nil {
		// The DATA response was already accepted. A server closing the connection
		// immediately after it is not a reason to tell the user the send failed.
		slog.Debug("smtp: QUIT after accepted message failed", "account", account, "error", err)
	}
	return nil
}

// Go 1.25 changed net/smtp.Auth.Next to take the "more" flag. The IMAP path uses
// emersion/go-sasl's older interface, so adapt it at the SMTP boundary instead of
// maintaining a second XOAUTH2 implementation.
type smtpSASLAuth struct {
	client sasl.Client
}

func smtpAuthFromSASL(client sasl.Client) smtp.Auth {
	return smtpSASLAuth{client: client}
}

func (a smtpSASLAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return a.client.Start()
}

func (a smtpSASLAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	// net/smtp calls Next once more after a successful 235 response, with
	// more=false. SASL clients such as PLAIN and XOAUTH2 do not expect a final
	// callback and report it as an unexpected challenge, so finish here first.
	if !more {
		return nil, nil
	}
	return a.client.Next(fromServer)
}

func (s *EmailSendService) connect(account EmailAccount, settings smtpSettings) (*smtp.Client, net.Conn, error) {
	timeout := 30 * time.Second
	dialer, err := dialerForAccount(account, timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("SMTP proxy: %w", err)
	}
	addr := net.JoinHostPort(settings.host, fmt.Sprintf("%d", settings.port))
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("SMTP connect to %s failed: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	tlsConfig := &tls.Config{
		ServerName: settings.host,
		MinVersion: tls.VersionTLS12,
	}
	if settings.implicitTLS {
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			_ = conn.Close()
			return nil, nil, fmt.Errorf("SMTP TLS handshake failed: %w", err)
		}
		conn = tlsConn
	}

	client, err := smtp.NewClient(conn, settings.host)
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("SMTP greeting failed: %w", err)
	}
	if settings.startTLS {
		if err := client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			_ = conn.Close()
			return nil, nil, fmt.Errorf("SMTP STARTTLS failed: %w", err)
		}
	}
	return client, conn, nil
}

func normalizeAddresses(input []EmailAddress) ([]EmailAddress, error) {
	result := make([]EmailAddress, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		address := strings.TrimSpace(item.Address)
		if address == "" {
			continue
		}
		parsed, err := netmail.ParseAddress(address)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", address, err)
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = parsed.Name
		}
		key := strings.ToLower(parsed.Address)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, EmailAddress{Name: name, Address: parsed.Address})
	}
	return result, nil
}

func envelopeRecipients(groups ...[]EmailAddress) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, group := range groups {
		for _, address := range group {
			key := strings.ToLower(address.Address)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, address.Address)
		}
	}
	return result
}

func sanitizeContentID(id string) string {
	id = strings.Trim(strings.TrimSpace(id), "<>")
	id = strings.Map(func(r rune) rune {
		if r <= 0x20 || r >= 0x7f || strings.ContainsRune(`()<>[]:;@\,`, r) {
			return '_'
		}
		return r
	}, id)
	return strings.Trim(id, "_")
}

func (s *EmailSendService) appendToSent(account EmailAccount, raw []byte) (string, error) {
	folders, err := s.email.ListFolders(account)
	if err != nil {
		return "", err
	}
	folder := sentFolderName(folders)
	if folder == "" {
		return "", nil
	}
	client, err := connectIMAP(account, 60*time.Second)
	if err != nil {
		return folder, err
	}
	defer client.Logout()

	cmd := client.Append(folder, int64(len(raw)), &imap.AppendOptions{
		Flags: []imap.Flag{imap.FlagSeen},
		Time:  time.Now(),
	})
	if _, err := cmd.Write(raw); err != nil {
		_ = cmd.Close()
		return folder, fmt.Errorf("append to %s failed: %w", folder, err)
	}
	if err := cmd.Close(); err != nil {
		return folder, fmt.Errorf("append to %s failed: %w", folder, err)
	}
	if _, err := cmd.Wait(); err != nil {
		return folder, fmt.Errorf("append to %s failed: %w", folder, err)
	}
	return folder, nil
}

func sentFolderName(folders []FolderInfo) string {
	for _, folder := range folders {
		for _, attr := range folder.Attrs {
			normalized := strings.TrimLeft(strings.ToLower(attr), `\`)
			if normalized == "sent" {
				return folder.Name
			}
		}
	}
	for _, folder := range folders {
		name := strings.ToLower(strings.TrimSpace(folder.Name))
		for _, candidate := range []string{"sent", "sent mail", "sent messages", "sent items", "已发送", "已发送邮件", "已发邮件", "[gmail]/sent mail"} {
			if name == candidate || strings.HasSuffix(name, "/"+candidate) {
				return folder.Name
			}
		}
	}
	return ""
}
