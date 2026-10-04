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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"gopkg.in/yaml.v3"
)

// AppConfig represents the application configuration stored in YAML.
type AppConfig struct {
	Language          string            `yaml:"language"`
	Theme             string            `yaml:"theme"`
	PrimaryColor      string            `yaml:"primary_color"`
	DatabasePath      string            `yaml:"database_path,omitempty"`
	LogConfig         LogConfig         `yaml:"log_config,omitempty"`
	TranslationConfig TranslationConfig `yaml:"translation_config,omitempty"`
	// No `omitempty` on the flags whose default is true: otherwise saving "off"
	// would omit the key and the next read would fall back to the default.
	ScanExternalSafety     bool                `yaml:"scan_external_safety"`
	SearchTagDeletePresses int                 `yaml:"search_tag_delete_presses,omitempty"`
	LLMConfigs             []LLMConfig         `yaml:"llm_configs,omitempty"`
	EmailAccounts          []EmailAccount      `yaml:"email_accounts,omitempty"`
	RemoteContent          RemoteContentConfig `yaml:"remote_content,omitempty"`
	// MinimizeWhenClose hides the main window to the system tray instead of
	// quitting. The zero value quits, which is what a config written before this
	// option existed expects.
	MinimizeWhenClose bool `yaml:"minimize_when_close"`
	// DetectDatesInMail highlights date/time mentions inside email bodies and lets
	// the user turn one into a reminder. Defaults to on (see defaultConfig), hence
	// no `omitempty` — an explicit "off" has to be written.
	DetectDatesInMail bool `yaml:"detect_dates_in_mail"`
}

// RemoteContentConfig controls how remote content referenced by email bodies
// (tracking pixels, remote images, fonts, imported CSS) is treated when a
// message is displayed.
type RemoteContentConfig struct {
	// AllowRemoteContent loads remote resources instead of blocking them. The
	// zero value (false) blocks by default, so configs written before this
	// option existed keep the safe behaviour.
	AllowRemoteContent bool `yaml:"allow_remote_content,omitempty"`
	// AllowedSenders lists senders whose remote content is always loaded
	// (lower-cased email addresses; a leading "@" matches a whole domain).
	AllowedSenders []string `yaml:"allowed_senders,omitempty"`
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
	// LLMModel picks which of that config's models to translate with. Empty means
	// "use the config's default model", so configs from older versions keep
	// working unchanged.
	LLMModel     string `yaml:"llm_model,omitempty"`
	DeepLAuthKey string `yaml:"deepl_auth_key"` // AES-256-GCM encrypted
}

