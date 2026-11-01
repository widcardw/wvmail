package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTagMatchesUsesAnyKeywordAndSelectedFields(t *testing.T) {
	summary := EmailSummary{
		Subject:  "New arXiv submission",
		From:     "AAAI Conference <notice@aaai.org>",
		FromName: "AAAI Conference",
		FromAddr: "notice@aaai.org",
	}
	tag := MailTag{
		Enabled:  true,
		Keywords: []string{"arxiv", "kdd"},
		Fields:   []string{tagFieldSubject},
	}
	if !tagMatches(tag, summary, "") {
		t.Fatal("a keyword in the subject must match")
	}
	tag.Keywords = []string{"aaai.org"}
	tag.Fields = []string{tagFieldDomain}
	if !tagMatches(tag, summary, "") {
		t.Fatal("a keyword in the sender domain must match")
	}
	tag.Keywords = []string{"body-only"}
	tag.Fields = []string{tagFieldBody}
	if tagMatches(tag, summary, "") {
		t.Fatal("body rules must not inspect the summary")
	}
	if !tagMatches(tag, summary, "BODY-ONLY notice") {
		t.Fatal("body matching must be case-insensitive")
	}
	tag.Enabled = false
	if tagMatches(tag, summary, "body-only") {
		t.Fatal("disabled tags must never match")
	}
}

func TestClassifyCachedEmailsDeduplicatesGmailViews(t *testing.T) {
	tag := MailTag{
		ID:       "tag-arxiv",
		Enabled:  true,
		Keywords: []string{"arxiv"},
		Fields:   []string{tagFieldSubject},
	}
	emails := []CachedEmail{
		{AccountID: "gmail", FolderName: "INBOX", UID: 10, MessageID: "<same@example.com>", Subject: "arXiv daily"},
		{AccountID: "gmail", FolderName: "Important", UID: 20, MessageID: "<same@example.com>", Subject: "arXiv daily"},
		{AccountID: "gmail", FolderName: "All Mail", UID: 30, MessageID: "<same@example.com>", Subject: "arXiv daily"},
	}
	got := classifyCachedEmails(
		emails,
		nil,
		[]MailTag{tag},
		nil,
		map[string][]string{tagFolderKey("gmail", "All Mail"): {`\All`}},
		map[string]bool{tag.ID: true},
		time.Now(),
	)
	if len(got) != 1 {
		t.Fatalf("deduplicated result has %d rows, want 1: %+v", len(got), got)
	}
	if got[0].FolderName != "INBOX" || got[0].UID != 10 {
		t.Fatalf("canonical row must come from INBOX, got folder=%q uid=%d", got[0].FolderName, got[0].UID)
	}
	if len(got[0].TagIDs) != 1 || got[0].TagIDs[0] != tag.ID {
		t.Fatalf("tag IDs were not retained: %+v", got[0].TagIDs)
	}
}

func TestClassifyCachedEmailsAppliesExplicitExclusion(t *testing.T) {
	tag := MailTag{
		ID:       "tag-aaai",
		Enabled:  true,
		Keywords: []string{"aaai"},
		Fields:   []string{tagFieldSubject},
	}
	emails := []CachedEmail{{
		AccountID:  "acct",
		FolderName: "INBOX",
		UID:        7,
		Subject:    "AAAI author response",
	}}
	got := classifyCachedEmails(
		emails,
		nil,
		[]MailTag{tag},
		[]EmailTagOverride{{
			AccountID:  "acct",
			FolderName: "INBOX",
			UID:        7,
			TagID:      tag.ID,
			Included:   false,
		}},
		nil,
		map[string]bool{tag.ID: true},
		time.Now(),
	)
	if len(got) != 0 {
		t.Fatalf("an explicit exclusion must remove the automatic match: %+v", got)
	}
}

func TestParseTagSuggestionsAcceptsFencesAndRejectsInvalidRules(t *testing.T) {
	got, err := parseTagSuggestions("```json\n[{\"label\":\"arXiv\",\"color\":\"#4F83CC\",\"keywords\":[\"arxiv\",\"arXiv\"],\"fields\":[\"subject\",\"domain\",\"unknown\"]},{\"label\":\"Bad\",\"keywords\":[],\"fields\":[\"subject\"]}]\n```")
	if err != nil {
		t.Fatalf("parseTagSuggestions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d usable suggestions, want 1: %+v", len(got), got)
	}
	if got[0].Color != "#4f83cc" || len(got[0].Keywords) != 1 || len(got[0].Fields) != 2 {
		t.Fatalf("suggestion was not normalized: %+v", got[0])
	}
}

func TestTagSuggestionPreviewIsExactRequestPrompt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var sentPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &payload)
		if len(payload.Messages) > 0 {
			sentPrompt = payload.Messages[0].Content
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"[{\"label\":\"arXiv\",\"color\":\"#4f83cc\",\"keywords\":[\"arxiv\"],\"fields\":[\"subject\"]}]"}}]}`))
	}))
	t.Cleanup(server.Close)

	cfg := &ConfigService{}
	if err := cfg.SaveLLMConfigs([]LLMConfig{{
		Name:     "test",
		Provider: "custom",
		Model:    "test-model",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	}}); err != nil {
		t.Fatalf("SaveLLMConfigs: %v", err)
	}
	db := NewDBManager()
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{{
		UID:      1,
		Subject:  "New arXiv paper",
		From:     "arXiv <no-reply@arxiv.org>",
		FromAddr: "no-reply@arxiv.org",
		Date:     "2026-10-31T09:00:00Z",
	}}); err != nil {
		t.Fatalf("SaveEmails: %v", err)
	}

	svc := NewTagService(cfg)
	request := TagSuggestionRequest{
		AccountIDs:    []string{"acct"},
		SampleLimit:   20,
		Fields:        []string{"subject", "domain"},
		LLMConfigName: "test",
		LLMModel:      "test-model",
	}
	preview, err := svc.PreviewTagSuggestionPrompt(request)
	if err != nil {
		t.Fatalf("PreviewTagSuggestionPrompt: %v", err)
	}
	if preview.SampleCount != 1 || !strings.Contains(preview.Prompt, "New arXiv paper") {
		t.Fatalf("preview did not include the cached sample: %+v", preview)
	}
	if strings.Contains(preview.Prompt, "test-key") {
		t.Fatal("API key must never enter the prompt")
	}
	got, err := svc.GenerateTagSuggestions(request)
	if err != nil {
		t.Fatalf("GenerateTagSuggestions: %v", err)
	}
	if len(got) != 1 || got[0].Label != "arXiv" {
		t.Fatalf("unexpected suggestions: %+v", got)
	}
	if sentPrompt != preview.Prompt {
		t.Fatalf("preview and request prompt differ\npreview:\n%s\nsent:\n%s", preview.Prompt, sentPrompt)
	}
}
