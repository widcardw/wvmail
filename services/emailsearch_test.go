package services

import "testing"

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
