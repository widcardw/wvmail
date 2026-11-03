package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func aiAssistChatServer(t *testing.T, reply func(model string) string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(request.Messages) == 0 || !strings.Contains(request.Messages[0].Content, "email") {
			t.Errorf("prompt did not contain the email: %s", string(body))
		}
		response, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{"content": reply(request.Model)},
			}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func seedAIAssistConfig(t *testing.T, baseURL string, models ...string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	cfg := &ConfigService{}
	if err := cfg.SaveConfig(AppConfig{
		TranslationConfig: TranslationConfig{
			DefaultTargetLang: "zh-CN",
			DefaultEngine:     "llm",
			LLMConfigName:     "main",
			LLMModel:          models[0],
		},
		LLMConfigs: []LLMConfig{{
			Name:     "main",
			Provider: "custom",
			BaseURL:  baseURL,
			Model:    models[0],
			Models:   models,
		}},
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
}

func TestSummarizeTextUsesSelectedModelAndCachesSuccess(t *testing.T) {
	server, calls := aiAssistChatServer(t, func(model string) string {
		return "summary from " + model
	})
	seedAIAssistConfig(t, server.URL, "default-model", "summary-model")

	svc := NewAIAssistService(&ConfigService{})
	request := AIAssistRequest{
		Text:          "Please review the attached email before Friday.",
		LLMConfigName: "main",
		LLMModel:      "summary-model",
	}
	got, err := svc.SummarizeText(request)
	if err != nil {
		t.Fatalf("SummarizeText: %v", err)
	}
	if got != "summary from summary-model" {
		t.Fatalf("summary = %q", got)
	}

	again, err := svc.SummarizeText(request)
	if err != nil {
		t.Fatalf("cached SummarizeText: %v", err)
	}
	if again != got {
		t.Fatalf("cached summary = %q, want %q", again, got)
	}
	if calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1", calls.Load())
	}
}

func TestTranslateTextCacheSeparatesModels(t *testing.T) {
	server, calls := aiAssistChatServer(t, func(model string) string {
		return "translation from " + model
	})
	seedAIAssistConfig(t, server.URL, "model-a", "model-b")

	svc := NewAIAssistService(&ConfigService{})
	first, err := svc.TranslateText(AIAssistRequest{
		Text:          "hello email",
		LLMConfigName: "main",
		LLMModel:      "model-a",
	})
	if err != nil {
		t.Fatalf("first TranslateText: %v", err)
	}
	second, err := svc.TranslateText(AIAssistRequest{
		Text:          "hello email",
		LLMConfigName: "main",
		LLMModel:      "model-b",
	})
	if err != nil {
		t.Fatalf("second TranslateText: %v", err)
	}
	if first != "translation from model-a" || second != "translation from model-b" {
		t.Fatalf("translations = %q, %q", first, second)
	}
	if calls.Load() != 2 {
		t.Fatalf("provider calls = %d, want 2 (cache key must include the model)", calls.Load())
	}
}

func TestSkipAIAssistConfirmationRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := &ConfigService{}
	if err := cfg.SaveConfigField("skip_ai_assist_confirmation", "true"); err != nil {
		t.Fatalf("SaveConfigField: %v", err)
	}
	got, err := cfg.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if !got.SkipAIAssistConfirmation {
		t.Fatal("SkipAIAssistConfirmation was not persisted")
	}
	if err := cfg.SaveConfigField("skip_ai_assist_confirmation", "not-a-bool"); err == nil {
		t.Fatal("invalid boolean must be rejected")
	}
}
