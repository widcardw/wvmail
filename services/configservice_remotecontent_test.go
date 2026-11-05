package services

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// The remote-content section has to survive a config round-trip, and a config
// written before the option existed must keep blocking remote content.
func TestRemoteContentConfigRoundTrip(t *testing.T) {
	in := AppConfig{
		RemoteContent: RemoteContentConfig{
			AllowRemoteContent: true,
			AllowedSenders:     []string{"someone@example.com", "@trusted.example"},
			ImageProxyURL:      "https://mail.example.com/image-proxy",
		},
	}

	data, err := yaml.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out AppConfig
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.RemoteContent.AllowRemoteContent {
		t.Error("AllowRemoteContent was lost in the round-trip")
	}
	if len(out.RemoteContent.AllowedSenders) != 2 {
		t.Fatalf("AllowedSenders = %v, want 2 entries", out.RemoteContent.AllowedSenders)
	}
	if out.RemoteContent.ImageProxyURL != "https://mail.example.com/image-proxy" {
		t.Errorf("ImageProxyURL = %q, want the round-tripped URL", out.RemoteContent.ImageProxyURL)
	}
}

func TestRemoteContentDefaultsToBlocking(t *testing.T) {
	// "remote_content" is absent from configs written by older versions; the
	// zero value has to mean "block remote content", not "load it".
	var cfg AppConfig
	if err := yaml.Unmarshal([]byte("language: zh-CN\ntheme: Dark\n"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.RemoteContent.AllowRemoteContent {
		t.Error("remote content must be blocked by default")
	}
	if len(cfg.RemoteContent.AllowedSenders) != 0 {
		t.Errorf("AllowedSenders = %v, want empty", cfg.RemoteContent.AllowedSenders)
	}
}

func TestValidateImageProxyURL(t *testing.T) {
	valid := []string{
		"",
		"https://mail.example.com/image-proxy",
		"http://127.0.0.1:8080/image-proxy?token=x",
	}
	for _, raw := range valid {
		if err := validateImageProxyURL(raw); err != nil {
			t.Errorf("validateImageProxyURL(%q) = %v, want nil", raw, err)
		}
	}

	invalid := []string{
		"mail.example.com/image-proxy",
		"javascript:alert(1)",
		"https://",
		"https://user:" + "pw" + "@example.com/image-proxy",
		"https://mail.example.com/image-proxy#fragment",
	}
	for _, raw := range invalid {
		if err := validateImageProxyURL(raw); err == nil {
			t.Errorf("validateImageProxyURL(%q) = nil, want an error", raw)
		}
	}
}
