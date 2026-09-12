package services

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// decodeMimeHeader decodes a MIME encoded-word (=?charset?B?...?= / =?charset?Q?...?=)
// with support for Chinese character sets (GBK, GB2312, GB18030).
func decodeMimeHeader(s string) string {
	if !strings.Contains(s, "=?") {
		return s
	}
	dec := &mime.WordDecoder{
		CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
			switch strings.ToLower(charset) {
			case "gbk", "gb2312", "gb18030":
				return simplifiedchinese.GBK.NewDecoder().Reader(input), nil
			default:
				return nil, fmt.Errorf("unhandled charset: %s", charset)
			}
		},
	}
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s // fallback to original
	}
	return decoded
}

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
	UID        uint32   `json:"uid"`
	Subject    string   `json:"subject"`
	From       string   `json:"from"`
	FromName   string   `json:"from_name"`
	FromAddr   string   `json:"from_addr"`
	To         []string `json:"to"`
	Date       string   `json:"date"`
	Flags      []string `json:"flags"`
	IsRead     bool     `json:"is_read"`
	AccountID  string   `json:"account_id,omitempty"`
	FolderName string   `json:"folder_name,omitempty"`
}

// EmailSearchFilter is one structured search tag. Supported fields are
// "from", "to", and "content".
type EmailSearchFilter struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// EmailSearchQuery searches the local email cache. An empty AccountID searches
// across all cached accounts; a non-empty AccountID limits results to it.
type EmailSearchQuery struct {
	AccountID string              `json:"account_id"`
	Filters   []EmailSearchFilter `json:"filters"`
}

// AttachmentInfo describes a single email attachment discovered in the MIME
// body structure.
type AttachmentInfo struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     uint32 `json:"size"`
	PartPath []int  `json:"part_path"`
	IsInline bool   `json:"is_inline"`
}

// EmailDetail is the full representation of an email including body content.
type EmailDetail struct {
	UID          uint32           `json:"uid"`
	Subject      string           `json:"subject"`
	From         string           `json:"from"`
	FromName     string           `json:"from_name"`
	FromAddr     string           `json:"from_addr"`
	To           []string         `json:"to"`
	Cc           []string         `json:"cc"`
	Date         string           `json:"date"`
	BodyText     string           `json:"body_text"`
	BodyHTML     string           `json:"body_html"`
	Flags        []string         `json:"flags"`
	Attachments  []AttachmentInfo `json:"attachments"`
	IsSuspicious bool             `json:"is_suspicious"`
}

// FolderInfo represents a mail folder with its metadata.
type FolderInfo struct {
	Name  string   `json:"name"`
	Delim string   `json:"delim"`
	Attrs []string `json:"attrs"`
}

