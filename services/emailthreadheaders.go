package services

import (
	"bytes"
	"fmt"
	"net/mail"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type threadingHeaders struct {
	InReplyTo  []string
	References []string
	Bulk       bool
}

func threadingHeaderSection() *imap.FetchItemBodySection {
	return &imap.FetchItemBodySection{
		Specifier: imap.PartSpecifierHeader,
		HeaderFields: []string{
			"References",
			"In-Reply-To",
			"List-Id",
			"Auto-Submitted",
			"Precedence",
		},
		Peek: true,
	}
}

// fetchThreadingHeaders asks for the few headers IMAP's ENVELOPE does not
// expose. It is one bulk FETCH for the page, never one connection per message.
func fetchThreadingHeaders(
	client *imapclient.Client,
	uidSet imap.UIDSet,
) (map[uint32]threadingHeaders, error) {
	section := threadingHeaderSection()
	fetchCmd := client.Fetch(uidSet, &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{section},
	})
	defer fetchCmd.Close()

	buffers, err := fetchCmd.Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch threading headers: %w", err)
	}

	headers := make(map[uint32]threadingHeaders, len(buffers))
	for _, buffer := range buffers {
		raw := buffer.FindBodySection(section)
		if len(raw) == 0 {
			continue
		}
		parsed, err := parseThreadingHeaders(raw)
		if err != nil {
			continue
		}
		headers[uint32(buffer.UID)] = parsed
	}
	return headers, nil
}

func parseThreadingHeaders(raw []byte) (threadingHeaders, error) {
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return threadingHeaders{}, err
	}
	precedence := strings.ToLower(strings.TrimSpace(message.Header.Get("Precedence")))
	autoSubmitted := strings.TrimSpace(message.Header.Get("Auto-Submitted"))
	return threadingHeaders{
		InReplyTo:  parseMessageIDHeader(message.Header.Get("In-Reply-To")),
		References: parseMessageIDHeader(message.Header.Get("References")),
		Bulk: message.Header.Get("List-Id") != "" ||
			(autoSubmitted != "" && !strings.EqualFold(autoSubmitted, "no")) ||
			precedence == "bulk" || precedence == "list" || precedence == "junk",
	}, nil
}

func parseMessageIDHeader(value string) []string {
	fields := strings.Fields(value)
	ids := make([]string, 0, len(fields))
	for _, field := range fields {
		id := strings.TrimSpace(strings.Trim(field, "<>"))
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func mergeThreadingHeaders(summary *EmailSummary, headers threadingHeaders) {
	if len(headers.InReplyTo) > 0 {
		summary.InReplyTo = headers.InReplyTo
	}
	if len(headers.References) > 0 {
		summary.References = headers.References
	}
	summary.IsBulk = headers.Bulk
}
