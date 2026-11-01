package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	tagFieldSubject = "subject"
	tagFieldFrom    = "from"
	tagFieldDomain  = "domain"
	tagFieldBody    = "body"

	defaultTagColor   = "#4f83cc"
	maxTagKeywords    = 50
	maxTagSuggestions = 30
)

var (
	tagSuggestionSubjectPrefixRE = regexp.MustCompile(
		`(?i)^\s*(?:(?:re|fwd?|aw|sv)\s*(?:\[\d+\])?\s*[:：]\s*|(?:回复|转发)\s*[:：]\s*)+`,
	)
	allTagFields = []string{tagFieldSubject, tagFieldFrom, tagFieldDomain, tagFieldBody}
	aiTagFields  = []string{
		tagFieldSubject,
		"from_name",
		"from_address",
		tagFieldDomain,
		"date",
		"body_excerpt",
	}
)

// MailTag is a local classification rule exposed to the frontend. A message can
// match any number of tags; tags are labels, never IMAP folders.
type MailTag struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Color     string   `json:"color"`
	Keywords  []string `json:"keywords"`
	Fields    []string `json:"fields"`
	Enabled   bool     `json:"enabled"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// EmailTagState is the effective tag set for one message after applying manual
// overrides to the automatic rule matches.
type EmailTagState struct {
	SelectedTagIDs  []string `json:"selected_tag_ids"`
	AutomaticTagIDs []string `json:"automatic_tag_ids"`
}

// TagSuggestionRequest controls the exact data sent to the LLM. Subject lines
// are always included; every other field is opt-in. PromptOverride lets the user
// edit the reviewed preview before sending it verbatim.
type TagSuggestionRequest struct {
	AccountIDs     []string `json:"account_ids"`
	SampleLimit    int      `json:"sample_limit"`
	Fields         []string `json:"fields"`
	LLMConfigName  string   `json:"llm_config_name"`
	LLMModel       string   `json:"llm_model"`
	PromptOverride string   `json:"prompt_override,omitempty"`
}

// TagSuggestionPreview is shown before any network request. Prompt is the
// generated string sent when PromptOverride is empty.
type TagSuggestionPreview struct {
	Prompt       string   `json:"prompt"`
	AccountNames []string `json:"account_names"`
	Fields       []string `json:"fields"`
	SampleCount  int      `json:"sample_count"`
	SampleFrom   string   `json:"sample_from"`
	SampleTo     string   `json:"sample_to"`
	LLMName      string   `json:"llm_name"`
	LLMModel     string   `json:"llm_model"`
}

type TagSuggestion struct {
	Label    string   `json:"label"`
	Color    string   `json:"color"`
	Keywords []string `json:"keywords"`
	Fields   []string `json:"fields"`
}

type preparedTagSuggestion struct {
	config  LLMConfig
	preview TagSuggestionPreview
}

// TagService owns tag CRUD, local classification and the privacy-reviewed AI
// rule suggestion flow.
type TagService struct {
	db  *DBManager
	cfg *ConfigService
}

func NewTagService(cfg *ConfigService) *TagService {
	if cfg == nil {
		cfg = &ConfigService{}
	}
	return &TagService{db: NewDBManager(), cfg: cfg}
}

func (s *TagService) ListMailTags() ([]MailTag, error) {
	cached, err := s.db.LoadMailTags()
	if err != nil {
		return nil, fmt.Errorf("load mail tags: %w", err)
	}
	tags := make([]MailTag, 0, len(cached))
	for _, row := range cached {
		tag, err := decodeMailTag(row)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func (s *TagService) SaveMailTag(tag MailTag) (MailTag, error) {
	normalized, err := normalizeMailTag(tag)
	if err != nil {
		return MailTag{}, err
	}
	existing, err := s.ListMailTags()
	if err != nil {
		return MailTag{}, err
	}
	for _, current := range existing {
		if current.ID != normalized.ID &&
			strings.EqualFold(strings.TrimSpace(current.Label), normalized.Label) {
			return MailTag{}, fmt.Errorf("a tag named %q already exists", normalized.Label)
		}
	}
	if normalized.ID == "" {
		normalized.ID, err = newTagID()
		if err != nil {
			return MailTag{}, fmt.Errorf("generate tag ID: %w", err)
		}
	}

	keywordsJSON, _ := json.Marshal(normalized.Keywords)
	fieldsJSON, _ := json.Marshal(normalized.Fields)
	row := CachedMailTag{
		ID:       normalized.ID,
		Label:    normalized.Label,
		Color:    normalized.Color,
		Keywords: string(keywordsJSON),
		Fields:   string(fieldsJSON),
		Enabled:  normalized.Enabled,
	}
	if err := s.db.SaveMailTag(&row); err != nil {
		return MailTag{}, fmt.Errorf("save mail tag: %w", err)
	}
	return decodeMailTag(row)
}

func (s *TagService) DeleteMailTag(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("tag ID is required")
	}
	if err := s.db.DeleteMailTag(id); err != nil {
		return fmt.Errorf("delete mail tag: %w", err)
	}
	return nil
}

// ListClassifiedEmails returns one page of messages matching any selected tag
// (OR semantics). Gmail's INBOX/Important/All Mail copies are collapsed to one
// row before filtering and paging.
func (s *TagService) ListClassifiedEmails(tagIDs []string, page, pageSize int) (*EmailPageResponse, error) {
	tags, err := s.ListMailTags()
	if err != nil {
		return nil, err
	}
	selected := make(map[string]bool)
	enabled := make(map[string]bool)
	for _, tag := range tags {
		if tag.Enabled {
			enabled[tag.ID] = true
		}
	}
	for _, id := range tagIDs {
		if enabled[id] {
			selected[id] = true
		}
	}
	if len(selected) == 0 {
		return &EmailPageResponse{Emails: []EmailSummary{}}, nil
	}

	emails, err := s.db.LoadTaggingEmails()
	if err != nil {
		return nil, fmt.Errorf("load emails for classification: %w", err)
	}
	bodies, err := s.db.LoadTaggingBodies()
	if err != nil {
		return nil, fmt.Errorf("load bodies for classification: %w", err)
	}
	overrides, err := s.db.LoadEmailTagOverrides()
	if err != nil {
		return nil, fmt.Errorf("load tag overrides: %w", err)
	}
	folderAttrs, err := s.loadFolderAttrs()
	if err != nil {
		return nil, err
	}

	classified := classifyCachedEmails(emails, bodies, tags, overrides, folderAttrs, selected, time.Now())
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return &EmailPageResponse{
		Emails: paginateSummaries(classified, page, pageSize),
		Total:  len(classified),
	}, nil
}

func (s *TagService) GetEmailTagState(
	accountID, folder string,
	uid uint32,
) (EmailTagState, error) {
	email, err := s.db.LoadCachedEmail(accountID, folder, uid)
	if err != nil {
		return EmailTagState{}, fmt.Errorf("load email: %w", err)
	}
	tags, err := s.ListMailTags()
	if err != nil {
		return EmailTagState{}, err
	}
	body, _ := s.loadTaggingBody(accountID, folder, uid)
	overrides, err := s.db.LoadEmailTagOverrides()
	if err != nil {
		return EmailTagState{}, fmt.Errorf("load tag overrides: %w", err)
	}
	return emailTagState(*email, body, tags, overrides), nil
}

// SetEmailTags stores only the differences between the selected tags and the
// rule-based result. No row means "follow the rules", so editing the rules later
// still updates this message.
func (s *TagService) SetEmailTags(
	accountID, folder string,
	uid uint32,
	selectedTagIDs []string,
) error {
	email, err := s.db.LoadCachedEmail(accountID, folder, uid)
	if err != nil {
		return fmt.Errorf("load email: %w", err)
	}
	tags, err := s.ListMailTags()
	if err != nil {
		return err
	}
	body, _ := s.loadTaggingBody(accountID, folder, uid)

	tagByID := make(map[string]MailTag, len(tags))
	for _, tag := range tags {
		tagByID[tag.ID] = tag
	}
	selected := make(map[string]bool)
	for _, id := range selectedTagIDs {
		if _, ok := tagByID[id]; ok {
			selected[id] = true
		}
	}

	summary := cachedEmailSummary(*email, time.Now())
	overrides := make([]EmailTagOverride, 0)
	for _, tag := range tags {
		automatic := tag.Enabled && tagMatches(tag, summary, body)
		if automatic == selected[tag.ID] {
			continue
		}
		overrides = append(overrides, EmailTagOverride{
			AccountID:  accountID,
			FolderName: folder,
			UID:        uid,
			TagID:      tag.ID,
			Included:   selected[tag.ID],
		})
	}
	if err := s.db.ReplaceEmailTagOverrides(accountID, folder, uid, overrides); err != nil {
		return err
	}
	return nil
}

// PreviewTagSuggestionPrompt builds the privacy-reviewed request without making
// any network call. The LLM is selected only after this preview, so an absent or
// stale LLM configuration must not block prompt generation.
func (s *TagService) PreviewTagSuggestionPrompt(
	request TagSuggestionRequest,
) (TagSuggestionPreview, error) {
	prepared, err := s.prepareTagSuggestion(request, false)
	if err != nil {
		return TagSuggestionPreview{}, err
	}
	return prepared.preview, nil
}

// GenerateTagSuggestions sends PromptOverride when present, otherwise the exact
// prompt returned by PreviewTagSuggestionPrompt for the same request. AI is
// never called by local classification or mail sync.
func (s *TagService) GenerateTagSuggestions(
	request TagSuggestionRequest,
) ([]TagSuggestion, error) {
	prepared, err := s.prepareTagSuggestion(request, true)
	if err != nil {
		return nil, err
	}
	prompt := prepared.preview.Prompt
	if strings.TrimSpace(request.PromptOverride) != "" {
		prompt = request.PromptOverride
	}
	output, err := requestLLMChat(
		&prepared.config,
		prompt,
		prepared.config.MaxOutputTokens,
	)
	if err != nil {
		return nil, err
	}
	return parseTagSuggestions(output)
}

func (s *TagService) loadFolderAttrs() (map[string][]string, error) {
	cfg, err := s.cfg.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("load accounts for tag folders: %w", err)
	}
	attrs := make(map[string][]string)
	for _, account := range cfg.EmailAccounts {
		folders, err := s.db.LoadFolders(account.ID)
		if err != nil {
			continue
		}
		for _, folder := range folders {
			attrs[tagFolderKey(account.ID, folder.Name)] = folder.Attrs
		}
	}
	return attrs, nil
}

func (s *TagService) loadTaggingBody(accountID, folder string, uid uint32) (string, bool) {
	email, err := s.db.LoadCachedEmail(accountID, folder, uid)
	if err != nil {
		return "", false
	}
	detail, err := s.db.LoadEmailBody(accountID, folder, uid)
	if err == nil && detail != nil {
		return detail.BodyText, true
	}

	// A Gmail message can have its cached body under another folder view. A body
	// match in any copy is still a match for the canonical row.
	if email.MessageID != "" {
		all, err := s.db.LoadTaggingBodies()
		if err == nil {
			for _, body := range all {
				if body.AccountID == accountID &&
					strings.EqualFold(strings.TrimSpace(body.MessageID), strings.TrimSpace(email.MessageID)) {
					return body.BodyText, true
				}
			}
		}
	}
	return "", false
}

func (s *TagService) prepareTagSuggestion(
	request TagSuggestionRequest,
	requireLLM bool,
) (preparedTagSuggestion, error) {
	cfg, err := s.cfg.GetConfig()
	if err != nil {
		return preparedTagSuggestion{}, fmt.Errorf("load LLM configuration: %w", err)
	}
	llmCfg, err := findLLMConfig(cfg.LLMConfigs, request.LLMConfigName, request.LLMModel)
	if err != nil {
		if requireLLM {
			return preparedTagSuggestion{}, err
		}
		llmCfg = LLMConfig{}
	}

	emails, err := s.db.LoadTaggingEmails()
	if err != nil {
		return preparedTagSuggestion{}, fmt.Errorf("load cached mail: %w", err)
	}
	bodies, err := s.db.LoadTaggingBodies()
	if err != nil {
		return preparedTagSuggestion{}, fmt.Errorf("load cached mail bodies: %w", err)
	}
	accountIDs := make(map[string]bool)
	for _, id := range request.AccountIDs {
		if strings.TrimSpace(id) != "" {
			accountIDs[id] = true
		}
	}
	accountNames := make([]string, 0)
	for _, account := range cfg.EmailAccounts {
		if len(accountIDs) == 0 || accountIDs[account.ID] {
			accountNames = append(accountNames, account.Name+" <"+account.Email+">")
		}
	}

	fields := normalizeSuggestionFields(request.Fields)
	samples := buildTagSuggestionSamples(
		emails,
		bodies,
		accountIDs,
		fields,
		request.SampleLimit,
	)
	prompt, from, to := buildTagSuggestionPrompt(samples, fields)
	return preparedTagSuggestion{
		config: llmCfg,
		preview: TagSuggestionPreview{
			Prompt:       prompt,
			AccountNames: accountNames,
			Fields:       fields,
			SampleCount:  len(samples),
			SampleFrom:   from,
			SampleTo:     to,
			LLMName:      llmCfg.Name,
			LLMModel:     llmCfg.Model,
		},
	}, nil
}

func findLLMConfig(configs []LLMConfig, name, model string) (LLMConfig, error) {
	if len(configs) == 0 {
		return LLMConfig{}, fmt.Errorf("no LLM configuration is available")
	}
	index := 0
	if strings.TrimSpace(name) != "" {
		index = -1
		for i, cfg := range configs {
			if cfg.Name == name {
				index = i
				break
			}
		}
		if index < 0 {
			return LLMConfig{}, fmt.Errorf("LLM configuration %q was not found", name)
		}
	}
	cfg := configs[index]
	if strings.TrimSpace(model) != "" {
		cfg.Model = strings.TrimSpace(model)
	}
	if cfg.Model == "" && len(cfg.Models) > 0 {
		cfg.Model = cfg.Models[0]
	}
	if cfg.Model == "" {
		return LLMConfig{}, fmt.Errorf("the selected LLM configuration has no model")
	}
	return cfg, nil
}

func normalizeSuggestionFields(fields []string) []string {
	allowed := make(map[string]bool, len(aiTagFields))
	for _, field := range aiTagFields {
		allowed[field] = true
	}
	selected := map[string]bool{tagFieldSubject: true}
	for _, field := range fields {
		field = strings.ToLower(strings.TrimSpace(field))
		if allowed[field] && field != tagFieldSubject {
			selected[field] = true
		}
	}
	out := make([]string, 0, len(selected))
	for _, field := range aiTagFields {
		if selected[field] {
			out = append(out, field)
		}
	}
	return out
}

type tagSuggestionSample struct {
	Date       string
	Subject    string
	FromName   string
	FromAddr   string
	Domain     string
	Body       string
	MessageKey string
}

func buildTagSuggestionSamples(
	emails []CachedEmail,
	bodies []CachedEmailBody,
	accountIDs map[string]bool,
	fields []string,
	limit int,
) []tagSuggestionSample {
	if limit <= 0 {
		limit = 120
	}
	if limit > 300 {
		limit = 300
	}
	include := make(map[string]bool, len(fields))
	for _, field := range fields {
		include[field] = true
	}
	bodyByFolderKey := make(map[string]string, len(bodies))
	bodyByMessageID := make(map[string]string, len(bodies))
	for _, body := range bodies {
		bodyByFolderKey[emailCacheKey(body.AccountID, body.FolderName, body.UID)] = body.BodyText
		if body.MessageID != "" {
			bodyByMessageID[strings.ToLower(strings.TrimSpace(body.MessageID))] = body.BodyText
		}
	}

	summaries := make([]EmailSummary, 0, len(emails))
	byKey := make(map[string]CachedEmail)
	for _, email := range emails {
		if len(accountIDs) > 0 && !accountIDs[email.AccountID] {
			continue
		}
		summary := cachedEmailSummary(email, time.Now())
		summary.AccountID = email.AccountID
		summary.FolderName = email.FolderName
		summaries = append(summaries, summary)
		key := tagMessageKey(email)
		if current, ok := byKey[key]; !ok || tagFolderPreference(email) > tagFolderPreference(current) {
			byKey[key] = email
		}
	}
	sortEmailsNewestFirst(summaries)

	out := make([]tagSuggestionSample, 0, limit)
	seenMessages := make(map[string]bool)
	seenSubjects := make(map[string]bool)
	for _, summary := range summaries {
		if len(out) >= limit {
			break
		}
		email := byKey[tagMessageKeyFromSummary(summary)]
		key := tagMessageKey(email)
		if seenMessages[key] {
			continue
		}
		subjectKey := tagSuggestionSubjectKey(summary.Subject)
		if subjectKey != "" && seenSubjects[subjectKey] {
			continue
		}
		seenMessages[key] = true
		if subjectKey != "" {
			seenSubjects[subjectKey] = true
		}
		sample := tagSuggestionSample{
			Date:       summary.Date,
			Subject:    summary.Subject,
			MessageKey: key,
		}
		if include["from_name"] {
			sample.FromName = summary.FromName
		}
		if include["from_address"] {
			sample.FromAddr = summary.FromAddr
		}
		if include[tagFieldDomain] {
			sample.Domain = emailDomain(summary.FromAddr)
		}
		if include["body_excerpt"] {
			body := bodyByFolderKey[emailCacheKey(email.AccountID, email.FolderName, email.UID)]
			if body == "" && email.MessageID != "" {
				body = bodyByMessageID[strings.ToLower(strings.TrimSpace(email.MessageID))]
			}
			sample.Body = boundedBodyExcerpt(body, 500)
		}
		if !include["date"] {
			sample.Date = ""
		}
		out = append(out, sample)
	}
	return out
}

// tagSuggestionSubjectKey collapses repeated mail into one prompt example.
// Reply/forward prefixes and formatting differences do not add useful signal for
// category discovery. Empty subjects stay separate because unrelated messages
// commonly omit the subject entirely.
func tagSuggestionSubjectKey(subject string) string {
	normalized := strings.Join(strings.Fields(subject), " ")
	normalized = tagSuggestionSubjectPrefixRE.ReplaceAllString(normalized, "")
	normalized = strings.Join(strings.Fields(normalized), " ")
	return strings.ToLower(normalized)
}

func buildTagSuggestionPrompt(
	samples []tagSuggestionSample,
	fields []string,
) (string, string, string) {
	lines := make([]map[string]string, 0, len(samples))
	from, to := "", ""
	for _, sample := range samples {
		line := map[string]string{"subject": sample.Subject}
		if sample.FromName != "" {
			line["from_name"] = sample.FromName
		}
		if sample.FromAddr != "" {
			line["from_address"] = sample.FromAddr
		}
		if sample.Domain != "" {
			line["domain"] = sample.Domain
		}
		if sample.Date != "" {
			line["date"] = sample.Date
			if from == "" || sample.Date < from {
				from = sample.Date
			}
			if to == "" || sample.Date > to {
				to = sample.Date
			}
		}
		if sample.Body != "" {
			line["body_excerpt"] = sample.Body
		}
		lines = append(lines, line)
	}
	payload, _ := json.MarshalIndent(lines, "", "  ")

	prompt := fmt.Sprintf(`You create conservative, reusable email classification rules.

