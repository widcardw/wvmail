package services

import "testing"

// A bare address is trimmed of surrounding brackets and whitespace — and only
// those: the cutset used to be written as a raw string, which put `n`, `r`, `t`
// and `\` into it and turned "noreply@example.com" into "oreply@example.com".
func TestParseCachedContactKeepsLeadingLetters(t *testing.T) {
	tests := []struct {
		raw     string
		address string
		name    string
	}{
		{raw: "noreply@example.com", address: "noreply@example.com"},
		{raw: "notifications@vercel.com", address: "notifications@vercel.com"},
		{raw: "  <team@example.com>\r\n", address: "team@example.com"},
		{raw: "Alice <alice@example.com>", address: "alice@example.com", name: "Alice"},
		{raw: "no at sign", address: ""},
	}

	for _, tt := range tests {
		address, name := parseCachedContact(tt.raw)
		if address != tt.address || name != tt.name {
			t.Errorf("parseCachedContact(%q) = (%q, %q), want (%q, %q)", tt.raw, address, name, tt.address, tt.name)
		}
	}
}

func TestSearchEmailMatches(t *testing.T) {
	summary := EmailSummary{
		Subject:  "Project update",
		From:     "Alice <alice@example.com>",
		FromAddr: "alice@example.com",
		To:       []string{"me@example.com", "team@example.com"},
	}
	body := &CachedEmailBody{
		BodyText: "The release is ready for review.",
		To:       `["me@example.com", "team@example.com"]`,
	}

	tests := []struct {
		name    string
		filters []EmailSearchFilter
		want    bool
	}{
		{
			name:    "from and content are combined with and",
			filters: []EmailSearchFilter{{Field: "from", Value: "alice"}, {Field: "content", Value: "release"}},
			want:    true,
		},
		{
			name:    "to values are alternatives",
			filters: []EmailSearchFilter{{Field: "to", Value: "other@example.com"}, {Field: "to", Value: "team@example.com"}},
			want:    true,
		},
		{
			name:    "content pipe is or",
			filters: []EmailSearchFilter{{Field: "content", Value: "missing|review"}},
			want:    true,
		},
		{
			name:    "missing required condition fails",
			filters: []EmailSearchFilter{{Field: "from", Value: "bob"}, {Field: "content", Value: "release"}},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searchEmailMatches(summary, body, tt.filters); got != tt.want {
				t.Fatalf("searchEmailMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
