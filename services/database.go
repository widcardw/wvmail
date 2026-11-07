package services

import (
	"fmt"
	"path/filepath"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The SQLite cache: the GORM row types (the schema), the manager and the
// connection/migration that opens it. The per-entity reads and writes live in
// databaseemail.go, databasebody.go, databasenotes.go and databasereminders.go.

// CachedFolder represents a cached IMAP folder/mailbox.
type CachedFolder struct {
	ID        uint   `gorm:"primaryKey"`
	AccountID string `gorm:"index:idx_cf_account;size:64;not null"`
	Name      string `gorm:"size:255;not null"`
	Delim     string `gorm:"size:4"`
	Attrs     string `gorm:"size:512"` // JSON serialized []string
}

// CachedEmail represents a cached email summary within a folder.
type CachedEmail struct {
	ID         uint   `gorm:"primaryKey"`
	AccountID  string `gorm:"index:idx_ce_account_folder;size:64;not null"`
	FolderName string `gorm:"size:255;not null"`
	UID        uint32 `gorm:"not null"`
	MessageID  string `gorm:"size:512"`
	InReplyTo  string `gorm:"type:text"` // JSON serialized []string
	References string `gorm:"type:text"` // JSON serialized []string
	Subject    string `gorm:"size:512"`
	From       string `gorm:"size:256"` // display "Name <email>"
	FromName   string `gorm:"size:128"`
	FromAddr   string `gorm:"size:256"`
	To         string `gorm:"type:text"` // JSON serialized []string
	Date       string `gorm:"size:64"`
	Flags      string `gorm:"size:512"` // JSON serialized []string
	IsRead     bool
	IsBulk     bool `gorm:"not null;default:false"`
}

// CachedEmailBody represents a cached full email body content.
type CachedEmailBody struct {
	ID         uint   `gorm:"primaryKey"`
	AccountID  string `gorm:"index:idx_ceb_account_folder_uid;size:64;not null"`
	FolderName string `gorm:"size:255;not null"`
	UID        uint32 `gorm:"not null"`
	MessageID  string `gorm:"size:512"`
	InReplyTo  string `gorm:"type:text"` // JSON serialized []string
	References string `gorm:"type:text"` // JSON serialized []string
	Subject    string `gorm:"size:512"`
	From       string `gorm:"size:256"` // display "Name <email>"
	FromName   string `gorm:"size:128"`
	FromAddr   string `gorm:"size:256"`
	To         string `gorm:"size:512"` // JSON serialized []string
	Cc         string `gorm:"size:512"` // JSON serialized []string
	Date       string `gorm:"size:64"`
	BodyText   string `gorm:"type:text"`
	BodyHTML   string `gorm:"type:text"`
	// Zero for cache rows written before this field existed. The frontend treats
	// an empty BodyHTML as plain text too, so those rows keep their old behaviour.
	BodyIsPlainText bool   `gorm:"not null;default:false"`
	Flags           string `gorm:"size:512"`  // JSON serialized []string
	Attachments     string `gorm:"type:text"` // JSON serialized []AttachmentInfo
}

// CachedNote represents a persisted note with rich text content.
type CachedNote struct {
	ID        string `gorm:"primaryKey;size:64"`
	Title     string `gorm:"size:255"`
	Content   string `gorm:"type:text"`
	CreatedAt int64  `gorm:"autoCreateTime"`
	UpdatedAt int64  `gorm:"autoUpdateTime"`
}

// CachedContact is a locally saved address-book entry. Email is stored in lower
// case and uniquely indexed so the same person cannot be added twice.
type CachedContact struct {
	ID        string `gorm:"primaryKey;size:64"`
	Name      string `gorm:"size:255"`
	Email     string `gorm:"uniqueIndex;size:320;not null"`
	CreatedAt int64  `gorm:"autoCreateTime"`
	UpdatedAt int64  `gorm:"autoUpdateTime"`
}

// CachedFolderMeta stores what one fetch saw of a folder — the message count and
// the mailbox's UIDNEXT — kept separate from the email cache so it survives
// page-by-page cache replacement.
//
// UIDNEXT is persisted rather than kept in memory because it is the reference
// point the frontend compares a later STATUS against before refreshing an opened
// folder: without it, the first open of every folder after a restart would cost a
// fetch even when nothing had arrived.
type CachedFolderMeta struct {
	AccountID  string `gorm:"primaryKey;size:64"`
	FolderName string `gorm:"primaryKey;size:255"`
	Total      int    `gorm:"not null"`
	UIDNext    uint32 `gorm:"not null;default:0"`
}

// CachedFlaggedSnapshot stores one account's last successful "All Starred"
// result. The aggregate can span hundreds of mailboxes, so keeping the finished
// list as JSON makes reopening the view a local read instead of another IMAP
// search. FetchedAt lets the UI show the snapshot immediately and refresh it in
// the background only when it has aged.
type CachedFlaggedSnapshot struct {
	AccountID string `gorm:"primaryKey;size:64"`
	Emails    string `gorm:"type:text"`
	FetchedAt int64  `gorm:"not null;default:0"`
}

// CachedReminder represents a persisted reminder. `Time` is epoch seconds, the
// unit the frontend (and the mail date detector) work in.
type CachedReminder struct {
	ID        string `gorm:"primaryKey;size:64"`
	Title     string `gorm:"size:512"`
	Time      int64  `gorm:"index"`
	Done      bool   `gorm:"not null;default:false"`
	CreatedAt int64  `gorm:"autoCreateTime"`
	UpdatedAt int64  `gorm:"autoUpdateTime"`
}

// CachedMailTag is a user-defined, rule-based mail label. Keywords and Fields are
// JSON arrays. Tags never move mail; a message may match several tags at once.
type CachedMailTag struct {
	ID        string `gorm:"primaryKey;size:64"`
	Label     string `gorm:"uniqueIndex;size:128;not null"`
	Color     string `gorm:"size:16;not null"`
	Keywords  string `gorm:"type:text;not null"`
	Fields    string `gorm:"size:128;not null"`
	Enabled   bool   `gorm:"not null;default:true"`
	CreatedAt int64  `gorm:"autoCreateTime"`
	UpdatedAt int64  `gorm:"autoUpdateTime"`
}

// EmailTagOverride records a user's manual decision that differs from the
// rule-based result for one cached message. Included=false is an explicit
// exclusion; no row means "follow the tag rules".
type EmailTagOverride struct {
	ID         uint   `gorm:"primaryKey"`
	AccountID  string `gorm:"uniqueIndex:idx_eto_message_tag;size:64;not null"`
	FolderName string `gorm:"uniqueIndex:idx_eto_message_tag;size:255;not null"`
	UID        uint32 `gorm:"uniqueIndex:idx_eto_message_tag;not null"`
	TagID      string `gorm:"uniqueIndex:idx_eto_message_tag;index;size:64;not null"`
	Included   bool   `gorm:"not null"`
}

// DBManager provides a lazily-initialized GORM database connection for
// caching IMAP data locally.
type DBManager struct {
	db  *gorm.DB
	mu  sync.Mutex
	cfg *ConfigService
}

// NewDBManager creates a new DBManager.
func NewDBManager() *DBManager {
	return &DBManager{
		cfg: &ConfigService{},
	}
}

// getDB returns the initialized GORM DB, initializing it on first call.
func (m *DBManager) getDB() (*gorm.DB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db != nil {
		return m.db, nil
	}

	// Resolve database path from config
	appCfg, err := m.cfg.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	dbPath := appCfg.DatabasePath
	if dbPath == "" {
		dir, err := configDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get config dir: %w", err)
		}
		dbPath = filepath.Join(dir, "cache.db")
	}

	m.db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := m.db.AutoMigrate(
		&CachedFolder{},
		&CachedEmail{},
		&CachedEmailBody{},
		&CachedNote{},
		&CachedContact{},
		&CachedFolderMeta{},
		&CachedFlaggedSnapshot{},
		&CachedReminder{},
		&CachedMailTag{},
		&EmailTagOverride{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return m.db, nil
}
