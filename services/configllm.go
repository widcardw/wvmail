package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// The LLM provider side of the config: the connection test and the model list
// behind Settings → LLM. Both talk HTTP; neither touches config.yaml.

// TestLLMConnection tests the LLM configuration by attempting a real chat
// completion request. If a model is specified it sends a minimal message
// ("Say hi in one word") to validate end-to-end connectivity. Without a model
// it falls back to a basic connectivity check. The request times out after 30
// seconds.
func (s *ConfigService) TestLLMConnection(cfg LLMConfig) (bool, string) {
	client := &http.Client{Timeout: 60 * time.Second}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	var req *http.Request
	var err error

	if cfg.Model != "" {
		// Real chat completion test
		var jsonBody []byte
		switch cfg.Provider {
		case "anthropic":
			body := map[string]interface{}{
				"model":      cfg.Model,
				"max_tokens": 10,
				"thinking":   map[string]string{"type": "disabled"},
				"messages":   []map[string]string{{"role": "user", "content": "Say hi in one word"}},
			}
			jsonBody, err = json.Marshal(body)
			if err == nil {
				req, err = http.NewRequest("POST", baseURL+"/v1/messages", strings.NewReader(string(jsonBody)))
				if err == nil {
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("x-api-key", cfg.APIKey)
					req.Header.Set("anthropic-version", "2023-06-01")
				}
			}
		case "gemini":
			body := fmt.Sprintf(`{"contents":[{"parts":[{"text":"Say hi in one word"}]}],"model":"models/%s"}`, cfg.Model)
			req, err = http.NewRequest("POST", baseURL+"/v1/models/"+cfg.Model+":generateContent", strings.NewReader(body))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("x-goog-api-key", cfg.APIKey)
			}
		case "ollama":
			body := map[string]interface{}{
				"model":    cfg.Model,
				"messages": []map[string]string{{"role": "user", "content": "Say hi in one word"}},
				"stream":   false,
			}
			jsonBody, err = json.Marshal(body)
			if err == nil {
				req, err = http.NewRequest("POST", baseURL+"/api/chat", strings.NewReader(string(jsonBody)))
				if err == nil {
					req.Header.Set("Content-Type", "application/json")
				}
			}
		default:
			// deepseek, openai, kimi, minimax, bytedance, custom
			body := map[string]interface{}{
				"model":      cfg.Model,
				"messages":   []map[string]string{{"role": "user", "content": "Say hi in one word"}},
				"max_tokens": 10,
			}
			jsonBody, err = json.Marshal(body)
			if err == nil {
				req, err = http.NewRequest("POST", baseURL+"/chat/completions", strings.NewReader(string(jsonBody)))
				if err == nil {
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
				}
			}
		}
	} else {
		// Fallback – basic connectivity check. This mirrors the catalog endpoints
		// in ListLLMModels: a probe must hit the same URL the "fetch models" path
		// uses, otherwise a valid key can look broken (Anthropic's /v1/messages is
		// POST-only and answers 405 to a GET).
		switch cfg.Provider {
		case "anthropic":
			req, err = http.NewRequest("GET", baseURL+"/v1/models", nil)
			if err == nil {
				req.Header.Set("x-api-key", cfg.APIKey)
				req.Header.Set("anthropic-version", "2023-06-01")
			}
		case "gemini":
			req, err = http.NewRequest("GET", baseURL+"/v1/models", nil)
			if err == nil {
				req.Header.Set("x-goog-api-key", cfg.APIKey)
			}
		case "kimi", "minimax":
			req, err = http.NewRequest("GET", baseURL+"/v1/models", nil)
			if err == nil {
				req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			}
		case "ollama":
			req, err = http.NewRequest("GET", baseURL+"/api/tags", nil)
		default:
			req, err = http.NewRequest("GET", baseURL+"/models", nil)
			if err == nil {
				req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			}
		}
	}
	if err != nil {
		return false, fmt.Sprintf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) || strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "deadline exceeded") {
			return false, "Connection timed out after 60 seconds"
		}
		return false, fmt.Sprintf("Connection failed: %v", err)
	}
	defer resp.Body.Close()

	// A connection-only test requests a model catalog. Its successful response
	// can contain thousands of models, but the caller only needs the status.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && cfg.Model == "" {
		return true, fmt.Sprintf("Connected successfully (HTTP %d)", resp.StatusCode)
	}

	// Keep diagnostic responses bounded so a provider cannot send a large body
	// that has to be transferred, parsed, and rendered by the frontend.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	bodyStr := strings.TrimSpace(string(body))
	if bodyStr == "" {
		bodyStr = "(no response body)"
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, fmt.Sprintf("Connected successfully (HTTP %d)\n%s", resp.StatusCode, bodyStr)
	}
	return false, fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, bodyStr)
}

