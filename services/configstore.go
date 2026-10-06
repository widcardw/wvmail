package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"gopkg.in/yaml.v3"
)

// Storage for the single config.yaml: where it lives, the master key and the
// AES-GCM secret helpers, the atomic + serialized writes, the field-level save
// the settings UI uses, and the migrations applied while reading.

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
