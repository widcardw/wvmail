package services

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	stdhtml "html"
	"mime"
	"net/http"
	netmail "net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-message"
	gomail "github.com/emersion/go-message/mail"
	xhtml "golang.org/x/net/html"
)

type preparedAttachment struct {
	filename    string
	contentType string
	data        []byte
	contentID   string
}

type builtEmail struct {
	raw       []byte
	messageID string
}

func buildEmailMessage(account EmailAccount, req SendEmailRequest, attachments, inlineImages []preparedAttachment) (builtEmail, error) {
	from, err := mailAddress(account.SenderName, account.Email)
	if err != nil {
		return builtEmail{}, fmt.Errorf("invalid sender address: %w", err)
	}
	to, err := mailAddressList(req.To)
	if err != nil {
		return builtEmail{}, fmt.Errorf("invalid To address: %w", err)
	}
	cc, err := mailAddressList(req.Cc)
	if err != nil {
		return builtEmail{}, fmt.Errorf("invalid Cc address: %w", err)
	}

	var header gomail.Header
	header.SetAddressList("From", []*gomail.Address{from})
	header.SetAddressList("To", to)
	header.SetAddressList("Cc", cc)
	header.SetSubject(strings.TrimSpace(req.Subject))
	header.SetDate(time.Now())
	if domain := addressDomain(account.Email); domain != "" {
		if err := header.GenerateMessageIDWithHostname(domain); err != nil {
			return builtEmail{}, fmt.Errorf("generate Message-ID: %w", err)
		}
	} else if err := header.GenerateMessageID(); err != nil {
		return builtEmail{}, fmt.Errorf("generate Message-ID: %w", err)
	}
	header.Set("X-Mailer", "wvmail")
	if inReplyTo := trimMessageID(req.InReplyTo); inReplyTo != "" {
		header.SetMsgIDList("In-Reply-To", []string{inReplyTo})
	}
	if references := cleanMessageIDs(req.References); len(references) > 0 {
		header.SetMsgIDList("References", references)
	}

	messageID, err := header.MessageID()
	if err != nil {
		return builtEmail{}, fmt.Errorf("read generated Message-ID: %w", err)
	}

	var out bytes.Buffer
	contentType := "multipart/alternative"
	switch {
	case len(attachments) > 0:
		contentType = "multipart/mixed"
	case len(inlineImages) > 0:
		contentType = mime.FormatMediaType("multipart/related", map[string]string{
			"type": "multipart/alternative",
		})
	}
	header.Set("Content-Type", contentType)

	writer, err := message.CreateWriter(&out, header.Header)
	if err != nil {
		return builtEmail{}, fmt.Errorf("create MIME message: %w", err)
	}

	if len(attachments) > 0 {
		if len(inlineImages) > 0 {
			related, err := createMultipartPart(writer, mime.FormatMediaType("multipart/related", map[string]string{
				"type": "multipart/alternative",
			}))
			if err != nil {
				return builtEmail{}, err
			}
			if err := writeBodyParts(related, req.Text, req.HTML); err != nil {
				return builtEmail{}, err
			}
			if err := writeInlineParts(related, inlineImages); err != nil {
				return builtEmail{}, err
			}
			if err := related.Close(); err != nil {
				return builtEmail{}, fmt.Errorf("close related MIME part: %w", err)
			}
		} else if err := writeBodyParts(writer, req.Text, req.HTML); err != nil {
			return builtEmail{}, err
		}
		for _, attachment := range attachments {
			if err := writeFilePart(writer, attachment, false); err != nil {
				return builtEmail{}, err
			}
		}
	} else {
		if err := writeBodyParts(writer, req.Text, req.HTML); err != nil {
			return builtEmail{}, err
		}
		if len(inlineImages) > 0 {
			if err := writeInlineParts(writer, inlineImages); err != nil {
				return builtEmail{}, err
			}
		}
	}

	if err := writer.Close(); err != nil {
		return builtEmail{}, fmt.Errorf("close MIME message: %w", err)
	}
	if out.Len() > maxOutgoingMessageBytes {
		return builtEmail{}, fmt.Errorf("message is too large: %d bytes (limit %d)", out.Len(), maxOutgoingMessageBytes)
	}
	return builtEmail{raw: out.Bytes(), messageID: messageID}, nil
}

func createMultipartPart(writer *message.Writer, contentType string) (*message.Writer, error) {
	var header message.Header
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("create %s part: %w", contentType, err)
	}
	return part, nil
}

func writeBodyParts(writer *message.Writer, text, htmlBody string) error {
	text = strings.TrimRight(text, "\r\n")
	htmlBody = strings.TrimSpace(htmlBody)

	switch {
	case text != "" && htmlBody != "":
		alternative, err := createMultipartPart(writer, "multipart/alternative")
		if err != nil {
			return err
		}
		if err := writeTextPart(alternative, "text/plain; charset=utf-8", []byte(text+"\r\n")); err != nil {
			return err
		}
		if err := writeTextPart(alternative, "text/html; charset=utf-8", []byte(htmlBody)); err != nil {
			return err
		}
		if err := alternative.Close(); err != nil {
			return fmt.Errorf("close alternative MIME part: %w", err)
		}
		return nil
	case htmlBody != "":
		return writeTextPart(writer, "text/html; charset=utf-8", []byte(htmlBody))
	default:
		return writeTextPart(writer, "text/plain; charset=utf-8", []byte(text+"\r\n"))
	}
}

func writeTextPart(writer *message.Writer, contentType string, body []byte) error {
	var header message.Header
	header.Set("Content-Type", contentType)
	header.Set("Content-Transfer-Encoding", "quoted-printable")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create text MIME part: %w", err)
	}
	if _, err := part.Write(body); err != nil {
		_ = part.Close()
		return fmt.Errorf("write text MIME part: %w", err)
	}
	if err := part.Close(); err != nil {
		return fmt.Errorf("close text MIME part: %w", err)
	}
	return nil
}

