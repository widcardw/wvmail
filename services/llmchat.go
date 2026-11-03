package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultMaxOutputTokens = 4096

// findLLMConfig resolves one provider by name and applies an optional per-call
// model override. An empty name means the first configured provider, and an
// empty model means that provider's default. Every LLM-backed feature uses this
// rule, so Settings, tag suggestions, translation and summaries cannot disagree
// about which model a selection means.
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

	logLLMRequest(cfg.Provider, cfg.Model, prompt)
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

// --- Response parsers ---

func parseOpenAIResponse(resp *http.Response) (string, error) {
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	logLLMResponse("openai-compatible", string(respBody))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	var result struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
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
	content := result.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		if result.Choices[0].FinishReason == "length" {
			return "", fmt.Errorf(
				"the LLM response was truncated before visible content was returned; increase the output token limit",
			)
		}
		return "", fmt.Errorf("the LLM returned an empty response")
	}
	return content, nil
}

func parseAnthropicResponse(resp *http.Response) (string, error) {
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	logLLMResponse("anthropic", string(respBody))
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
	if strings.TrimSpace(result.Content[0].Text) == "" {
		return "", fmt.Errorf("the LLM returned an empty response")
	}
	return result.Content[0].Text, nil
}

func parseGeminiResponse(resp *http.Response) (string, error) {
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	logLLMResponse("gemini", string(respBody))
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
	if strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text) == "" {
		return "", fmt.Errorf("the LLM returned an empty response")
	}
	return result.Candidates[0].Content.Parts[0].Text, nil
}
