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

const defaultMaxOutputTokens = 4096

// TranslationService provides methods for translating email content using
// either a configured LLM or the DeepL API. It caches translations in memory
// so repeated requests for the same text + target language return immediately.
type TranslationService struct {
	cache sync.Map // map[string]cachedResult
}

type cachedResult struct {
	text      string
	err       error
	timestamp time.Time
}

func (s *TranslationService) cacheKey(text, targetLang, engine string) string {
	h := sha256.Sum256([]byte(text + "|" + targetLang + "|" + engine))
	return fmt.Sprintf("%x", h[:16])
}

func (s *TranslationService) getFromCache(key string) (string, error, bool) {
	if v, ok := s.cache.Load(key); ok {
		cr := v.(cachedResult)
		// Cache valid for 1 hour
		if time.Since(cr.timestamp) < time.Hour {
			return cr.text, cr.err, true
		}
		s.cache.Delete(key)
	}
	return "", nil, false
}

func (s *TranslationService) storeInCache(key, text string, err error) {
	s.cache.Store(key, cachedResult{text: text, err: err, timestamp: time.Now()})
}

// TranslateText translates the given text to the target language using the
// specified engine. Supported engines: "llm", "deepl". Results are cached
// in memory for 1 hour to avoid repeated API calls.
func (s *TranslationService) TranslateText(text, targetLang, engine string, llmCfg *LLMConfig, deeplKey string) (string, error) {
	key := s.cacheKey(text, targetLang, engine)
	if cached, err, ok := s.getFromCache(key); ok {
		return cached, err
	}

	var result string
	var err error
	switch engine {
	case "deepl":
		result, err = s.translateDeepL(text, targetLang, deeplKey)
	case "llm":
		if llmCfg == nil {
			err = fmt.Errorf("no LLM configuration provided for translation")
		} else {
			result, err = s.translateLLM(text, targetLang, llmCfg)
		}
	default:
		err = fmt.Errorf("unsupported translation engine: %q", engine)
	}
	s.storeInCache(key, result, err)
	return result, err
}

// translateLLM translates text using the given LLM config.
// It uses provider-specific API formats.
func (s *TranslationService) translateLLM(text, targetLang string, cfg *LLMConfig) (string, error) {
	prompt := fmt.Sprintf(
		"Translate the following text to %s. Preserve any email formatting, line breaks, and "+
			"only respond with the translation, no explanations.\n\n%s",
		targetLang, text,
	)
	return requestLLMChat(cfg, prompt, cfg.MaxOutputTokens)
}

// translateDeepL translates text using the DeepL API.
// targetLang should be a DeepL language code (e.g. "ZH", "EN", "JA").
func (s *TranslationService) translateDeepL(text, targetLang, authKey string) (string, error) {
	if authKey == "" {
		return "", fmt.Errorf("DeepL auth key is not configured")
	}

	client := newHTTPClient(30 * time.Second)

	values := url.Values{}
	values.Set("auth_key", authKey)
	values.Set("text", text)
	values.Set("target_lang", strings.ToUpper(targetLang))

	req, err := http.NewRequest("POST", "https://api-free.deepl.com/v2/translate",
		strings.NewReader(values.Encode()))
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

// --- Response parsers ---

func parseOpenAIResponse(resp *http.Response) (string, error) {
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return result.Choices[0].Message.Content, nil
}

func parseAnthropicResponse(resp *http.Response) (string, error) {
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse anthropic response: %w", err)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("no content in anthropic response")
	}
	return result.Content[0].Text, nil
}

func parseGeminiResponse(resp *http.Response) (string, error) {
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse gemini response: %w", err)
	}
	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in gemini response")
	}
	if len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no parts in gemini response")
	}
	return result.Candidates[0].Content.Parts[0].Text, nil
}