Return ONLY a JSON array. Each item must have:
- "label": a short human-readable category name
- "color": a hex color such as "#4f83cc"
- "keywords": 1-12 lower-case literal substrings
- "fields": one or more of "subject", "from", "domain", "body"

Rules:
- Prefer categories a person would actually browse (for example arXiv, AAAI, KDD, cloud billing, university notices).
- Use literal substrings that are specific enough to avoid accidental matches.
- Do not create a category for one individual message unless the pattern is clearly recurring.
- Do not infer or repeat sensitive personal facts beyond the supplied samples.
- Return at most 12 categories and no Markdown fences or explanations.

The user explicitly selected these fields: %s.
Email samples:
%s`, strings.Join(fields, ", "), string(payload))
	return prompt, from, to
}

// boundedBodyExcerpt keeps an AI sample small and strips collapsed whitespace so
// prompt previews remain readable. It only uses an already cached body.
func boundedBodyExcerpt(body string, limit int) string {
	body = strings.Join(strings.Fields(body), " ")
	if limit <= 0 || len([]rune(body)) <= limit {
		return body
	}
	return string([]rune(body)[:limit]) + "…"
}

func parseTagSuggestions(output string) ([]TagSuggestion, error) {
	raw := strings.TrimSpace(output)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```JSON")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("the LLM response did not contain a JSON array")
	}
	var suggestions []TagSuggestion
	if err := json.Unmarshal([]byte(raw[start:end+1]), &suggestions); err != nil {
		return nil, fmt.Errorf("parse LLM tag suggestions: %w", err)
	}

	out := make([]TagSuggestion, 0, len(suggestions))
	seen := make(map[string]bool)
	for _, suggestion := range suggestions {
		label := strings.TrimSpace(suggestion.Label)
		if label == "" || seen[strings.ToLower(label)] {
			continue
		}
		keywords := normalizeKeywords(suggestion.Keywords)
		fields := normalizeTagFields(suggestion.Fields)
		if len(keywords) == 0 || len(fields) == 0 {
			continue
		}
		color := strings.ToLower(strings.TrimSpace(suggestion.Color))
		if !validTagColor(color) {
			color = defaultTagColor
		}
		seen[strings.ToLower(label)] = true
		out = append(out, TagSuggestion{
			Label:    label,
			Color:    color,
			Keywords: keywords,
			Fields:   fields,
		})
		if len(out) >= maxTagSuggestions {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("the LLM returned no usable tag rules")
	}
	return out, nil
}

