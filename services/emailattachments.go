package services

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
)

// Attachment collection and download: what ends up in the attachment list (the
// body's own inline images do not) and streaming one part out to a file.

// hasAttachmentDisposition reports whether a MIME part is explicitly marked as
// an attachment (Content-Disposition: attachment).
func hasAttachmentDisposition(p *imap.BodyStructureSinglePart) bool {
	d := p.Disposition()
	return d != nil && strings.EqualFold(d.Value, "attachment")
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

// visibleAttachments drops the inline parts the body already renders.
//
// Only images count as body content: plenty of clients put a Content-ID on every part, so
// a `.log`, `.zip` or `.pdf` can look "inline" in the structure while being a real file the
// user has to be able to download (measured on a real sent mail whose 25 MB `.log` part
// carries a Content-ID).
//
// Such an image is dropped when it is also among `rendered` — the paths whose bytes were
// substituted into the HTML. An inline image nothing references (no `cid:` in the body)
// stays, otherwise it would be invisible in the body *and* missing from the list.
//
// `rendered` is nil for rows loaded from an older cache: the substitution already happened
// before they were stored, so the body no longer records which CIDs it consumed. There,
// an inline image in the list is one the body shows, so all of them are dropped. An empty
// (non-nil) slice is the opposite, known case: nothing was inlined at all — a plain-text
// message has no body to show images in — so every inline image stays downloadable.
func visibleAttachments(attachments []AttachmentInfo, rendered [][]int) []AttachmentInfo {
	visible := make([]AttachmentInfo, 0, len(attachments))
	for _, a := range attachments {
		if a.IsInline && isImageMime(a.MimeType) {
			if rendered == nil {
				continue
			}
			shown := false
			for _, path := range rendered {
				if sectionPathEqual(path, a.PartPath) {
					shown = true
					break
				}
			}
			if shown {
				continue
			}
		}
		visible = append(visible, a)
	}
	return visible
}

// isImageMime reports whether a part is an image, i.e. something a body can display.
func isImageMime(mimeType string) bool {
	return strings.HasPrefix(strings.ToLower(mimeType), "image/")
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