// EmailPageResponse is returned by ListEmails / RefreshEmails and contains
// the page of email summaries along with the total message count in the folder.
type EmailPageResponse struct {
	Emails []EmailSummary `json:"emails"`
	Total  int            `json:"total"` // total number of messages in the folder
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
func (s *EmailService) ListEmails(account EmailAccount, folder string, page, pageSize int) (*EmailPageResponse, error) {
	// Try cache first — always return page 1 from cache; pagination over
	// cached data is disabled for simplicity.
	if page <= 1 {
		cached, err := s.db.LoadEmails(account.ID, folder)
		if err == nil && len(cached) > 0 {
			total, metaErr := s.db.LoadFolderMeta(account.ID, folder)
			// If no meta is stored yet (e.g. old cache before this feature),
			// force a refresh to get the real total from the server.
			if metaErr != nil || total == 0 {
				return s.RefreshEmails(account, folder, page, pageSize)
			}
			return &EmailPageResponse{
				Emails: s.paginate(cached, page, pageSize),
				Total:  total,
			}, nil
		}
	}

	return s.RefreshEmails(account, folder, page, pageSize)
}

// SearchEmails searches cached email summaries and cached bodies. Results are
// ordered newest first and paginated before being returned to the frontend.
func (s *EmailService) SearchEmails(query EmailSearchQuery, page, pageSize int) (*EmailPageResponse, error) {
	emails, err := s.db.SearchEmails(query)
	if err != nil {
		return nil, fmt.Errorf("search emails failed: %w", err)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return &EmailPageResponse{
		Emails: s.paginate(emails, page, pageSize),
		Total:  len(emails),
	}, nil
}

// RefreshEmails fetches email summaries from the IMAP server and updates the
// cache.
func (s *EmailService) RefreshEmails(account EmailAccount, folder string, page, pageSize int) (*EmailPageResponse, error) {
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

	// A successful SELECT reports NumMessages for the mailbox. QQ/Foxmail IMAP can
	// transiently report 0 for an otherwise non-empty Sent folder right after a
	// change; when we have a prior non-zero total for this exact folder, retry the
	// select once before trusting an empty result.
	priorTotal, _ := s.db.LoadFolderMeta(account.ID, folder)
	for retry := 0; mbox.NumMessages == 0 && priorTotal > 0 && retry < 1; retry++ {
		time.Sleep(300 * time.Millisecond)
		if rs, e2 := client.Select(folder, &imap.SelectOptions{ReadOnly: true}).Wait(); e2 == nil {
			mbox = rs
		}
	}

	if mbox.NumMessages == 0 {
		if priorTotal > 0 {
			return nil, fmt.Errorf(
				"refresh reported 0 messages in %q while %d were cached — keeping previous list",
				folder, priorTotal,
			)
		}
		return &EmailPageResponse{Emails: []EmailSummary{}, Total: 0}, nil
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
		return &EmailPageResponse{Emails: []EmailSummary{}, Total: total}, nil
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

	buffers, err := fetchCmd.Collect()
	if err != nil {
		// Some servers (e.g. Tencent WeCom) return malformed ENVELOPE data.
		// Retry without Envelope and use default values for subject/from/date.
		fmt.Printf("envelope fetch failed (malformed server response), retrying without envelope: %v\n", err)
		fetchCmd.Close()
		fetchCmd = client.Fetch(seqSet, &imap.FetchOptions{
			Flags:        true,
			InternalDate: true,
			RFC822Size:   true,
			UID:          true,
		})
		buffers, err = fetchCmd.Collect()
		if err != nil {
			return nil, fmt.Errorf("fetch failed (even without envelope): %w", err)
		}
	}
	defer fetchCmd.Close()

	// Results come in ascending order; reverse for newest-first
	summaries := make([]EmailSummary, 0, len(buffers))
	for _, buf := range buffers {
		summary := EmailSummary{
			UID:   uint32(buf.UID),
			Flags: make([]string, len(buf.Flags)),
		}
		if buf.Envelope != nil {
			summary.Subject = decodeMimeHeader(buf.Envelope.Subject)
			summary.Date = buf.Envelope.Date.Format(time.RFC3339)
		}
		if summary.Date == "" && !buf.InternalDate.IsZero() {
			summary.Date = buf.InternalDate.Format(time.RFC3339)
		}
		if summary.Subject == "" {
			summary.Subject = "(No Subject)"
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
		if buf.Envelope != nil && len(buf.Envelope.From) > 0 {
			addr := buf.Envelope.From[0]
			if addr.Host != "" {
				summary.FromAddr = addr.Mailbox + "@" + addr.Host
			} else {
				summary.FromAddr = addr.Mailbox
			}
			summary.FromName = decodeMimeHeader(addr.Name)
			if summary.FromName != "" {
				summary.From = summary.FromName + " <" + summary.FromAddr + ">"
			} else {
				summary.From = summary.FromAddr
			}
		}
		if buf.Envelope != nil {
			for _, addr := range buf.Envelope.To {
				if addr.Host != "" {
					summary.To = append(summary.To, addr.Mailbox+"@"+addr.Host)
				} else {
					summary.To = append(summary.To, addr.Mailbox)
				}
			}
		}
		summaries = append(summaries, summary)
	}

	// Reverse to get newest first
	for i, j := 0, len(summaries)-1; i < j; i, j = i+1, j-1 {
		summaries[i], summaries[j] = summaries[j], summaries[i]
	}

	// Save meta + cache emails on every fetch. SaveEmails merges by UID without
	// removing older rows, so paging to an earlier page keeps those in the cache.
	if err := s.db.SaveFolderMeta(account.ID, folder, total); err != nil {
		fmt.Printf("failed to cache folder meta: %v\n", err)
	}
	if err := s.db.SaveEmails(account.ID, folder, summaries); err != nil {
		fmt.Printf("failed to cache emails: %v\n", err)
	}

	// The IMAP request above only fetched the requested page. Return the page
	// from the merged cache for page 1, so callers get newly fetched messages
	// and previously cached older messages in one response. For later pages,
	// keep returning the IMAP page unless the cache has enough rows to rebuild
	// that page completely.
	responseEmails := summaries
	if cached, cacheErr := s.db.LoadEmails(account.ID, folder); cacheErr == nil && len(cached) > 0 {
		cacheCoversPage := page <= 1 || len(cached) >= page*pageSize || len(cached) >= total
		if cacheCoversPage {
			responseEmails = s.paginate(cached, page, pageSize)
		}
	}

	return &EmailPageResponse{
		Emails: responseEmails,
		Total:  total,
	}, nil
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
		Subject: decodeMimeHeader(buf.Envelope.Subject),
		Date:    buf.Envelope.Date.Format(time.RFC3339),
		Flags:   make([]string, len(buf.Flags)),
	}

	// Copy flags from IMAP response
	for i, f := range buf.Flags {
		detail.Flags[i] = string(f)
	}

	// Detect suspicious (spam/junk) email
	folderLower := strings.ToLower(folder)
	isSpamFolder := strings.Contains(folderLower, "spam") ||
		strings.Contains(folderLower, "junk") ||
		strings.Contains(folderLower, "垃圾")
	hasJunkFlag := false
	for _, f := range detail.Flags {
		if strings.ToLower(f) == "\\junk" {
			hasJunkFlag = true
			break
		}
	}
	detail.IsSuspicious = isSpamFolder || hasJunkFlag

	// Build sender name + address from envelope
	for _, addr := range buf.Envelope.From {
		if addr.Host != "" {
			detail.FromAddr = addr.Mailbox + "@" + addr.Host
		} else {
			detail.FromAddr = addr.Mailbox
		}
		detail.FromName = decodeMimeHeader(addr.Name)
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

	// --- Walk body structure to find text/plain, text/html, and attachments ---
	var parts []bodyPartInfo
	var attachments []AttachmentInfo
	var inlineImgs []inlineImgInfo
	if buf.BodyStructure != nil {
		buf.BodyStructure.Walk(func(path []int, part imap.BodyStructure) bool {
			mt := part.MediaType()
			switch p := part.(type) {
			case *imap.BodyStructureSinglePart:
				if mt == "text/plain" || mt == "text/html" {
					// Body text part
					parts = append(parts, bodyPartInfo{
						path:     path,
						isHTML:   mt == "text/html",
						encoding: p.Encoding,
						charset:  p.Params["charset"],
					})
				} else if p.Disposition() != nil && p.Disposition().Value == "attachment" {
					// Explicit attachment (Content-Disposition: attachment)
					filename := decodeMimeHeader(p.Filename())
					if filename == "" {
						if name, ok := p.Params["name"]; ok {
							filename = decodeMimeHeader(name)
						} else {
							filename = fmt.Sprintf("attachment-%d", len(attachments)+1)
						}
					}
					attachments = append(attachments, AttachmentInfo{
						Filename: filename,
						MimeType: mt,
						Size:     p.Size,
						PartPath: path,
					})
				} else if fn := decodeMimeHeader(p.Filename()); fn != "" && !strings.HasPrefix(mt, "text/") {
					// Has a filename in Content-Type params — also capture CID if present (inline image)
					if cid := p.ID; cid != "" {
						inlineImgs = append(inlineImgs, inlineImgInfo{path: path, cid: strings.Trim(cid, "<>"), mime: mt, encoding: p.Encoding})
					}
					attachments = append(attachments, AttachmentInfo{
						Filename: fn,
						MimeType: mt,
						Size:     p.Size,
						PartPath: path,
						IsInline: true,
					})
				} else if p.Disposition() != nil && p.Disposition().Value == "inline" && !strings.HasPrefix(mt, "text/") {
					// Inline non-text (e.g. embedded image) — replace CID in HTML
					if cid := p.ID; cid != "" {
						inlineImgs = append(inlineImgs, inlineImgInfo{path: path, cid: strings.Trim(cid, "<>"), mime: mt, encoding: p.Encoding})
					}
					filename := decodeMimeHeader(p.Filename())
					if filename == "" {
						if name, ok := p.Params["name"]; ok {
							filename = decodeMimeHeader(name)
						} else {
							filename = fmt.Sprintf("inline-%d", len(attachments)+1)
						}
					}
					attachments = append(attachments, AttachmentInfo{
						Filename: filename,
						MimeType: mt,
						Size:     p.Size,
						PartPath: path,
					})
					fmt.Println("disposition + inline + !hasprefix text")
					fmt.Println(inlineImgs)
				} else if cid := p.ID; cid != "" && !strings.HasPrefix(mt, "text/") {
					// No disposition but has Content-ID — CID-referenced inline image
					inlineImgs = append(inlineImgs, inlineImgInfo{path: path, cid: strings.Trim(cid, "<>"), mime: mt, encoding: p.Encoding})
					fmt.Println("cid not nil + !hasprefix text")
					fmt.Println(inlineImgs)
					// Also record as downloadable attachment
					filename := decodeMimeHeader(p.Filename())
					if filename == "" {
						if name, ok := p.Params["name"]; ok {
							filename = decodeMimeHeader(name)
						} else {
							filename = fmt.Sprintf("inline-%d", len(attachments)+1)
						}
					}
					attachments = append(attachments, AttachmentInfo{
						Filename: filename,
						MimeType: mt,
						Size:     p.Size,
						PartPath: path,
						IsInline: true,
					})
				}
			case *imap.BodyStructureMultiPart:
				return true // continue recursing into children
			}
			return false
		})
	}

	detail.Attachments = attachments

	// --- Pass 2: Fetch the discovered body sections ---
	var sections []*imap.FetchItemBodySection
	if len(parts) == 0 && len(attachments) == 0 {
		sections = append(sections, &imap.FetchItemBodySection{Peek: true})
	}
	for _, p := range parts {
		sections = append(sections, &imap.FetchItemBodySection{Part: p.path, Peek: true})
	}
	// Only fetch inline image parts for CID resolution, skip regular download-only attachments
	for _, img := range inlineImgs {
		sections = append(sections, &imap.FetchItemBodySection{Part: img.path, Peek: true})
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
				} else if !isAttachmentSection(sec.Part, attachments) {
					// Fallback section (not an attachment) — try as plain text
					detail.BodyText = string(raw)
				}
			}

			// --- Resolve inline CID images ---
			if detail.BodyHTML != "" && len(inlineImgs) > 0 {
				cidMap := make(map[string]string)
				for _, sec := range sections {
					raw := buf2.FindBodySection(sec)
					if raw == nil {
						continue
					}
					for _, img := range inlineImgs {
						if !sectionPathEqual(sec.Part, img.path) {
							continue
						}
						var decoded []byte
						switch strings.ToLower(img.encoding) {
						case "base64":
							decoded, err = base64.StdEncoding.DecodeString(string(raw))
							if err != nil {
								// IMAP may wrap base64 at 76 chars — retry without whitespace
								s := strings.TrimSpace(strings.ReplaceAll(string(raw), "\n", ""))
								decoded, _ = base64.StdEncoding.DecodeString(s)
							}
						case "quoted-printable":
							decoded, _ = io.ReadAll(quotedprintable.NewReader(strings.NewReader(string(raw))))
						default:
							decoded = raw
						}
						var data string
						if len(decoded) > 0 {
							data = base64.StdEncoding.EncodeToString(decoded)
						} else {
							// If decode failed entirely, use raw bytes as-is
							data = base64.StdEncoding.EncodeToString(raw)
						}
						cidMap[img.cid] = "data:" + img.mime + ";base64," + data
						break
					}
				}
				for cid, dataURI := range cidMap {
					detail.BodyHTML = strings.ReplaceAll(detail.BodyHTML, "cid:"+cid, dataURI)
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

type inlineImgInfo struct {
	path     []int
	cid      string
	mime     string
	encoding string
}

// isAttachmentSection checks whether the given part path matches any of the
// discovered attachment paths.
func isAttachmentSection(path []int, attachments []AttachmentInfo) bool {
	for _, a := range attachments {
		if sectionPathEqual(path, a.PartPath) {
			return true
		}
	}
	return false
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

// DownloadAttachment fetches a specific MIME part from the IMAP server by its
// part path and writes the decoded body bytes to the given file path.
func (s *EmailService) DownloadAttachment(account EmailAccount, folder string, uid uint32, partPath []int, savePath string) error {
	client, err := dialIMAP(account, 60*time.Second)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Logout()

	// Select the mailbox (read-only)
	selectCmd := client.Select(folder, &imap.SelectOptions{ReadOnly: true})
	_, err = selectCmd.Wait()
	if err != nil {
		return wrapIMAPError("select folder failed", err)
	}

	// Build the UID set
	var uidSet imap.UIDSet
	uidSet.AddNum(imap.UID(uid))

	// Fetch only the attachment body section
	section := &imap.FetchItemBodySection{Part: partPath, Peek: true}
	fetchCmd := client.Fetch(uidSet, &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{section},
	})
	defer fetchCmd.Close()

	bufs, err := fetchCmd.Collect()
	if err != nil {
		return fmt.Errorf("fetch attachment failed: %w", err)
	}
	if len(bufs) == 0 {
		return fmt.Errorf("email not found: uid=%d", uid)
	}

	raw := bufs[0].FindBodySection(section)
	if raw == nil {
		return fmt.Errorf("attachment section not found")
	}

	// Write raw bytes to the destination file
	if err := os.WriteFile(savePath, raw, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
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
