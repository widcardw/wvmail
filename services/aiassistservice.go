package services

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AIAssistRequest is the frontend's complete input for one translation or
// summary. API keys and provider configuration stay on the Go side; empty
// fields mean "use the selection saved in Settings → AI 辅助".
type AIAssistRequest struct {
	Text          string
	TargetLang    string
	Engine        string
	LLMConfigName string
	LLMModel      string
}

// AIAssistService provides translation and summarisation for the message being
// read. It resolves the selected provider from config on every call, so the
// confirmation dialog, a skipped confirmation and the settings page all share
// one source of truth.
type AIAssistService struct {
	cfg   *ConfigService
	cache sync.Map // map[string]cachedAIResult
}

type cachedAIResult struct {
	text      string
	timestamp time.Time
}

func NewAIAssistService(cfg *ConfigService) *AIAssistService {
	return &AIAssistService{cfg: cfg}
}

// TranslateText translates text using the configured LLM or DeepL account.
func (s *AIAssistService) TranslateText(request AIAssistRequest) (string, error) {
	if strings.TrimSpace(request.Text) == "" {
		return "", fmt.Errorf("no email content to translate")
	}
	cfg, err := s.loadConfig()
	if err != nil {
		return "", err
	}

	targetLang := firstNonEmpty(
		request.TargetLang,
		cfg.TranslationConfig.DefaultTargetLang,
		"zh-CN",
	)
	engine := strings.ToLower(firstNonEmpty(
		request.Engine,
		cfg.TranslationConfig.DefaultEngine,
		"llm",
	))

	var llmCfg *LLMConfig
	provider := engine
	switch engine {
	case "llm":
		resolved, err := resolveAIAssistLLM(
			cfg,
			request.LLMConfigName,
			request.LLMModel,
		)
		if err != nil {
			return "", err
		}
		llmCfg = &resolved
		provider = resolved.Provider
	case "deepl":
		// DeepL has no model selection.
	default:
		return "", fmt.Errorf("unsupported translation engine: %q", engine)
	}

	key := s.cacheKey("translate", request.Text, targetLang, engine, provider, llmCfg)
	if cached, ok := s.getFromCache(key); ok {
		return cached, nil
	}

	var result string
	if engine == "deepl" {
		result, err = s.translateDeepL(request.Text, targetLang, cfg.TranslationConfig.DeepLAuthKey)
	} else {
		result, err = s.translateLLM(request.Text, targetLang, llmCfg)
	}
	if err != nil {
		return "", err
	}
	s.storeInCache(key, result)
	return result, nil
}

// SummarizeText turns the message into a concise overview, key points and
// action items. Summaries always use an LLM; DeepL only translates.
func (s *AIAssistService) SummarizeText(request AIAssistRequest) (string, error) {
	if strings.TrimSpace(request.Text) == "" {
		return "", fmt.Errorf("no email content to summarize")
	}
	cfg, err := s.loadConfig()
	if err != nil {
		return "", err
	}
	targetLang := firstNonEmpty(
		request.TargetLang,
		cfg.TranslationConfig.DefaultTargetLang,
		"zh-CN",
	)
	llmCfg, err := resolveAIAssistLLM(cfg, request.LLMConfigName, request.LLMModel)
	if err != nil {
		return "", err
	}

	key := s.cacheKey("summary", request.Text, targetLang, "llm", llmCfg.Provider, &llmCfg)
	if cached, ok := s.getFromCache(key); ok {
		return cached, nil
	}
	result, err := requestLLMChat(&llmCfg, summaryPrompt(request.Text, targetLang), llmCfg.MaxOutputTokens)
	if err != nil {
		return "", err
	}
	s.storeInCache(key, result)
	return result, nil
}

func (s *AIAssistService) loadConfig() (AppConfig, error) {
	if s.cfg == nil {
		return AppConfig{}, fmt.Errorf("AI assist config service is not available")
	}
	return s.cfg.GetConfig()
}

func resolveAIAssistLLM(cfg AppConfig, name, model string) (LLMConfig, error) {
	if strings.TrimSpace(name) == "" && strings.TrimSpace(model) == "" {
		name = cfg.TranslationConfig.LLMConfigName
		model = cfg.TranslationConfig.LLMModel
	}
	return findLLMConfig(cfg.LLMConfigs, name, model)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *AIAssistService) cacheKey(
	operation, text, targetLang, engine, provider string,
	llmCfg *LLMConfig,
) string {
	identity := provider
	if llmCfg != nil {
		identity = strings.Join(
			[]string{llmCfg.Provider, strings.TrimRight(llmCfg.BaseURL, "/"), llmCfg.Model},
			"|",
		)
	}
	h := sha256.Sum256([]byte(strings.Join(
		[]string{operation, text, targetLang, engine, identity},
		"\x00",
	)))
	return fmt.Sprintf("%x", h[:16])
}

func (s *AIAssistService) getFromCache(key string) (string, bool) {
	if value, ok := s.cache.Load(key); ok {
		entry := value.(cachedAIResult)
		if time.Since(entry.timestamp) < time.Hour {
			return entry.text, true
		}
		s.cache.Delete(key)
	}
	return "", false
}

func (s *AIAssistService) storeInCache(key, text string) {
	s.cache.Store(key, cachedAIResult{text: text, timestamp: time.Now()})
}

func (s *AIAssistService) translateLLM(text, targetLang string, cfg *LLMConfig) (string, error) {
	prompt := fmt.Sprintf(
		"Translate the following text to %s. Preserve any email formatting, line breaks, and "+
			"only respond with the translation, no explanations.\n\n%s",
		targetLang, text,
	)
	return requestLLMChat(cfg, prompt, cfg.MaxOutputTokens)
}

func summaryPrompt(text, targetLang string) string {
	return fmt.Sprintf(
		"Summarize the email below for a busy reader.\n"+
			"- Write the summary in %s.\n"+
			"- Start with a one-sentence overview, then list the key points, decisions, "+
			"action items, and deadlines.\n"+
			"- Preserve names, dates, amounts, and commitments exactly.\n"+
			"- Do not invent facts. If something is unclear, say so.\n"+
			"- Use concise Markdown. Do not repeat the whole email or add a preamble.\n\n"+
			"Email:\n<email>\n%s\n</email>",
		targetLang, text,
	)
}

func (s *AIAssistService) translateDeepL(text, targetLang, authKey string) (string, error) {
	if authKey == "" {
		return "", fmt.Errorf("DeepL auth key is not configured")
	}

	client := newHTTPClient(30 * time.Second)

	values := url.Values{}
	values.Set("auth_key", authKey)
	values.Set("text", text)
	values.Set("target_lang", strings.ToUpper(targetLang))

	req, err := http.NewRequest(
		"POST",
		"https://api-free.deepl.com/v2/translate",
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("deepl request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("deepl API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse deepl response: %w", err)
	}
	if len(result.Translations) == 0 {
		return "", fmt.Errorf("deepl returned no translations")
	}
	return result.Translations[0].Text, nil
}
