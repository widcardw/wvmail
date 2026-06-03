package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AppConfig represents the application configuration stored in YAML.
type AppConfig struct {
	Language   string      `yaml:"language"`
	Theme      string      `yaml:"theme"`
	LLMConfigs []LLMConfig `yaml:"llm_configs,omitempty"`
}

// LLMConfig represents a single LLM provider configuration.
type LLMConfig struct {
	Name     string `yaml:"name"`
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
	APIKey   string `yaml:"api_key"`
}

// ConfigService provides methods for reading, writing, and managing
// the application's YAML configuration file, with encrypted API key storage.
type ConfigService struct{}

func configDir() (string, error) {
	userDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config dir: %w", err)
	}
	dir := filepath.Join(userDir, "wvmail")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config dir: %w", err)
	}
	return dir, nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func masterKeyPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".master.key"), nil
}

// loadMasterKey loads the AES-256 master key from disk, generating one if it
// does not exist.
func loadMasterKey() ([]byte, error) {
	path, err := masterKeyPath()
	if err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(path); err == nil && len(data) == 32 {
		return data, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate master key: %w", err)
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, fmt.Errorf("failed to write master key: %w", err)
	}
	return key, nil
}

// encrypt encrypts plaintext with AES-256-GCM and returns a base64-encoded
// ciphertext.
func encrypt(plaintext string) (string, error) {
	key, err := loadMasterKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

// decrypt decrypts a base64-encoded AES-256-GCM ciphertext back to plaintext.
func decrypt(encoded string) (string, error) {
	key, err := loadMasterKey()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plaintext, err := aead.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}
	return string(plaintext), nil
}

func defaultConfig() AppConfig {
	return AppConfig{
		Language:   "en-US",
		Theme:      "System",
		LLMConfigs: []LLMConfig{},
	}
}

// GetConfig reads the application config from disk. If the file does not exist,
// a default config is returned without error.
func (s *ConfigService) GetConfig() (AppConfig, error) {
	path, err := configPath()
	if err != nil {
		return defaultConfig(), err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return defaultConfig(), fmt.Errorf("failed to read config: %w", err)
	}
	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), fmt.Errorf("failed to parse config: %w", err)
	}
	// Decrypt API keys
	for i := range cfg.LLMConfigs {
		if cfg.LLMConfigs[i].APIKey != "" {
			decrypted, err := decrypt(cfg.LLMConfigs[i].APIKey)
			if err == nil {
				cfg.LLMConfigs[i].APIKey = decrypted
			}
			// If decryption fails, leave the raw value (might be a legacy
			// unencrypted key, or corrupted).
		}
	}
	if cfg.Language == "" {
		cfg.Language = "en-US"
	}
	if cfg.Theme == "" {
		cfg.Theme = "System"
	}
	if cfg.LLMConfigs == nil {
		cfg.LLMConfigs = []LLMConfig{}
	}
	return cfg, nil
}

// SaveConfig writes the application config to disk, encrypting all API keys
// before persisting.
func (s *ConfigService) SaveConfig(cfg AppConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	// Encrypt API keys before writing
	for i := range cfg.LLMConfigs {
		if cfg.LLMConfigs[i].APIKey != "" {
			encrypted, err := encrypt(cfg.LLMConfigs[i].APIKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt API key for %q: %w", cfg.LLMConfigs[i].Name, err)
			}
			cfg.LLMConfigs[i].APIKey = encrypted
		}
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// SaveConfigField updates a single top-level field in the config file. This
// avoids read-update-write races between stores (e.g. language & theme).
// Supported fields: "language", "theme".
func (s *ConfigService) SaveConfigField(field, value string) error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	switch field {
	case "language":
		cfg.Language = value
	case "theme":
		cfg.Theme = value
	default:
		return fmt.Errorf("unknown config field: %s", field)
	}
	return s.SaveConfig(cfg)
}

// SaveLLMConfigs replaces the entire LLM config list in the config file.
func (s *ConfigService) SaveLLMConfigs(configs []LLMConfig) error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	cfg.LLMConfigs = configs
	return s.SaveConfig(cfg)
}

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
		// Fallback – basic connectivity check
		switch cfg.Provider {
		case "anthropic":
			req, err = http.NewRequest("GET", baseURL+"/v1/messages", nil)
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

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	bodyStr := strings.TrimSpace(string(body))
	if bodyStr == "" {
		bodyStr = "(no response body)"
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, fmt.Sprintf("Connected successfully (HTTP %d)\n%s", resp.StatusCode, bodyStr)
	}
	return false, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, bodyStr)
}
