package services

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParseOpenAIResponseRejectsReasoningOnlyTruncation(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"choices": [{
				"finish_reason": "length",
				"message": {
					"content": "",
					"reasoning_content": "The model used the whole budget while thinking."
				}
			}]
		}`)),
	}
	_, err := parseOpenAIResponse(resp)
	if err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("expected an actionable truncation error, got %v", err)
	}
}

func TestParseOpenAIResponseAcceptsVisibleContent(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"[]"}}]}`)),
	}
	got, err := parseOpenAIResponse(resp)
	if err != nil {
		t.Fatalf("parseOpenAIResponse: %v", err)
	}
	if got != "[]" {
		t.Fatalf("got %q, want []", got)
	}
}
