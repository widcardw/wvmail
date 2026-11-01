package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// requestLLMChat is the one chat-completion request path shared by translation,
// tag suggestions and any future LLM feature. Provider-specific wire formats stay
// here; callers only supply the final prompt and output budget.
func requestLLMChat(cfg *LLMConfig, prompt string, maxOutputTokens int) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("no LLM configuration provided")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		return "", fmt.Errorf("LLM base URL is required")
	}
	if cfg.Model == "" {
		return "", fmt.Errorf("LLM model is required")
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultMaxOutputTokens
	}

	client := newHTTPClient(60 * time.Second)
	var req *http.Request
	var err error

	switch cfg.Provider {
	case "anthropic":
		body := map[string]interface{}{
			"model":      cfg.Model,
			"max_tokens": maxOutputTokens,
			"thinking":   map[string]string{"type": "disabled"},
			"messages":   []map[string]string{{"role": "user", "content": prompt}},
		}
		var jsonBody []byte
		jsonBody, err = json.Marshal(body)
		if err == nil {
			req, err = http.NewRequest("POST", baseURL+"/v1/messages", bytes.NewReader(jsonBody))
		}
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", cfg.APIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		}
	case "gemini":
		body := map[string]interface{}{
			"contents": []map[string]interface{}{
				{"parts": []map[string]string{{"text": prompt}}},
			},
			"generationConfig": map[string]int{"maxOutputTokens": maxOutputTokens},
		}
		var jsonBody []byte
		jsonBody, err = json.Marshal(body)
		if err == nil {
			req, err = http.NewRequest(
				"POST",
				baseURL+"/v1/models/"+cfg.Model+":generateContent",
				bytes.NewReader(jsonBody),
			)
		}
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-goog-api-key", cfg.APIKey)
		}
	case "ollama":
		body := map[string]interface{}{
			"model":    cfg.Model,
			"messages": []map[string]string{{"role": "user", "content": prompt}},
			"stream":   false,
			"options":  map[string]int{"num_predict": maxOutputTokens},
		}
		var jsonBody []byte
		jsonBody, err = json.Marshal(body)
		if err == nil {
			req, err = http.NewRequest("POST", baseURL+"/api/chat", bytes.NewReader(jsonBody))
		}
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	default:
		// OpenAI-compatible providers: OpenAI, DeepSeek, Kimi, MiniMax,
		// Volcengine and custom gateways.
		body := map[string]interface{}{
			"model":      cfg.Model,
			"messages":   []map[string]string{{"role": "user", "content": prompt}},
			"max_tokens": maxOutputTokens,
		}
		var jsonBody []byte
		jsonBody, err = json.Marshal(body)
		if err == nil {
			req, err = http.NewRequest(
				"POST",
				baseURL+"/chat/completions",
				bytes.NewReader(jsonBody),
			)
		}
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
	}
	if err != nil {
		return "", fmt.Errorf("create LLM request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s request failed: %w", cfg.Provider, err)
	}
	defer resp.Body.Close()

	switch cfg.Provider {
	case "anthropic":
		return parseAnthropicResponse(resp)
	case "gemini":
		return parseGeminiResponse(resp)
	default:
		return parseOpenAIResponse(resp)
	}
}
