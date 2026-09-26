package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func llmCatalogServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Every provider shape below is reached through a different path; record
		// nothing, just answer with the canned payload.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestListLLMModelsParsesCatalogShapes(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		body     string
		want     []string
	}{
		{
			name:     "openai style data[].id",
			provider: "custom",
			body:     `{"object":"list","data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"},{"id":"gpt-4o"}]}`,
			want:     []string{"gpt-4o", "gpt-4o-mini"},
		},
		{
			name:     "gemini models[].name keeps only the id",
			provider: "gemini",
			body:     `{"models":[{"name":"models/gemini-2.5-pro"},{"name":"models/gemini-2.0-flash"}]}`,
			want:     []string{"gemini-2.0-flash", "gemini-2.5-pro"},
		},
		{
			name:     "ollama tags use model name",
			provider: "ollama",
			body:     `{"models":[{"name":"llama3.2:latest","model":"llama3.2:latest"},{"name":"qwen2.5:7b"}]}`,
			want:     []string{"llama3.2:latest", "qwen2.5:7b"},
		},
		{
			name:     "volcengine style gateway",
			provider: "volcengine",
			body:     `{"data":[{"id":"doubao-seed-1-6-250615"},{"id":"doubao-1-5-pro-32k"}]}`,
			want:     []string{"doubao-1-5-pro-32k", "doubao-seed-1-6-250615"},
		},
		{
			name:     "unknown fields and empty ids are ignored",
			provider: "custom",
			body:     `{"data":[{"id":""},{"id":"m1","object":"model"}],"extra":42}`,
			want:     []string{"m1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := llmCatalogServer(t, http.StatusOK, tc.body)
			got, err := (&ConfigService{}).ListLLMModels(LLMConfig{
				Provider: tc.provider,
				BaseURL:  server.URL,
				APIKey:   "test-key",
			})
			if err != nil {
				t.Fatalf("ListLLMModels: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestListLLMModelsReportsProviderRefusal(t *testing.T) {
	// A provider without a catalog endpoint must surface a usable reason: the UI
	// keeps offering manual model entry and shows this text.
	server := llmCatalogServer(t, http.StatusNotFound, "not found\nsecond line")
	_, err := (&ConfigService{}).ListLLMModels(LLMConfig{Provider: "custom", BaseURL: server.URL})
	if err == nil {
		t.Fatal("expected an error for HTTP 404")
	}
	msg := err.Error()
	if !strings.Contains(msg, "404") || !strings.Contains(msg, "not found") {
		t.Fatalf("error should mention the status and first body line, got %q", msg)
	}
	if strings.Contains(msg, "second line") {
		t.Fatalf("error should only keep the first body line, got %q", msg)
	}
}

func TestListLLMModelsRejectsEmptyCatalog(t *testing.T) {
	server := llmCatalogServer(t, http.StatusOK, `{"data":[]}`)
	if _, err := (&ConfigService{}).ListLLMModels(LLMConfig{Provider: "custom", BaseURL: server.URL}); err == nil {
		t.Fatal("expected an error when the catalog is empty")
	}
}

func TestListLLMModelsRequiresBaseURL(t *testing.T) {
	if _, err := (&ConfigService{}).ListLLMModels(LLMConfig{Provider: "custom"}); err == nil {
		t.Fatal("expected an error without a base URL")
	}
}

func TestLLMConfigModelsRoundTrip(t *testing.T) {
	// The picked model set has to survive a config round-trip, and configs written
	// before the field existed must stay valid (Model remains the active one).
	in := AppConfig{LLMConfigs: []LLMConfig{{
		Name:     "volcengine",
		Provider: "volcengine",
		Model:    "doubao-seed-1-6-250615",
		Models:   []string{"doubao-1-5-pro-32k", "doubao-seed-1-6-250615"},
		BaseURL:  "https://ark.cn-beijing.volces.com/api/v3",
		APIKey:   "secret",
	}}}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out AppConfig
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.LLMConfigs) != 1 || len(out.LLMConfigs[0].Models) != 2 {
		t.Fatalf("Models did not round-trip: %+v", out.LLMConfigs)
	}
	if out.LLMConfigs[0].Model != "doubao-seed-1-6-250615" {
		t.Errorf("active model changed: %q", out.LLMConfigs[0].Model)
	}
}
