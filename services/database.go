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
	Subject    string `gorm:"size:512"`
	From       string `gorm:"size:256"` // display "Name <email>"
	FromName   string `gorm:"size:128"`
	FromAddr   string `gorm:"size:256"`
	To         string `gorm:"type:text"` // JSON serialized []string
	Date       string `gorm:"size:64"`
	Flags      string `gorm:"size:512"` // JSON serialized []string
	IsRead     bool
}

// CachedEmailBody represents a cached full email body content.
type CachedEmailBody struct {
	ID          uint   `gorm:"primaryKey"`
	AccountID   string `gorm:"index:idx_ceb_account_folder_uid;size:64;not null"`
	FolderName  string `gorm:"size:255;not null"`
	UID         uint32 `gorm:"not null"`
	Subject     string `gorm:"size:512"`
	From        string `gorm:"size:256"` // display "Name <email>"
	FromName    string `gorm:"size:128"`
	FromAddr    string `gorm:"size:256"`
	To          string `gorm:"size:512"` // JSON serialized []string
	Cc          string `gorm:"size:512"` // JSON serialized []string
	Date        string `gorm:"size:64"`
	BodyText    string `gorm:"type:text"`
	BodyHTML    string `gorm:"type:text"`
	Flags       string `gorm:"size:512"`  // JSON serialized []string
	Attachments string `gorm:"type:text"` // JSON serialized []AttachmentInfo
}

// CachedNote represents a persisted note with rich text content.
type CachedNote struct {
	ID        string `gorm:"primaryKey;size:64"`
	Title     string `gorm:"size:255"`
	Content   string `gorm:"type:text"`
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

	if err := m.db.AutoMigrate(&CachedFolder{}, &CachedEmail{}, &CachedEmailBody{}, &CachedNote{}, &CachedFolderMeta{}, &CachedReminder{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return m.db, nil
}
