package services

import "testing"

func TestParseThreadingHeaders(t *testing.T) {
	raw := []byte("References: <root@example.com>\r\n <second@example.com>\r\n" +
		"In-Reply-To: <second@example.com>\r\n" +
		"List-Id: <announce.example.com>\r\n" +
		"\r\n")
	headers, err := parseThreadingHeaders(raw)
	if err != nil {
		t.Fatalf("parseThreadingHeaders: %v", err)
	}
	if len(headers.References) != 2 ||
		headers.References[0] != "root@example.com" ||
		headers.References[1] != "second@example.com" {
		t.Fatalf("references = %v", headers.References)
	}
	if len(headers.InReplyTo) != 1 || headers.InReplyTo[0] != "second@example.com" {
		t.Fatalf("in-reply-to = %v", headers.InReplyTo)
	}
	if !headers.Bulk {
		t.Fatal("a List-Id header must mark the message as bulk")
	}
}

func TestParseThreadingHeadersBulkSignals(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"auto submitted", "Auto-Submitted: auto-generated\r\n\r\n", true},
		{"auto submitted no", "Auto-Submitted: no\r\n\r\n", false},
		{"precedence bulk", "Precedence: Bulk\r\n\r\n", true},
		{"ordinary", "Subject: hello\r\n\r\n", false},
	}
	for _, tc := range cases {
		headers, err := parseThreadingHeaders([]byte(tc.raw))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if headers.Bulk != tc.want {
			t.Errorf("%s: Bulk = %v, want %v", tc.name, headers.Bulk, tc.want)
		}
	}
}