func normalizeMailTag(tag MailTag) (MailTag, error) {
	tag.Label = strings.TrimSpace(tag.Label)
	if tag.Label == "" {
		return MailTag{}, fmt.Errorf("tag label is required")
	}
	if len([]rune(tag.Label)) > 64 {
		return MailTag{}, fmt.Errorf("tag label is too long")
	}
	tag.Color = strings.ToLower(strings.TrimSpace(tag.Color))
	if !validTagColor(tag.Color) {
		tag.Color = defaultTagColor
	}
	tag.Keywords = normalizeKeywords(tag.Keywords)
	if len(tag.Keywords) == 0 {
		return MailTag{}, fmt.Errorf("add at least one keyword")
	}
	tag.Fields = normalizeTagFields(tag.Fields)
	if len(tag.Fields) == 0 {
		return MailTag{}, fmt.Errorf("select at least one matching field")
	}
	return tag, nil
}

func normalizeKeywords(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		if len([]rune(value)) > 160 {
			value = string([]rune(value)[:160])
		}
		seen[value] = true
		out = append(out, value)
		if len(out) >= maxTagKeywords {
			break
		}
	}
	return out
}

func normalizeTagFields(values []string) []string {
	allowed := make(map[string]bool, len(allTagFields))
	for _, field := range allTagFields {
		allowed[field] = true
	}
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if allowed[value] {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for _, field := range allTagFields {
		if seen[field] {
			out = append(out, field)
		}
	}
	return out
}

func validTagColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func decodeMailTag(row CachedMailTag) (MailTag, error) {
	var keywords, fields []string
	if err := json.Unmarshal([]byte(row.Keywords), &keywords); err != nil {
		return MailTag{}, fmt.Errorf("decode keywords for tag %q: %w", row.Label, err)
	}
	if err := json.Unmarshal([]byte(row.Fields), &fields); err != nil {
		return MailTag{}, fmt.Errorf("decode fields for tag %q: %w", row.Label, err)
	}
	return MailTag{
		ID:        row.ID,
		Label:     row.Label,
		Color:     row.Color,
		Keywords:  keywords,
		Fields:    fields,
		Enabled:   row.Enabled,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func newTagID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func tagMatches(tag MailTag, summary EmailSummary, body string) bool {
	if !tag.Enabled || len(tag.Keywords) == 0 || len(tag.Fields) == 0 {
		return false
	}
	fieldValues := make([]string, 0, len(tag.Fields))
	for _, field := range tag.Fields {
		switch field {
		case tagFieldSubject:
			fieldValues = append(fieldValues, summary.Subject)
		case tagFieldFrom:
			fieldValues = append(fieldValues, summary.From, summary.FromName, summary.FromAddr)
		case tagFieldDomain:
			fieldValues = append(fieldValues, emailDomain(summary.FromAddr))
		case tagFieldBody:
			fieldValues = append(fieldValues, body)
		}
	}
	haystack := strings.ToLower(strings.Join(fieldValues, "\n"))
	for _, keyword := range tag.Keywords {
		if keyword != "" && strings.Contains(haystack, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func emailDomain(address string) string {
	if start := strings.LastIndex(address, "<"); start >= 0 {
		if end := strings.LastIndex(address, ">"); end > start {
			address = address[start+1 : end]
		}
	}
	address = strings.TrimSpace(address)
	if at := strings.LastIndex(address, "@"); at >= 0 && at+1 < len(address) {
		return strings.ToLower(strings.Trim(address[at+1:], "> \t\r\n"))
	}
	return ""
}

type classifiedMail struct {
	summary EmailSummary
	tagIDs  map[string]bool
	score   int
}

func classifyCachedEmails(
	emails []CachedEmail,
	bodies []CachedEmailBody,
	tags []MailTag,
	overrides []EmailTagOverride,
	folderAttrs map[string][]string,
	selected map[string]bool,
	now time.Time,
) []EmailSummary {
	bodyByKey := make(map[string]string, len(bodies))
	for _, body := range bodies {
		bodyByKey[emailCacheKey(body.AccountID, body.FolderName, body.UID)] = body.BodyText
	}
	overrideByKey := make(map[string]bool, len(overrides))
	for _, override := range overrides {
		overrideByKey[tagOverrideKey(
			override.AccountID,
			override.FolderName,
			override.UID,
			override.TagID,
		)] = override.Included
	}
	tagOrder := make([]string, 0, len(tags))
	for _, tag := range tags {
		tagOrder = append(tagOrder, tag.ID)
	}

	groups := make(map[string]*classifiedMail)
	groupOrder := make([]string, 0, len(emails))
	for _, email := range emails {
		summary := cachedEmailSummary(email, now)
		summary.AccountID = email.AccountID
		summary.FolderName = email.FolderName
		body := bodyByKey[emailCacheKey(email.AccountID, email.FolderName, email.UID)]
		key := tagMessageKey(email)

		matches := make(map[string]bool)
		for _, tag := range tags {
			matched := tagMatches(tag, summary, body)
			overrideKey := tagOverrideKey(email.AccountID, email.FolderName, email.UID, tag.ID)
			if included, ok := overrideByKey[overrideKey]; ok {
				matched = included
			}
			if matched && tag.Enabled {
				matches[tag.ID] = true
			}
		}
		if len(matches) == 0 {
			continue
		}

		score := tagFolderScore(email.AccountID, email.FolderName, folderAttrs)
		current, ok := groups[key]
		if !ok {
			groups[key] = &classifiedMail{
				summary: summary,
				tagIDs:  matches,
				score:   score,
			}
			groupOrder = append(groupOrder, key)
			continue
		}
		for tagID := range matches {
			current.tagIDs[tagID] = true
		}
		if score > current.score {
			current.summary = summary
			current.score = score
		}
	}

	out := make([]EmailSummary, 0, len(groups))
	for _, key := range groupOrder {
		group := groups[key]
		included := false
		group.summary.TagIDs = group.summary.TagIDs[:0]
		for _, tagID := range tagOrder {
			if group.tagIDs[tagID] {
				group.summary.TagIDs = append(group.summary.TagIDs, tagID)
				if selected[tagID] {
					included = true
				}
			}
		}
		if included {
			out = append(out, group.summary)
		}
	}
	sortEmailsNewestFirst(out)
	return out
}

func emailTagState(
	email CachedEmail,
	body string,
	tags []MailTag,
	overrides []EmailTagOverride,
) EmailTagState {
	summary := cachedEmailSummary(email, time.Now())
	automatic := make([]string, 0)
	selected := make([]string, 0)
	overrideByTag := make(map[string]bool)
	for _, override := range overrides {
		if override.AccountID == email.AccountID &&
			override.FolderName == email.FolderName &&
			override.UID == email.UID {
			overrideByTag[override.TagID] = override.Included
		}
	}
	for _, tag := range tags {
		matched := tag.Enabled && tagMatches(tag, summary, body)
		if matched {
			automatic = append(automatic, tag.ID)
		}
		if included, ok := overrideByTag[tag.ID]; ok {
			matched = included
		}
		if matched {
			selected = append(selected, tag.ID)
		}
	}
	return EmailTagState{SelectedTagIDs: selected, AutomaticTagIDs: automatic}
}

func paginateSummaries(emails []EmailSummary, page, pageSize int) []EmailSummary {
	start := (page - 1) * pageSize
	if start >= len(emails) {
		return []EmailSummary{}
	}
	end := start + pageSize
	if end > len(emails) {
		end = len(emails)
	}
	return emails[start:end]
}

func tagMessageKey(email CachedEmail) string {
	if messageID := strings.ToLower(strings.TrimSpace(email.MessageID)); messageID != "" {
		return "message\x00" + email.AccountID + "\x00" + messageID
	}
	return tagFolderMessageKey(email.AccountID, email.FolderName, email.UID)
}

func tagMessageKeyFromSummary(summary EmailSummary) string {
	if messageID := strings.ToLower(strings.TrimSpace(summary.MessageID)); messageID != "" {
		return "message\x00" + summary.AccountID + "\x00" + messageID
	}
	return tagFolderMessageKey(summary.AccountID, summary.FolderName, summary.UID)
}

func tagFolderMessageKey(accountID, folder string, uid uint32) string {
	return fmt.Sprintf("folder\x00%s\x00%s\x00%d", accountID, folder, uid)
}

func tagOverrideKey(accountID, folder string, uid uint32, tagID string) string {
	return fmt.Sprintf("%s\x00%s\x00%d\x00%s", accountID, folder, uid, tagID)
}

func tagFolderKey(accountID, folder string) string {
	return accountID + "\x00" + folder
}

func tagFolderScore(accountID, folder string, attrs map[string][]string) int {
	if isInbox(folder) {
		return 3
	}
	for _, attr := range attrs[tagFolderKey(accountID, folder)] {
		if strings.EqualFold(attr, "\\All") {
			return 1
		}
	}
	return 2
}

func tagFolderPreference(email CachedEmail) int {
	if isInbox(email.FolderName) {
		return 2
	}
	return 1
}

func sortMailTags(tags []MailTag) {
	sort.SliceStable(tags, func(i, j int) bool {
		return tags[i].CreatedAt < tags[j].CreatedAt
	})
}
