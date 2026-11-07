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

func TestBuiltinTagsAreSeededPreservedAndProtected(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := NewTagService(&ConfigService{})
	tags, err := svc.ListMailTags()
	if err != nil {
		t.Fatalf("ListMailTags: %v", err)
	}
	if len(tags) != len(builtinTagDefinitions) {
		t.Fatalf("got %d tags, want %d built-in tags", len(tags), len(builtinTagDefinitions))
	}

	byID := make(map[string]MailTag, len(tags))
	for _, tag := range tags {
		byID[tag.ID] = tag
		if !tag.BuiltIn {
			t.Fatalf("tag %q was not marked as built-in", tag.ID)
		}
	}
	verification, ok := byID[builtinTagVerification]
	if !ok {
		t.Fatalf("missing built-in verification tag: %+v", tags)
	}

	again, err := svc.ListMailTags()
	if err != nil {
		t.Fatalf("ListMailTags (second call): %v", err)
	}
	if len(again) != len(tags) {
		t.Fatalf("second load returned %d tags, want %d", len(again), len(tags))
	}

	verification.Label = "我的验证消息"
	verification.Enabled = false
	saved, err := svc.SaveMailTag(verification)
	if err != nil {
		t.Fatalf("SaveMailTag(built-in): %v", err)
	}
	if !saved.BuiltIn {
		t.Fatal("editing a built-in tag must not clear its built-in marker")
	}

	reloaded, err := svc.ListMailTags()
	if err != nil {
		t.Fatalf("ListMailTags after edit: %v", err)
	}
	var edited MailTag
	for _, tag := range reloaded {
		if tag.ID == builtinTagVerification {
			edited = tag
			break
		}
	}
	if edited.Label != "我的验证消息" || edited.Enabled {
		t.Fatalf("built-in edit was not preserved: %+v", edited)
	}

	if err := svc.DeleteMailTag(builtinTagVerification); err == nil {
		t.Fatal("built-in tags must not be deletable")
	}
}

