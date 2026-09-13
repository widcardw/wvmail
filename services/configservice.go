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
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AppConfig represents the application configuration stored in YAML.
type AppConfig struct {
	Language               string            `yaml:"language"`
	Theme                  string            `yaml:"theme"`
	PrimaryColor           string            `yaml:"primary_color"`
	DatabasePath           string            `yaml:"database_path,omitempty"`
	LogConfig              LogConfig         `yaml:"log_config,omitempty"`
	TranslationConfig      TranslationConfig `yaml:"translation_config,omitempty"`
	ScanExternalSafety     bool              `yaml:"scan_external_safety,omitempty"`
	SearchTagDeletePresses int               `yaml:"search_tag_delete_presses,omitempty"`
	LLMConfigs             []LLMConfig       `yaml:"llm_configs,omitempty"`
	EmailAccounts          []EmailAccount    `yaml:"email_accounts,omitempty"`
}

// LogConfig controls the application's file logging behaviour.
type LogConfig struct {
	Level         string `yaml:"level"`          // debug, info, warn, error
	RetentionDays int    `yaml:"retention_days"` // days to keep old log files
	MaxSizeMB     int    `yaml:"max_size_mb"`    // max size per log file before rotation
}

// TranslationConfig controls the email translation feature.
type TranslationConfig struct {
	DefaultTargetLang string `yaml:"default_target_lang"`
	DefaultEngine     string `yaml:"default_engine"` // "llm", "deepl"
	LLMConfigName     string `yaml:"llm_config_name"`
	DeepLAuthKey      string `yaml:"deepl_auth_key"` // AES-256-GCM encrypted
}

// LLMConfig represents a single LLM provider configuration.
type LLMConfig struct {
	Name                string `yaml:"name"`
	Provider            string `yaml:"provider"`
	Model               string `yaml:"model"`
	BaseURL             string `yaml:"base_url"`
	APIKey              string `yaml:"api_key"`
	ContextWindowTokens int    `yaml:"context_window_tokens,omitempty"`
	MaxOutputTokens     int    `yaml:"max_output_tokens,omitempty"`
}

// EmailAccount represents an email account configuration.
type EmailAccount struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Email    string `yaml:"email"`
	Provider string `yaml:"provider"` // gmail, outlook, 163, qq, custom, etc.
	// IMAP settings
	IMAPHost string `yaml:"imap_host"`
	IMAPPort int    `yaml:"imap_port"`
	IMAPUser string `yaml:"imap_user"`
	IMAPPass string `yaml:"imap_pass"` // AES-256-GCM encrypted on disk
	// SMTP settings
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	SMTPUser string `yaml:"smtp_user"`
	SMTPPass string `yaml:"smtp_pass"` // AES-256-GCM encrypted on disk
	UseTLS   bool   `yaml:"use_tls"`
	// Sync settings
	SyncInterval int    `yaml:"sync_interval"` // Auto-sync interval in minutes (0 = disabled)
	Avatar       string `yaml:"avatar"`        // Avatar filename (relative to avatar dir)
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

func avatarDir() (string, error) {
	base, err := configDir()
	if err != nil {
		return "", err
	}
	avatarPath := filepath.Join(base, "avatars")
	if err := os.MkdirAll(avatarPath, 0700); err != nil {
		return "", fmt.Errorf("failed to create avatar dir: %w", err)
	}
	return avatarPath, nil
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
		Language:     "en-US",
		Theme:        "System",
		PrimaryColor: "#18a058",
		DatabasePath: "",
		LogConfig: LogConfig{
			Level:         "info",
			RetentionDays: 30,
			MaxSizeMB:     10,
		},
		TranslationConfig: TranslationConfig{
			DefaultTargetLang: "zh-CN",
			DefaultEngine:     "llm",
		},
		ScanExternalSafety:     true,
		SearchTagDeletePresses: 2,
		EmailAccounts:          []EmailAccount{},
	}
}

// GetConfigDir returns the path to the application config directory.
// This is used by the frontend to open the config folder in the file manager.
func (s *ConfigService) GetConfigDir() (string, error) {
	return configDir()
}

