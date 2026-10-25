package services

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"mime/quotedprintable"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
)

// One message: the headers, the MIME walk, the body text and the attachments of
// a single mail, plus the spam gate that decides whether the body may render.
// Decoding helpers live in emaildecode.go.

// isSuspiciousMail reports whether a message has to stay behind the spam gate: it was
// listed from a spam/Junk folder, or the server itself flagged it \Junk.
//
// This is derived state and deliberately not part of the stored body: the cache holds the
// flags, so the rule must be applied on the cache path as well (see FetchEmail). Applying
// it on the fetch path only meant a spam message passed the gate on every open after the
// first — the first open caches the body, and the cached detail had IsSuspicious false.
func isSuspiciousMail(folder string, flags []string) bool {
	folderLower := strings.ToLower(folder)
	if strings.Contains(folderLower, "spam") ||
		strings.Contains(folderLower, "junk") ||
		strings.Contains(folderLower, "垃圾") {
		return true
	}
	for _, f := range flags {
		if strings.EqualFold(f, `\junk`) {
			return true
		}
	}
	return false
}

// FetchEmail retrieves the full content of a single email by its UID.
// Returns cached data if available.
func (s *EmailService) FetchEmail(account EmailAccount, folder string, uid uint32) (*EmailDetail, error) {
	// Try cache first
	cached, err := s.db.LoadEmailBody(account.ID, folder, uid)
	if err == nil && cached != nil {
		// The stored row has the flags but not the verdict, so recompute it here:
		// without this a spam message looked harmless as soon as its body was cached.
		cached.IsSuspicious = isSuspiciousMail(folder, cached.Flags)
		// Rows cached before inline parts were hidden still list them (see
		// visibleAttachments); the body they came with already shows them inline.
		cached.Attachments = visibleAttachments(cached.Attachments, nil)
		// Bodies cached before the quoted-printable heuristic existed still hold raw
		// escapes; repair them here so an old message does not stay broken.
		if repairCachedEncoding(cached) {
			if err := s.db.SaveEmailBody(account.ID, folder, cached); err != nil {
				slog.Warn("body: failed to persist repaired email body", "uid", uid, "error", err)
			}
		}
		if cachedHTMLHasCharsetCorruption(cached) {
			slog.Warn("body: cached HTML looks like UTF-8 decoded as GBK; refetching original body",
				"account", account, "folder", folder, "uid", uid)
			refreshed, refreshErr := s.RefreshEmailBody(account, folder, uid)
			if refreshErr != nil {
				slog.Warn("body: failed to repair cached charset by refetching; showing cached body as fallback",
					"account", account, "folder", folder, "uid", uid, "error", refreshErr)
				return cached, nil
			}
			return refreshed, nil
		}
		return cached, nil
	}

	return s.RefreshEmailBody(account, folder, uid)
}

// repairCachedEncoding decodes already-cached bodies that are really quoted-printable
// (`style=3D"…"`, `=E8=AE=A1`) but were stored raw because their part advertised
// 7bit/8bit. It reports whether anything changed, so the caller can write it back once.
func repairCachedEncoding(detail *EmailDetail) bool {
	changed := false
	if decoded, ok := tryQuotedPrintable([]byte(detail.BodyHTML)); ok {
		if text, ok := preferCleanerDecode(decoded, []byte(detail.BodyHTML), "utf-8"); ok {
			detail.BodyHTML = text
			changed = true
		}
	}
	if decoded, ok := tryQuotedPrintable([]byte(detail.BodyText)); ok {
		if text, ok := preferCleanerDecode(decoded, []byte(detail.BodyText), "utf-8"); ok {
			detail.BodyText = text
			changed = true
		}
	}
	return changed
}