func TestBuiltinTagsMatchRepresentativeMail(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := NewTagService(&ConfigService{})
	tags, err := svc.ListMailTags()
	if err != nil {
		t.Fatalf("ListMailTags: %v", err)
	}
	byID := make(map[string]MailTag, len(tags))
	for _, tag := range tags {
		byID[tag.ID] = tag
	}

	tests := []struct {
		name    string
		tagID   string
		subject string
		body    string
	}{
		{
			name:    "verification",
			tagID:   builtinTagVerification,
			subject: "Your verification code is 123456",
		},
		{
			name:    "billing",
			tagID:   builtinTagBilling,
			subject: "Invoice for October",
		},
		{
			name:    "travel",
			tagID:   builtinTagTravel,
			subject: "Your flight itinerary",
		},
		{
			name:    "schedule",
			tagID:   builtinTagSchedule,
			subject: "Meeting invitation: roadmap review",
		},
		{
			name:    "subscription",
			tagID:   builtinTagSubscription,
			subject: "Weekly newsletter",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tag, ok := byID[test.tagID]
			if !ok {
				t.Fatalf("missing built-in tag %q", test.tagID)
			}
			summary := EmailSummary{Subject: test.subject}
			if !tagMatches(tag, summary, test.body) {
				t.Fatalf("built-in tag %q did not match %q", test.tagID, test.subject)
			}
		})
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

func TestListTagFilteredEmailsScopesBeforePaginating(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{
		{UID: 1, Subject: "New arXiv paper", Date: "2026-10-31T09:00:00Z"},
		{UID: 2, Subject: "KDD deadline", Date: "2026-10-30T09:00:00Z"},
		{UID: 3, Subject: "Ordinary notice", Date: "2026-10-29T09:00:00Z"},
	}); err != nil {
		t.Fatalf("SaveEmails(INBOX): %v", err)
	}
	if err := db.SaveEmails("acct", "Archive", []EmailSummary{
		{UID: 4, Subject: "Archived arXiv paper", Date: "2026-10-28T09:00:00Z"},
	}); err != nil {
		t.Fatalf("SaveEmails(Archive): %v", err)
	}
	if err := db.SaveEmails("other", "INBOX", []EmailSummary{
		{UID: 5, Subject: "Another arXiv paper", Date: "2026-10-27T09:00:00Z"},
	}); err != nil {
		t.Fatalf("SaveEmails(other): %v", err)
	}

	svc := NewTagService(&ConfigService{})
	arxiv, err := svc.SaveMailTag(MailTag{
		Label:    "arXiv",
		Color:    "#4f83cc",
		Keywords: []string{"arxiv"},
		Fields:   []string{tagFieldSubject},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SaveMailTag(arXiv): %v", err)
	}
	kdd, err := svc.SaveMailTag(MailTag{
		Label:    "KDD",
		Color:    "#e06c75",
		Keywords: []string{"kdd"},
		Fields:   []string{tagFieldSubject},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SaveMailTag(KDD): %v", err)
	}

	request := EmailTagFilterRequest{
		Scopes:   []EmailTagFilterScope{{AccountID: "acct", FolderName: "INBOX"}},
		TagIDs:   []string{arxiv.ID, kdd.ID},
		Page:     1,
		PageSize: 2,
	}
	page1, err := svc.ListTagFilteredEmails(request)
	if err != nil {
		t.Fatalf("ListTagFilteredEmails(page 1): %v", err)
	}
	if page1.Total != 2 || len(page1.Emails) != 2 {
		t.Fatalf("page 1 = total %d, rows %d; want total 2, rows 2", page1.Total, len(page1.Emails))
	}
	if page1.Emails[0].UID != 1 || page1.Emails[1].UID != 2 {
		t.Fatalf("page 1 UIDs = %v, want [1 2]", []uint32{page1.Emails[0].UID, page1.Emails[1].UID})
	}
	if len(page1.Emails[0].TagIDs) != 1 || page1.Emails[0].TagIDs[0] != arxiv.ID {
		t.Fatalf("arXiv row tag IDs = %v, want [%s]", page1.Emails[0].TagIDs, arxiv.ID)
	}

	request.Page = 2
	request.PageSize = 1
	page2, err := svc.ListTagFilteredEmails(request)
	if err != nil {
		t.Fatalf("ListTagFilteredEmails(page 2): %v", err)
	}
	if page2.Total != 2 || len(page2.Emails) != 1 || page2.Emails[0].UID != 2 {
		t.Fatalf("page 2 = total %d, rows %+v; want total 2 and UID 2", page2.Total, page2.Emails)
	}
}

func TestListTagFilteredEmailsCanLimitStarredMessages(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	db := NewDBManager()
	if err := db.SaveEmails("acct", "INBOX", []EmailSummary{
		{UID: 1, Subject: "Starred arXiv", Flags: []string{`\Flagged`}},
		{UID: 2, Subject: "Unstarred arXiv"},
	}); err != nil {
		t.Fatalf("SaveEmails: %v", err)
	}

	svc := NewTagService(&ConfigService{})
	tag, err := svc.SaveMailTag(MailTag{
		Label:    "arXiv",
		Color:    "#4f83cc",
		Keywords: []string{"arxiv"},
		Fields:   []string{tagFieldSubject},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("SaveMailTag: %v", err)
	}
	resp, err := svc.ListTagFilteredEmails(EmailTagFilterRequest{
		TagIDs:      []string{tag.ID},
		FlaggedOnly: true,
		Page:        1,
		PageSize:    20,
	})
	if err != nil {
		t.Fatalf("ListTagFilteredEmails: %v", err)
	}
	if resp.Total != 1 || len(resp.Emails) != 1 || resp.Emails[0].UID != 1 {
		t.Fatalf("starred filter returned total %d, rows %+v; want only UID 1", resp.Total, resp.Emails)
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

func TestParseTagSuggestionsRepairsMissingClosingBrace(t *testing.T) {
	got, err := parseTagSuggestions(
		`[{"label":"AAAI","color":"#8e6bbf","keywords":["aaai 2027"],"fields":["subject"]]`,
	)
	if err != nil {
		t.Fatalf("parseTagSuggestions: %v", err)
	}
	if len(got) != 1 || got[0].Label != "AAAI" {
		t.Fatalf("unexpected repaired suggestions: %#v", got)
	}
}

func TestParseTagSuggestionsRepairsTrailingCommas(t *testing.T) {
	got, err := parseTagSuggestions(
		`[{"label":"KDD","color":"#2f7d6d","keywords":["kdd"],"fields":["subject"],},]`,
	)
	if err != nil {
		t.Fatalf("parseTagSuggestions: %v", err)
	}
	if len(got) != 1 || got[0].Label != "KDD" {
		t.Fatalf("unexpected repaired suggestions: %#v", got)
	}
}

func TestParseTagSuggestionsStillRejectsBrokenStructure(t *testing.T) {
	if _, err := parseTagSuggestions(
		`[{"label":"AAAI","keywords":["aaai"],"fields":["subject"]},` +
			`{"label":"KDD","keywords":["kdd"],"fields":]`,
	); err == nil {
		t.Fatal("expected malformed JSON to remain rejected")
	}
}

func TestParseTagSuggestionsRejectsEmptyResponse(t *testing.T) {
	if _, err := parseTagSuggestions("   "); err == nil ||
		!strings.Contains(err.Error(), "empty response") {
		t.Fatalf("expected an actionable empty-response error, got %v", err)
	}
}

func TestTagSuggestionOutputTokensLeavesRoomForReasoning(t *testing.T) {
	if got := tagSuggestionOutputTokens(LLMConfig{MaxOutputTokens: 4096}); got != 8192 {
		t.Fatalf("tag suggestions got %d output tokens, want the 8192 reasoning floor", got)
	}
	if got := tagSuggestionOutputTokens(LLMConfig{MaxOutputTokens: 12000}); got != 12000 {
		t.Fatalf("tag suggestions got %d output tokens, want the configured 12000", got)
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

	request.PromptOverride = "Edited prompt\nwith a preserved line break"
	if _, err := svc.GenerateTagSuggestions(request); err != nil {
		t.Fatalf("GenerateTagSuggestions with override: %v", err)
	}
	if sentPrompt != request.PromptOverride {
		t.Fatalf("prompt override was not sent verbatim\ngot:\n%s\nwant:\n%s", sentPrompt, request.PromptOverride)
	}
}

func TestTagSuggestionPreviewDoesNotRequireLLM(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

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

	svc := NewTagService(&ConfigService{})
	request := TagSuggestionRequest{
		AccountIDs:  []string{"acct"},
		SampleLimit: 20,
		Fields:      []string{"subject"},
	}
	preview, err := svc.PreviewTagSuggestionPrompt(request)
	if err != nil {
		t.Fatalf("PreviewTagSuggestionPrompt without an LLM: %v", err)
	}
	if preview.SampleCount != 1 || !strings.Contains(preview.Prompt, "New arXiv paper") {
		t.Fatalf("preview did not include the cached sample: %+v", preview)
	}
	if preview.LLMName != "" || preview.LLMModel != "" {
		t.Fatalf("an absent LLM must not be reported as configured: %+v", preview)
	}
	if _, err := svc.GenerateTagSuggestions(request); err == nil {
		t.Fatal("generation must still require an LLM configuration")
	}
}

func TestBuildTagSuggestionSamplesDeduplicatesNormalizedSubjects(t *testing.T) {
	emails := []CachedEmail{
		{
			AccountID:  "acct",
			FolderName: "INBOX",
			UID:        1,
			MessageID:  "<aaai-old>",
			Subject:    "AAAI 2027 author responses now open",
			FromAddr:   "first@example.com",
			Date:       "2026-10-01T10:00:00Z",
		},
		{
			AccountID:  "acct",
			FolderName: "INBOX",
			UID:        2,
			MessageID:  "<aaai-new>",
			Subject:    "Re:   AAAI 2027 AUTHOR responses now open",
			FromAddr:   "second@example.com",
			Date:       "2026-10-02T10:00:00Z",
		},
		{
			AccountID:  "acct",
			FolderName: "INBOX",
			UID:        3,
			MessageID:  "<kdd>",
			Subject:    "KDD 2027 call for papers",
			FromAddr:   "third@example.com",
			Date:       "2026-10-03T10:00:00Z",
		},
		{
			AccountID:  "acct",
			FolderName: "INBOX",
			UID:        4,
			MessageID:  "<empty-one>",
			Date:       "2026-10-04T10:00:00Z",
		},
		{
			AccountID:  "acct",
			FolderName: "INBOX",
			UID:        5,
			MessageID:  "<empty-two>",
			Date:       "2026-10-05T10:00:00Z",
		},
		{
			AccountID:  "acct",
			FolderName: "INBOX",
			UID:        6,
			MessageID:  "<aaai-cn>",
			Subject:    "回复： AAAI 2027 author responses now open",
			Date:       "2026-09-30T10:00:00Z",
		},
	}

	samples := buildTagSuggestionSamples(
		emails,
		nil,
		map[string]bool{"acct": true},
		[]string{"subject", "from_address"},
		10,
	)
	if len(samples) != 4 {
		t.Fatalf("got %d samples, want 4 after subject deduplication: %+v", len(samples), samples)
	}
	subjects := make(map[string]int)
	for _, sample := range samples {
		subjects[tagSuggestionSubjectKey(sample.Subject)]++
	}
	if subjects["aaai 2027 author responses now open"] != 1 {
		t.Fatalf("normalized AAAI subject was not deduplicated: %+v", samples)
	}
	if subjects[""] != 2 {
		t.Fatalf("empty subjects must remain separate: %+v", samples)
	}
}

func TestBuildTagSuggestionSamplesAppliesLimitAfterSubjectDeduplication(t *testing.T) {
	emails := []CachedEmail{
		{
			AccountID:  "acct",
			FolderName: "INBOX",
			UID:        1,
			MessageID:  "<duplicate-old>",
			Subject:    "Weekly digest",
			Date:       "2026-10-01T10:00:00Z",
		},
		{
			AccountID:  "acct",
			FolderName: "INBOX",
			UID:        2,
			MessageID:  "<duplicate-new>",
			Subject:    "Fwd: Weekly digest",
			Date:       "2026-10-02T10:00:00Z",
		},
		{
			AccountID:  "acct",
			FolderName: "INBOX",
			UID:        3,
			MessageID:  "<unique>",
			Subject:    "Scholarship notice",
			Date:       "2026-09-30T10:00:00Z",
		},
	}

	samples := buildTagSuggestionSamples(
		emails,
		nil,
		map[string]bool{"acct": true},
		[]string{"subject"},
		2,
	)
	if len(samples) != 2 {
		t.Fatalf("got %d samples, want 2 unique subjects: %+v", len(samples), samples)
	}
	if samples[0].Subject != "Fwd: Weekly digest" || samples[1].Subject != "Scholarship notice" {
		t.Fatalf("unexpected sample order/content: %+v", samples)
	}
}