// LLMConfig represents a single LLM provider configuration.
type LLMConfig struct {
	Name                string   `yaml:"name"`
	Provider            string   `yaml:"provider"`
	Model               string   `yaml:"model"`            // default/active model
	Models              []string `yaml:"models,omitempty"` // models picked from the provider catalog
	BaseURL             string   `yaml:"base_url"`
	APIKey              string   `yaml:"api_key"`
	ContextWindowTokens int      `yaml:"context_window_tokens,omitempty"`
	MaxOutputTokens     int      `yaml:"max_output_tokens,omitempty"`
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
		Language:          "en-US",
		Theme:             "System",
		PrimaryColor:      "#e11d48",
		DatabasePath:      "",
		MinimizeWhenClose: false,
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
		DetectDatesInMail:      true,
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

// configWriteMu serializes config writes. Every mutating method is a
// read-modify-write cycle, so holding a single lock across both steps is what
// stops two stores (e.g. language and theme) from overwriting each other. Reads
// do not take it: writes land with a rename, so a reader always sees a complete
// file, and read-modify-write callers use readConfig under the lock instead.
var configWriteMu sync.Mutex

// GetConfig reads the application config from disk. If the file does not exist,
// a default config is returned without error.
func (s *ConfigService) GetConfig() (AppConfig, error) {
	return readConfig()
}

// readConfig is GetConfig without the write lock. Callers that intend to mutate
// the result must hold configWriteMu across the read and the write.
func readConfig() (AppConfig, error) {
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
	// Start from the defaults so a key that is absent from an older config file
	// keeps its documented default instead of the Go zero value (a missing
	// `detect_dates_in_mail` must not silently disable the feature).
	cfg := defaultConfig()
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
		cfg.PrimaryColor = "#e11d48"
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

// MinimizeWhenClose reports whether closing the main window should hide it to the
// system tray instead of quitting. It is deliberately read on every call rather
// than cached at startup, so flipping the switch in Settings takes effect on the
// very next close. A read error means "quit", the less surprising behaviour.
func (s *ConfigService) MinimizeWhenClose() bool {
	cfg, err := s.GetConfig()
	if err != nil {
		return false
	}
	return cfg.MinimizeWhenClose
}

// SaveConfig writes the application config to disk, encrypting all API keys
// before persisting.
func (s *ConfigService) SaveConfig(cfg AppConfig) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	return writeConfig(cfg)
}

// writeConfig encrypts and persists cfg. Callers must hold configWriteMu.
func writeConfig(cfg AppConfig) error {
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
	if err := writeFileAtomic(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// writeFileAtomic replaces path with data through a temporary file in the same
// directory, so a crash or a full disk can never leave a truncated config behind.
// That matters here because the file also holds the encrypted account passwords
// and API keys: a plain os.WriteFile truncates first and would take them with it.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// No-op once the rename below has moved the file into place.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	// Flush before the rename: otherwise the rename can be durable while the
	// contents are not.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// SaveConfigField updates a single top-level field in the config file. The whole
// read-modify-write runs under configWriteMu, so two settings saved at the same
// time cannot lose each other's update.
// Supported fields: "language", "theme", "primary_color",
// "scan_external_safety", "search_tag_delete_presses", "allow_remote_content",
// "minimize_when_close", "detect_dates_in_mail" and "allowed_remote_senders"
// (the latter as a JSON array of strings).
func (s *ConfigService) SaveConfigField(field, value string) error {
	configWriteMu.Lock()
	languageChanged, err := saveConfigFieldLocked(field, value)
	configWriteMu.Unlock()

	if err != nil {
		return err
	}
	if languageChanged {
		notifyLanguageChanged(value)
	}
	return nil
}

// saveConfigFieldLocked performs the read-modify-write. Callers must hold
// configWriteMu. It reports whether the UI language actually changed, which the
// caller turns into an event once the lock is released.
func saveConfigFieldLocked(field, value string) (bool, error) {
	cfg, err := readConfig()
	if err != nil {
		return false, err
	}
	languageChanged := false
	switch field {
	case "language":
		languageChanged = cfg.Language != value
		cfg.Language = value
	case "theme":
		cfg.Theme = value
	case "primary_color":
		cfg.PrimaryColor = value
	case "scan_external_safety":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return false, fmt.Errorf("invalid boolean value for scan_external_safety: %s", value)
		}
		cfg.ScanExternalSafety = b
	case "search_tag_delete_presses":
		presses, err := strconv.Atoi(value)
		if err != nil || (presses != 1 && presses != 2) {
			return false, fmt.Errorf("invalid value for search_tag_delete_presses: %s", value)
		}
		cfg.SearchTagDeletePresses = presses
	case "allow_remote_content":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return false, fmt.Errorf("invalid boolean value for allow_remote_content: %s", value)
		}
		cfg.RemoteContent.AllowRemoteContent = b
	case "allowed_remote_senders":
		var senders []string
		if err := json.Unmarshal([]byte(value), &senders); err != nil {
			return false, fmt.Errorf("invalid JSON for allowed_remote_senders: %w", err)
		}
		cfg.RemoteContent.AllowedSenders = senders
	case "minimize_when_close":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return false, fmt.Errorf("invalid boolean value for minimize_when_close: %s", value)
		}
		cfg.MinimizeWhenClose = b
	case "detect_dates_in_mail":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return false, fmt.Errorf("invalid boolean value for detect_dates_in_mail: %s", value)
		}
		cfg.DetectDatesInMail = b
	default:
		return false, fmt.Errorf("unknown config field: %s", field)
	}
	return languageChanged, writeConfig(cfg)
}

// notifyLanguageChanged tells the Go side that the UI language changed at runtime.
// The system tray builds its menu before the webview exists and cannot read the
// frontend's i18n, so main.go listens for this and rebuilds its labels.
func notifyLanguageChanged(language string) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit("language-changed", language)
}

// SaveLogConfig updates the log settings in the config file.
func (s *ConfigService) SaveLogConfig(cfg LogConfig) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	appCfg, err := readConfig()
	if err != nil {
		return err
	}
	appCfg.LogConfig = cfg
	return writeConfig(appCfg)
}

// SaveTranslationConfig updates the translation settings in the config file.
func (s *ConfigService) SaveTranslationConfig(cfg TranslationConfig) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	appCfg, err := readConfig()
	if err != nil {
		return err
	}
	appCfg.TranslationConfig = cfg
	return writeConfig(appCfg)
}

func (s *ConfigService) SaveLLMConfigs(configs []LLMConfig) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	cfg.LLMConfigs = configs
	return writeConfig(cfg)
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
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	cfg.EmailAccounts = accounts
	return writeConfig(cfg)
}

// AddAccount appends a new email account to the config.
func (s *ConfigService) AddAccount(account EmailAccount) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	cfg.EmailAccounts = append(cfg.EmailAccounts, account)
	return writeConfig(cfg)
}

// RemoveAccount removes an email account by its ID.
func (s *ConfigService) RemoveAccount(id string) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	cfg, err := readConfig()
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
	return writeConfig(cfg)
}

// UpdateAccount updates an existing email account identified by its ID.
func (s *ConfigService) UpdateAccount(id string, account EmailAccount) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	for i, a := range cfg.EmailAccounts {
		if a.ID == id {
			cfg.EmailAccounts[i] = account
			return writeConfig(cfg)
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
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	cfg, err := readConfig()
	if err != nil {
		return "", err
	}
	for i := range cfg.EmailAccounts {
		if cfg.EmailAccounts[i].ID == accountID {
			cfg.EmailAccounts[i].Avatar = filename
			break
		}
	}
	if err := writeConfig(cfg); err != nil {
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