// GetLogDir returns the path to the application log directory.
func (s *ConfigService) GetLogDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create log dir: %w", err)
	}
	return logDir, nil
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
	// Decrypt DeepL auth key
	if cfg.TranslationConfig.DeepLAuthKey != "" {
		decrypted, err := decrypt(cfg.TranslationConfig.DeepLAuthKey)
		if err == nil {
			cfg.TranslationConfig.DeepLAuthKey = decrypted
		}
	}
	// Decrypt email account passwords
	for i := range cfg.EmailAccounts {
		if cfg.EmailAccounts[i].IMAPPass != "" {
			decrypted, err := decrypt(cfg.EmailAccounts[i].IMAPPass)
			if err == nil {
				cfg.EmailAccounts[i].IMAPPass = decrypted
			}
		}
		if cfg.EmailAccounts[i].SMTPPass != "" {
			decrypted, err := decrypt(cfg.EmailAccounts[i].SMTPPass)
			if err == nil {
				cfg.EmailAccounts[i].SMTPPass = decrypted
			}
		}
	}
	if cfg.Language == "" {
		cfg.Language = "en-US"
	}
	if cfg.Theme == "" {
		cfg.Theme = "System"
	}
	if cfg.PrimaryColor == "" {
		cfg.PrimaryColor = "#18a058"
	}
	if cfg.SearchTagDeletePresses != 1 && cfg.SearchTagDeletePresses != 2 {
		cfg.SearchTagDeletePresses = 2
	}
	if cfg.LogConfig.Level == "" {
		cfg.LogConfig.Level = "info"
	}
	if cfg.LogConfig.RetentionDays == 0 {
		cfg.LogConfig.RetentionDays = 30
	}
	if cfg.LogConfig.MaxSizeMB == 0 {
		cfg.LogConfig.MaxSizeMB = 10
	}
	if cfg.TranslationConfig.DefaultTargetLang == "" {
		cfg.TranslationConfig.DefaultTargetLang = "zh-CN"
	}
	if cfg.TranslationConfig.DefaultEngine == "" {
		cfg.TranslationConfig.DefaultEngine = "llm"
	}
	if cfg.LLMConfigs == nil {
		cfg.LLMConfigs = []LLMConfig{}
	}
	if cfg.EmailAccounts == nil {
		cfg.EmailAccounts = []EmailAccount{}
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
	// Encrypt DeepL auth key
	if cfg.TranslationConfig.DeepLAuthKey != "" {
		encrypted, err := encrypt(cfg.TranslationConfig.DeepLAuthKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt DeepL auth key: %w", err)
		}
		cfg.TranslationConfig.DeepLAuthKey = encrypted
	}
	// Encrypt email account passwords before writing
	for i := range cfg.EmailAccounts {
		if cfg.EmailAccounts[i].IMAPPass != "" {
			encrypted, err := encrypt(cfg.EmailAccounts[i].IMAPPass)
			if err != nil {
				return fmt.Errorf("failed to encrypt IMAP password for %q: %w", cfg.EmailAccounts[i].Email, err)
			}
			cfg.EmailAccounts[i].IMAPPass = encrypted
		}
		if cfg.EmailAccounts[i].SMTPPass != "" {
			encrypted, err := encrypt(cfg.EmailAccounts[i].SMTPPass)
			if err != nil {
				return fmt.Errorf("failed to encrypt SMTP password for %q: %w", cfg.EmailAccounts[i].Email, err)
			}
			cfg.EmailAccounts[i].SMTPPass = encrypted
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
// Supported fields: "language", "theme", "primary_color",
// "scan_external_safety", and "search_tag_delete_presses".
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
	case "primary_color":
		cfg.PrimaryColor = value
	case "scan_external_safety":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value for scan_external_safety: %s", value)
		}
		cfg.ScanExternalSafety = b
	case "search_tag_delete_presses":
		presses, err := strconv.Atoi(value)
		if err != nil || (presses != 1 && presses != 2) {
			return fmt.Errorf("invalid value for search_tag_delete_presses: %s", value)
		}
		cfg.SearchTagDeletePresses = presses
	default:
		return fmt.Errorf("unknown config field: %s", field)
	}
	return s.SaveConfig(cfg)
}

