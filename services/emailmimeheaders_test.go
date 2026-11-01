package services

import (
	"testing"

	"github.com/emersion/go-imap/v2"
)

func TestMIMEPartHeadersRecoverMissingAttachmentMetadata(t *testing.T) {
	raw := []byte(
		"Content-Type: text/plain; charset=GB2312\r\n" +
			"Content-Disposition: attachment; filename=\"README.md\"\r\n" +
			"\r\n",
	)
	header, err := parseMIMEPartHeader(raw)
	if err != nil {
		t.Fatalf("parse MIME part header: %v", err)
	}
	part := &imap.BodyStructureSinglePart{
		Type:    "text",
		Subtype: "plain",
		Params:  map[string]string{"charset": "GB2312"},
	}

	mergeMIMEPartHeader(part, header)

	if !hasAttachmentDisposition(part) {
		t.Fatal("attachment disposition was not merged")
	}
	if got := part.Filename(); got != "README.md" {
		t.Fatalf("filename = %q, want README.md", got)
	}
}

func TestMIMEPartHeadersDoNotInventAttachmentDisposition(t *testing.T) {
	raw := []byte("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	header, err := parseMIMEPartHeader(raw)
	if err != nil {
		t.Fatalf("parse MIME part header: %v", err)
	}
	part := &imap.BodyStructureSinglePart{Type: "text", Subtype: "plain"}

	mergeMIMEPartHeader(part, header)

	if hasAttachmentDisposition(part) {
		t.Fatal("body part gained an attachment disposition")
	}
}