// llmModelsMaxEntries bounds the catalog returned to the frontend so a gateway
// listing thousands of models cannot bloat the response or the picker.
const llmModelsMaxEntries = 2000

// ListLLMModels asks the configured provider for its model catalog so the UI can
// offer a picker instead of requiring a hand-typed model name.
//
// Provider coverage mirrors TestLLMConnection's catalog probe:
//
//	openai-compatible (openai, deepseek, kimi, minimax, volcengine, custom)
//	    GET {base}/models            {"data":[{"id":"..."}]}
//	anthropic   GET {base}/v1/models {"data":[{"id":"..."}]}
//	gemini      GET {base}/v1/models {"models":[{"name":"models/..."}]}
//	ollama      GET {base}/api/tags  {"models":[{"name":"..."}]}
//
// Providers without a catalog endpoint (or that reject the request) return an
// error; callers must keep supporting a manually typed model name.
func (s *ConfigService) ListLLMModels(cfg LLMConfig) ([]string, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	var req *http.Request
	var err error

	switch cfg.Provider {
	case "anthropic":
		req, err = http.NewRequest("GET", baseURL+"/v1/models", nil)
		if err == nil {
			req.Header.Set("x-api-key", cfg.APIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		}
	case "gemini":
		// pageSize asks for the whole catalog in one response instead of the
		// default page of 50.
		req, err = http.NewRequest("GET", baseURL+"/v1/models?pageSize=1000", nil)
		if err == nil {
			req.Header.Set("x-goog-api-key", cfg.APIKey)
		}
	case "ollama":
		req, err = http.NewRequest("GET", baseURL+"/api/tags", nil)
	default:
		req, err = http.NewRequest("GET", baseURL+"/models", nil)
		if err == nil && cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) || strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "deadline exceeded") {
			return nil, fmt.Errorf("request timed out after 30 seconds")
		}
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	// Catalogs are small JSON, but bound the read anyway.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("the provider refused the model list: HTTP %d %s",
			resp.StatusCode, firstLine(body))
	}

	models := parseLLMModelIDs(body)
	if len(models) == 0 {
		return nil, fmt.Errorf("the provider returned no models (HTTP %d)", resp.StatusCode)
	}
	if len(models) > llmModelsMaxEntries {
		models = models[:llmModelsMaxEntries]
	}
	return models, nil
}

// llmModelCatalog covers every catalog shape we know of: OpenAI-style `data`,
// Gemini/Ollama-style `models`. Unknown fields are ignored.
type llmModelCatalog struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Models []struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	} `json:"models"`
}

// parseLLMModelIDs extracts model ids from a catalog response, strips Gemini's
// "models/" prefix, drops empties, de-duplicates and sorts the result.
func parseLLMModelIDs(body []byte) []string {
	var catalog llmModelCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil
	}

	ids := make([]string, 0, len(catalog.Data)+len(catalog.Models))
	for _, entry := range catalog.Data {
		if entry.ID != "" {
			ids = append(ids, entry.ID)
		}
	}
	for _, entry := range catalog.Models {
		name := entry.Name
		if name == "" {
			name = entry.Model
		}
		name = strings.TrimPrefix(name, "models/")
		if name != "" {
			ids = append(ids, name)
		}
	}

	sort.Strings(ids)
	unique := ids[:0]
	for i, id := range ids {
		if i == 0 || id != ids[i-1] {
			unique = append(unique, id)
		}
	}
	return unique
}

// firstLine returns the first non-empty line of a response body, bounded, for
// error messages.
func firstLine(body []byte) string {
	line := strings.TrimSpace(string(body))
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	if len(line) > 200 {
		line = line[:200] + "…"
	}
	if line == "" {
		return "(no response body)"
	}
	return line
}
