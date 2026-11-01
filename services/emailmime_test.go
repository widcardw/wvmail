package services

import (
	"bytes"
	"io"
	"strings"
	"testing"

	gomail "github.com/emersion/go-message/mail"
)

func TestBuildEmailMessageMultipart(t *testing.T) {
	account := EmailAccount{Name: "测试发件人", Email: "sender@example.com"}
	req := SendEmailRequest{
		To:      []EmailAddress{{Name: "收件人", Address: "to@example.com"}},
		Cc:      []EmailAddress{{Address: "cc@example.com"}},
		Bcc:     []EmailAddress{{Address: "bcc@example.com"}},
		Subject: "中文主题",
		Text:    "纯文本正文",
		HTML:    `<p>HTML 正文</p><img src="cid:wvmail-image">`,
	}
	attachments := []preparedAttachment{{
		filename:    "报告.txt",
		contentType: "text/plain",
		data:        []byte("attachment body"),
	}}
	inlineImages := []preparedAttachment{{
		filename:    "图片.png",
		contentType: "image/png",
		data:        []byte{0x89, 'P', 'N', 'G'},
		contentID:   "wvmail-image",
	}}

	built, err := buildEmailMessage(account, req, attachments, inlineImages)
	if err != nil {
		t.Fatalf("buildEmailMessage() error = %v", err)
	}
	if built.messageID == "" {
		t.Fatal("Message-ID is empty")
	}
	if bytes.Contains(built.raw, []byte("bcc@example.com")) {
		t.Fatal("Bcc address leaked into the MIME headers")
	}

	reader, err := gomail.CreateReader(bytes.NewReader(built.raw))
	if err != nil {
		t.Fatalf("CreateReader() error = %v", err)
	}
	defer reader.Close()
	if subject, err := reader.Header.Subject(); err != nil || subject != req.Subject {
		t.Fatalf("Subject = %q, %v; want %q", subject, err, req.Subject)
	}

	var sawPlain, sawHTML, sawInline, sawAttachment bool
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart() error = %v", err)
		}
		body, err := io.ReadAll(part.Body)
		if err != nil {
			t.Fatalf("read part error = %v", err)
		}
		contentType := strings.ToLower(part.Header.Get("Content-Type"))
		disposition := strings.ToLower(part.Header.Get("Content-Disposition"))
		contentID := part.Header.Get("Content-ID")
		switch {
		case strings.HasPrefix(contentType, "text/plain") && strings.Contains(string(body), "纯文本正文"):
			sawPlain = true
		case strings.HasPrefix(contentType, "text/html") && strings.Contains(string(body), "HTML 正文"):
			sawHTML = true
		case strings.HasPrefix(contentType, "image/png") && strings.Contains(disposition, "inline") && contentID == "<wvmail-image>":
			sawInline = true
		case strings.HasPrefix(contentType, "text/plain") && strings.Contains(disposition, "attachment") && strings.Contains(string(body), "attachment body"):
			sawAttachment = true
		}
	}
	if !sawPlain || !sawHTML || !sawInline || !sawAttachment {
		t.Fatalf("parts: plain=%v html=%v inline=%v attachment=%v", sawPlain, sawHTML, sawInline, sawAttachment)
	}
}

func TestBuildEmailMessageSinglePlainText(t *testing.T) {
	built, err := buildEmailMessage(
		EmailAccount{Email: "sender@example.com"},
		SendEmailRequest{
			To:      []EmailAddress{{Address: "to@example.com"}},
			Subject: "plain",
			Text:    "hello",
		},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("buildEmailMessage() error = %v", err)
	}
	if !bytes.Contains(built.raw, []byte("Content-Type: text/plain")) {
		t.Fatalf("plain-text message did not contain a text/plain part:\n%s", built.raw)
	}
}

func TestParseMailto(t *testing.T) {
	input, err := parseMailto("mailto:Name%20%3Ca%40example.com%3E,b%40example.com?cc=c%40example.com&bcc=d%40example.com&subject=Hello%20World&body=Line1%0ALine2")
	if err != nil {
		t.Fatalf("parseMailto() error = %v", err)
	}
	if len(input.To) != 2 || input.To[0].Name != "Name" || input.To[0].Address != "a@example.com" {
		t.Fatalf("To = %#v", input.To)
	}
	if len(input.Cc) != 1 || input.Cc[0].Address != "c@example.com" {
		t.Fatalf("Cc = %#v", input.Cc)
	}
	if len(input.Bcc) != 1 || input.Bcc[0].Address != "d@example.com" {
		t.Fatalf("Bcc = %#v", input.Bcc)
	}
	if input.Subject != "Hello World" {
		t.Fatalf("Subject = %q", input.Subject)
	}
	if !strings.Contains(input.HTML, "Line1<br>Line2") {
		t.Fatalf("HTML = %q", input.HTML)
	}
}

func TestSentFolderName(t *testing.T) {
	folders := []FolderInfo{
		{Name: "Inbox", Attrs: []string{`\Inbox`}},
		{Name: "已发送", Attrs: []string{`\Sent`}},
	}
	if got := sentFolderName(folders); got != "已发送" {
		t.Fatalf("sentFolderName() = %q", got)
	}
	if got := sentFolderName([]FolderInfo{{Name: "[Gmail]/Sent Mail"}}); got != "[Gmail]/Sent Mail" {
		t.Fatalf("fallback sentFolderName() = %q", got)
	}
}

func TestHTMLToPlainText(t *testing.T) {
	got := htmlToPlainText(`<p>Hello <strong>world</strong></p><p>Second<br>line</p>`)
	if got != "Hello world\n\nSecond\nline" {
		t.Fatalf("htmlToPlainText() = %q", got)
	}
}