func writeInlineParts(writer *message.Writer, inlineImages []preparedAttachment) error {
	for _, image := range inlineImages {
		if err := writeFilePart(writer, image, true); err != nil {
			return err
		}
	}
	return nil
}

func writeFilePart(writer *message.Writer, file preparedAttachment, inline bool) error {
	var header message.Header
	header.Set("Content-Type", file.contentType)
	header.Set("Content-Transfer-Encoding", "base64")
	disposition := "attachment"
	if inline {
		disposition = "inline"
	}
	header.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{
		"filename": file.filename,
	}))
	if inline {
		header.Set("Content-ID", "<"+file.contentID+">")
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create attachment MIME part %q: %w", file.filename, err)
	}
	if _, err := part.Write(file.data); err != nil {
		_ = part.Close()
		return fmt.Errorf("write attachment MIME part %q: %w", file.filename, err)
	}
	if err := part.Close(); err != nil {
		return fmt.Errorf("close attachment MIME part %q: %w", file.filename, err)
	}
	return nil
}

func mailAddress(name, address string) (*gomail.Address, error) {
	address = strings.TrimSpace(address)
	if address == "" || strings.ContainsAny(address, "\r\n") {
		return nil, fmt.Errorf("empty or invalid address")
	}
	parsed, err := netmail.ParseAddress(address)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = parsed.Name
	}
	return &gomail.Address{Name: displayName, Address: parsed.Address}, nil
}

func mailAddressList(addresses []EmailAddress) ([]*gomail.Address, error) {
	result := make([]*gomail.Address, 0, len(addresses))
	for _, address := range addresses {
		parsed, err := mailAddress(address.Name, address.Address)
		if err != nil {
			return nil, err
		}
		result = append(result, parsed)
	}
	return result, nil
}

func addressDomain(address string) string {
	parsed, err := netmail.ParseAddress(strings.TrimSpace(address))
	if err != nil {
		return ""
	}
	if at := strings.LastIndexByte(parsed.Address, '@'); at >= 0 && at+1 < len(parsed.Address) {
		return parsed.Address[at+1:]
	}
	return ""
}

func trimMessageID(id string) string {
	return strings.Trim(strings.TrimSpace(id), "<>")
}

func cleanMessageIDs(ids []string) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if cleaned := trimMessageID(id); cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

func prepareOutgoingFile(path, filename, contentType string, inline bool) (preparedAttachment, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return preparedAttachment{}, fmt.Errorf("attachment path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return preparedAttachment{}, fmt.Errorf("stat attachment %q: %w", filename, err)
	}
	if !info.Mode().IsRegular() {
		return preparedAttachment{}, fmt.Errorf("attachment %q is not a regular file", filename)
	}
	if info.Size() > maxOutgoingAttachmentBytes {
		return preparedAttachment{}, fmt.Errorf("attachment %q is too large: %d bytes (limit %d)", filename, info.Size(), maxOutgoingAttachmentBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return preparedAttachment{}, fmt.Errorf("read attachment %q: %w", filename, err)
	}
	if filename = sanitizeAttachmentFilename(filename); filename == "" {
		filename = sanitizeAttachmentFilename(filepath.Base(path))
	}
	if filename == "" {
		filename = "attachment"
	}
	if contentType = normalizeContentType(contentType, filename, data); contentType == "" {
		return preparedAttachment{}, fmt.Errorf("cannot determine content type for %q", filename)
	}
	if inline && !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return preparedAttachment{}, fmt.Errorf("inline attachment %q is not an image", filename)
	}
	contentID := ""
	if inline {
		contentID = "wvmail-" + randomHex(16)
	}
	return preparedAttachment{
		filename:    filename,
		contentType: contentType,
		data:        data,
		contentID:   contentID,
	}, nil
}

func normalizeContentType(contentType, filename string, data []byte) string {
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	if contentType != "" {
		return contentType
	}
	if byExtension := mime.TypeByExtension(filepath.Ext(filename)); byExtension != "" {
		return byExtension
	}
	if len(data) == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(data)
}

func sanitizeAttachmentFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	return strings.Trim(name, " .")
}

func htmlToPlainText(input string) string {
	doc, err := xhtml.Parse(strings.NewReader(input))
	if err != nil {
		return strings.TrimSpace(input)
	}
	var out strings.Builder
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.TextNode {
			out.WriteString(node.Data)
			return
		}
		switch node.Data {
		case "br":
			out.WriteByte('\n')
		case "p", "div", "li", "blockquote", "pre", "h1", "h2", "h3", "h4", "h5", "h6":
			if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n\n") {
				out.WriteByte('\n')
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if node.Data == "p" || node.Data == "div" || node.Data == "li" || node.Data == "blockquote" || node.Data == "pre" || strings.HasPrefix(node.Data, "h") {
			out.WriteByte('\n')
		}
	}
	walk(doc)
	text := out.String()
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}

func textToHTML(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	paragraphs := strings.Split(text, "\n\n")
	var out strings.Builder
	for _, paragraph := range paragraphs {
		if strings.TrimSpace(paragraph) == "" {
			continue
		}
		out.WriteString("<p>")
		lines := strings.Split(paragraph, "\n")
		for i, line := range lines {
			if i > 0 {
				out.WriteString("<br>")
			}
			out.WriteString(stdhtml.EscapeString(line))
		}
		out.WriteString("</p>")
	}
	return out.String()
}

func randomHex(bytesN int) string {
	data := make([]byte, bytesN)
	if _, err := rand.Read(data); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(data)
}