// cachedHTMLHasCharsetCorruption reports whether a stored body still reads as UTF-8
// text that was decoded as GBK. The rewrite needs the original bytes to be safe, and a
// stored row no longer has them, so the caller refetches once instead of guessing.
func cachedHTMLHasCharsetCorruption(detail *EmailDetail) bool {
	_, corrupt := repairGBKMojibake(detail.BodyHTML)
	return corrupt
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
		InternalDate:  true,
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
		UID:       uint32(buf.UID),
		MessageID: strings.TrimSpace(buf.Envelope.MessageID),
		Subject:   decodeMimeHeader(buf.Envelope.Subject),
		Flags:     make([]string, len(buf.Flags)),
	}
	// UTC, like the list rows: a missing Date header leaves the field empty for
	// the InternalDate fallback rather than printing year 1.
	if !buf.Envelope.Date.IsZero() {
		detail.Date = buf.Envelope.Date.UTC().Format(time.RFC3339)
	} else if !buf.InternalDate.IsZero() {
		detail.Date = buf.InternalDate.UTC().Format(time.RFC3339)
	}

	// Copy flags from IMAP response
	for i, f := range buf.Flags {
		detail.Flags[i] = string(f)
	}

	// Detect suspicious (spam/junk) email
	detail.IsSuspicious = isSuspiciousMail(folder, detail.Flags)

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
				// Content-Disposition wins over the media type: a part the sender
				// marked as an attachment must not be offered as the message body.
				// Without this, an attached .txt/.html was rendered as the body
				// (and could overwrite the real body text).
				if (mt == "text/plain" || mt == "text/html") && !hasAttachmentDisposition(p) {
					// Body text part
					parts = append(parts, bodyPartInfo{
						path:     path,
						isHTML:   mt == "text/html",
						encoding: p.Encoding,
						charset:  p.Params["charset"],
					})
				} else if hasAttachmentDisposition(p) {
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
						// Inline by disposition. Whether it also stays in the list
						// depends on the body actually showing it (see
						// visibleAttachments).
						IsInline: true,
					})
				} else if cid := p.ID; cid != "" && !strings.HasPrefix(mt, "text/") {
					// No disposition but has Content-ID — CID-referenced inline image
					inlineImgs = append(inlineImgs, inlineImgInfo{path: path, cid: strings.Trim(cid, "<>"), mime: mt, encoding: p.Encoding})
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
	// Part paths whose bytes were substituted into the body as `data:` URIs; the body
	// shows those, so they must not also sit in the attachment list. Empty (not nil) means
	// "nothing was inlined", which keeps every inline part listed — nil is reserved for
	// rows read back from an old cache, where the answer is unknown (see
	// visibleAttachments).
	renderedInline := make([][]int, 0, len(inlineImgs))
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
					// Whole-message fallback section (no usable body structure
					// was advertised): decode it like a body part so CTE and
					// charset are still handled instead of dumping raw bytes.
					detail.BodyText = decodeBodyPart(raw, "", "")
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
					marker := "cid:" + cid
					if !strings.Contains(detail.BodyHTML, marker) {
						// Nothing in the body references this part, so the body does not
						// show it: leave it in the attachment list (see below).
						continue
					}
					detail.BodyHTML = strings.ReplaceAll(detail.BodyHTML, marker, dataURI)
					for _, img := range inlineImgs {
						if img.cid == cid {
							renderedInline = append(renderedInline, img.path)
							break
						}
					}
				}
			}
		}
	}

	// Inline parts are rendered by the body itself; listing them as downloadable
	// attachments as well was noise (and inflated the "N attachments" header). Parts the
	// body never referenced stay listed, or they would be invisible everywhere.
	detail.Attachments = visibleAttachments(attachments, renderedInline)

	// Bodies that were double-encoded upstream (UTF-8 bytes decoded as GBK by a
	// gateway, or by an older build of this decoder) can be reversed here: the
	// rewrite only touches runs whose GBK encoding is itself valid UTF-8, so a
	// correctly decoded body is left byte-for-byte alone.
	for _, field := range []struct {
		name string
		body *string
	}{{"html", &detail.BodyHTML}, {"text", &detail.BodyText}} {
		if repaired, ok := repairGBKMojibake(*field.body); ok {
			slog.Warn("body: repaired UTF-8-as-GBK mojibake",
				"account", account, "folder", folder, "uid", uid, "part", field.name)
			*field.body = repaired
		}
	}

	// Update cache (best-effort)
	if err := s.db.SaveEmailBody(account.ID, folder, detail); err != nil {
		slog.Warn("body: failed to cache email body",
			"account", account, "folder", folder, "uid", uid, "error", err)
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
