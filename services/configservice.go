package services

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The config's public shape and the typed API the app uses: the AppConfig model
// (plus every sub-config and EmailAccount), the language/log/LLM savers and the
// account CRUD. How any of it is read from or written to disk is in
// configstore.go; talking to an LLM provider is in configllm.go.

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
	// Proxy is the app-wide proxy (Settings → Proxy). Off by default: a config
	// written before this option existed must keep connecting directly.
	Proxy ProxyConfig `yaml:"proxy,omitempty"`
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
	Level string `yaml:"level"` // debug, info, warn, error
	// RetentionDays is how long old log files are kept, restricted to the
	// Settings → About options (7, 30, 60, 90, 180, 365). 0 keeps expired
	// logs forever (expiry disabled).
	RetentionDays int `yaml:"retention_days"`
	// MaxSizeMB is the size at which app.log is rotated.
	MaxSizeMB int `yaml:"max_size_mb"`
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
	// OAuth2 online verification (Gmail / Microsoft). AuthType is "password"
	// (default, IMAPPass) or "oauth2" (OAuthRefreshToken exchanged for
	// short-lived access tokens, SASL XOAUTH2). OAuthClientID is the client the
	// refresh token was issued to and must be replayed on every refresh.
	AuthType          string `yaml:"auth_type,omitempty"`          // "password" | "oauth2"
	OAuthProvider     string `yaml:"oauth_provider,omitempty"`     // google | microsoft
	OAuthClientID     string `yaml:"oauth_client_id,omitempty"`    // client the token was issued to
	OAuthRefreshToken string `yaml:"oauth_refresh_token,omitempty"` // AES-256-GCM encrypted on disk
	// Sync settings
	SyncInterval int    `yaml:"sync_interval"` // Auto-sync interval in minutes (0 = disabled)
	Avatar       string `yaml:"avatar"`        // Avatar filename (relative to avatar dir)
	// Proxy overrides the global proxy for this account. The zero value follows
	// the global setting, which is what every account written before this option
	// existed gets.
	Proxy AccountProxy `yaml:"proxy,omitempty"`
}

// LogValue makes an account readable in the log. Pass the account itself to slog
// ("account", account) and appHandler flattens the group into
//
//	account.email=2300…@email.szu.edu.cn account.name=SZU
//
// instead of the UUID config.yaml keys it by, which is what a human reading
// app.log is actually looking for.
//
// It also keeps the secrets out: an EmailAccount carries IMAPPass, SMTPPass and
// OAuthRefreshToken, and logging the struct by value would print them.
func (a EmailAccount) LogValue() slog.Value {
	attrs := []slog.Attr{slog.String("email", a.Email)}
	if a.Name != "" {
		attrs = append(attrs, slog.String("name", a.Name))
	}
	return slog.GroupValue(attrs...)
}

// ConfigService provides methods for reading, writing, and managing
// the application's YAML configuration file, with encrypted API key storage.
type ConfigService struct {
	// onAccountRemoved is told about every deleted account id, after the config
	// write has landed. Wired once at startup (main.go) to drop that account's
	// IDLE connection and its sync bookkeeping; nil in tests and the server build.
	removedMu        sync.RWMutex
	onAccountRemoved func(accountID string)
}

// SetAccountRemovedListener registers the callback for account deletions. It is
// called after the config write has landed and outside configWriteMu: the callback
// belongs to other services' state, not to the config's, and it must not be able
// to extend the config's critical section.
func (s *ConfigService) SetAccountRemovedListener(listener func(accountID string)) {
	s.removedMu.Lock()
	defer s.removedMu.Unlock()
	s.onAccountRemoved = listener
}

func (s *ConfigService) accountRemovedListener() func(accountID string) {
	s.removedMu.RLock()
	defer s.removedMu.RUnlock()
	return s.onAccountRemoved
}

func defaultConfig() AppConfig {
	return AppConfig{
		Language:          "en-US",
		Theme:             "System",
		PrimaryColor:      "#e11d48",
		DatabasePath:      "",
		MinimizeWhenClose: false,
		LogConfig: LogConfig{
			Level: "info",
			// 0 = never remove expired logs (the default option).
			RetentionDays: 0,
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

// SaveLogConfig updates the log settings in the config file. The retention
// value is normalized onto the allowed options (0 = never remove expired logs).
func (s *ConfigService) SaveLogConfig(cfg LogConfig) error {
	cfg.RetentionDays = normalizeRetentionDays(cfg.RetentionDays)
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
	if err := validateAccountProxy(account); err != nil {
		return err
	}
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	cfg.EmailAccounts = append(cfg.EmailAccounts, account)
	return writeConfig(cfg)
}

// validateAccountProxy rejects a mode this build does not know, and a custom
// address that cannot be parsed.
//
// Checked at save time on purpose: a bad address that is only discovered when
// the next mail operation dials turns into "this account is broken" with no
// visible cause, and the account form is where the user can still fix it.
func validateAccountProxy(account EmailAccount) error {
	switch account.Proxy.Mode {
	case ProxyModeFollow, ProxyModeOff:
		return nil
	case ProxyModeCustom:
		u, err := parseProxyURL(account.Proxy.URL)
		if err != nil {
			return err
		}
		if u == nil {
			return fmt.Errorf("account proxy: no address given")
		}
		return nil
	}
	return fmt.Errorf("account proxy: unknown mode %q", account.Proxy.Mode)
}

// RemoveAccount removes an email account by its ID.
func (s *ConfigService) RemoveAccount(id string) error {
	if err := removeAccountFromConfig(id); err != nil {
		return err
	}
	// Outside configWriteMu: the listener stops that account's IDLE watcher and
	// drops its sync bookkeeping — other services' state, which a config write has
	// no business waiting on.
	if listener := s.accountRemovedListener(); listener != nil {
		listener(id)
	}
	return nil
}

// removeAccountFromConfig performs the read-modify-write. Callers must not hold
// configWriteMu; it is taken here, like every other mutation.
func removeAccountFromConfig(id string) error {
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
	if err := validateAccountProxy(account); err != nil {
		return err
	}
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

// MoveAccount moves one email account by `offset` slots in the configured order,
// which is the order the settings list and the sidebar render in.
//
// offset is -1 (up) or +1 (down). A move past either end is a no-op instead of an
// error, so the caller does not have to re-check the bounds it just rendered.
// Only the slice order changes: the entries themselves are read decrypted and
// written back re-encrypted by readConfig/writeConfig like every other mutation.
func (s *ConfigService) MoveAccount(id string, offset int) error {
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
	target := idx + offset
	if offset == 0 || target < 0 || target >= len(cfg.EmailAccounts) {
		return nil
	}
	accounts := cfg.EmailAccounts
	moved := accounts[idx]
	// Shift the entries between the old and the new slot over by one, then drop
	// the moved account into the hole. copy() handles the overlap correctly for
	// both directions, which a naive swap loop would not.
	if offset > 0 {
		copy(accounts[idx:target], accounts[idx+1:target+1])
	} else {
		copy(accounts[target+1:idx+1], accounts[target:idx])
	}
	accounts[target] = moved
	return writeConfig(cfg)
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