// SaveLogConfig updates the log settings in the config file.
func (s *ConfigService) SaveLogConfig(cfg LogConfig) error {
	appCfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	appCfg.LogConfig = cfg
	return s.SaveConfig(appCfg)
}

// SaveTranslationConfig updates the translation settings in the config file.
func (s *ConfigService) SaveTranslationConfig(cfg TranslationConfig) error {
	appCfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	appCfg.TranslationConfig = cfg
	return s.SaveConfig(appCfg)
}

func (s *ConfigService) SaveLLMConfigs(configs []LLMConfig) error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	cfg.LLMConfigs = configs
	return s.SaveConfig(cfg)
}

// GetAccounts returns all email accounts from the config.
func (s *ConfigService) GetAccounts() ([]EmailAccount, error) {
	cfg, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	return cfg.EmailAccounts, nil
}

// SaveAccounts replaces the entire email account list in the config file.
func (s *ConfigService) SaveAccounts(accounts []EmailAccount) error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	cfg.EmailAccounts = accounts
	return s.SaveConfig(cfg)
}

// AddAccount appends a new email account to the config.
func (s *ConfigService) AddAccount(account EmailAccount) error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	cfg.EmailAccounts = append(cfg.EmailAccounts, account)
	return s.SaveConfig(cfg)
}

// RemoveAccount removes an email account by its ID.
func (s *ConfigService) RemoveAccount(id string) error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	idx := -1
	for i, a := range cfg.EmailAccounts {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("account with id %q not found", id)
	}
	cfg.EmailAccounts = append(cfg.EmailAccounts[:idx], cfg.EmailAccounts[idx+1:]...)
	return s.SaveConfig(cfg)
}

// UpdateAccount updates an existing email account identified by its ID.
func (s *ConfigService) UpdateAccount(id string, account EmailAccount) error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	for i, a := range cfg.EmailAccounts {
		if a.ID == id {
			cfg.EmailAccounts[i] = account
			return s.SaveConfig(cfg)
		}
	}
	return fmt.Errorf("account with id %q not found", id)
}

// SaveAccountAvatar saves an avatar image for the given account ID.
// fileData should be the raw bytes of the image. Returns the relative filename stored.
func (s *ConfigService) SaveAccountAvatar(accountID string, fileData []byte, ext string) (string, error) {
	avatarPath, err := avatarDir()
	if err != nil {
		return "", err
	}
	filename := accountID + ext
	if err := os.WriteFile(filepath.Join(avatarPath, filename), fileData, 0644); err != nil {
		return "", fmt.Errorf("failed to save avatar: %w", err)
	}
	// Update account config with the new avatar filename
	cfg, err := s.GetConfig()
	if err != nil {
		return "", err
	}
	for i := range cfg.EmailAccounts {
		if cfg.EmailAccounts[i].ID == accountID {
			cfg.EmailAccounts[i].Avatar = filename
			break
		}
	}
	if err := s.SaveConfig(cfg); err != nil {
		return "", err
	}
	return filename, nil
}

// GetAccountAvatar reads an avatar image file and returns it as a base64 data URI.
func (s *ConfigService) GetAccountAvatar(accountID string) (string, error) {
	cfg, err := s.GetConfig()
	if err != nil {
		return "", err
	}
	avatarFile := ""
	for _, a := range cfg.EmailAccounts {
		if a.ID == accountID && a.Avatar != "" {
			avatarFile = a.Avatar
			break
		}
	}
	if avatarFile == "" {
		return "", nil
	}
	avatarPath, err := avatarDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(avatarPath, avatarFile))
	if err != nil {
		return "", fmt.Errorf("failed to read avatar: %w", err)
	}
	ext := filepath.Ext(avatarFile)
	mime := "image/png"
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
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
