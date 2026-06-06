package services

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
	From       string `gorm:"size:256"`  // display "Name <email>"
	FromName   string `gorm:"size:128"`
	FromAddr   string `gorm:"size:256"`
	Date       string `gorm:"size:64"`
	Flags      string `gorm:"size:512"` // JSON serialized []string
	IsRead     bool
}

// CachedEmailBody represents a cached full email body content.
type CachedEmailBody struct {
	ID        uint   `gorm:"primaryKey"`
	AccountID string `gorm:"index:idx_ceb_account_folder_uid;size:64;not null"`
	FolderName string `gorm:"size:255;not null"`
	UID       uint32 `gorm:"not null"`
	Subject   string `gorm:"size:512"`
	From      string `gorm:"size:256"`  // display "Name <email>"
	FromName  string `gorm:"size:128"`
	FromAddr  string `gorm:"size:256"`
	To        string `gorm:"size:512"`  // JSON serialized []string
	Cc        string `gorm:"size:512"`  // JSON serialized []string
	Date      string `gorm:"size:64"`
	BodyText  string `gorm:"type:text"`
	BodyHTML  string `gorm:"type:text"`
	Flags     string `gorm:"size:512"` // JSON serialized []string
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

	if err := m.db.AutoMigrate(&CachedFolder{}, &CachedEmail{}, &CachedEmailBody{}, &CachedNote{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return m.db, nil
}

// SaveFolders replaces all cached folders for an account.
func (m *DBManager) SaveFolders(accountID string, folders []FolderInfo) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}

	// Delete existing folders for this account
	if err := db.Where("account_id = ?", accountID).Delete(&CachedFolder{}).Error; err != nil {
		return err
	}

	// Insert new folders
	for _, f := range folders {
		attrsJSON, _ := json.Marshal(f.Attrs)
		cf := CachedFolder{
			AccountID: accountID,
			Name:      f.Name,
			Delim:     f.Delim,
			Attrs:     string(attrsJSON),
		}
		if err := db.Create(&cf).Error; err != nil {
			return fmt.Errorf("failed to save folder %q: %w", f.Name, err)
		}
	}
	return nil
}

// LoadFolders retrieves cached folders for an account.
func (m *DBManager) LoadFolders(accountID string) ([]FolderInfo, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}

	var cached []CachedFolder
	if err := db.Where("account_id = ?", accountID).
		Order("id ASC").
		Find(&cached).Error; err != nil {
		return nil, err
	}

	if len(cached) == 0 {
		return nil, nil
	}

	folders := make([]FolderInfo, 0, len(cached))
	for _, c := range cached {
		var attrs []string
		if c.Attrs != "" {
			json.Unmarshal([]byte(c.Attrs), &attrs)
		}
		folders = append(folders, FolderInfo{
			Name:  c.Name,
			Delim: c.Delim,
			Attrs: attrs,
		})
	}
	return folders, nil
}

// SaveEmails replaces all cached email summaries for a (account, folder).
func (m *DBManager) SaveEmails(accountID, folderName string, emails []EmailSummary) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}

	// Delete existing emails for this account+folder
	if err := db.Where("account_id = ? AND folder_name = ?", accountID, folderName).
		Delete(&CachedEmail{}).Error; err != nil {
		return err
	}

	// Insert new emails
	for _, e := range emails {
		flagsJSON, _ := json.Marshal(e.Flags)
		ce := CachedEmail{
			AccountID:  accountID,
			FolderName: folderName,
			UID:        e.UID,
			Subject:    e.Subject,
			From:       e.From,
			FromName:   e.FromName,
			FromAddr:   e.FromAddr,
			Date:       e.Date,
			Flags:      string(flagsJSON),
			IsRead:     e.IsRead,
		}
		if err := db.Create(&ce).Error; err != nil {
			return fmt.Errorf("failed to save email uid=%d: %w", e.UID, err)
		}
	}
	return nil
}

// LoadEmails retrieves cached email summaries for a (account, folder).
func (m *DBManager) LoadEmails(accountID, folderName string) ([]EmailSummary, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}

	var cached []CachedEmail
	if err := db.Where("account_id = ? AND folder_name = ?", accountID, folderName).
		Order("date DESC").
		Find(&cached).Error; err != nil {
		return nil, err
	}

	if len(cached) == 0 {
		return nil, nil
	}

	emails := make([]EmailSummary, 0, len(cached))
	for _, c := range cached {
		var flags []string
		if c.Flags != "" {
			json.Unmarshal([]byte(c.Flags), &flags)
		}
		emails = append(emails, EmailSummary{
			UID:      c.UID,
			Subject:  c.Subject,
			From:     c.From,
			FromName: c.FromName,
			FromAddr: c.FromAddr,
			Date:     c.Date,
			Flags:    flags,
			IsRead:   c.IsRead,
		})
	}
	return emails, nil
}

// SaveEmailBody caches the full content of a single email.
func (m *DBManager) SaveEmailBody(accountID, folderName string, detail *EmailDetail) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}

	toJSON, _ := json.Marshal(detail.To)
	ccJSON, _ := json.Marshal(detail.Cc)
	flagsJSON, _ := json.Marshal(detail.Flags)
	attachmentsJSON, _ := json.Marshal(detail.Attachments)

	body := CachedEmailBody{
		AccountID:   accountID,
		FolderName:  folderName,
		UID:         detail.UID,
		Subject:     detail.Subject,
		From:        detail.From,
		FromName:    detail.FromName,
		FromAddr:    detail.FromAddr,
		To:          string(toJSON),
		Cc:          string(ccJSON),
		Date:        detail.Date,
		BodyText:    detail.BodyText,
		BodyHTML:    detail.BodyHTML,
		Flags:       string(flagsJSON),
		Attachments: string(attachmentsJSON),
	}

	if err := db.Where("account_id = ? AND folder_name = ? AND uid = ?",
		accountID, folderName, detail.UID).
		Delete(&CachedEmailBody{}).Error; err != nil {
		return err
	}
	return db.Create(&body).Error
}

// LoadEmailBody retrieves a cached full email body.
func (m *DBManager) LoadEmailBody(accountID, folderName string, uid uint32) (*EmailDetail, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}

	var cached CachedEmailBody
	if err := db.Where("account_id = ? AND folder_name = ? AND uid = ?",
		accountID, folderName, uid).First(&cached).Error; err != nil {
		return nil, err
	}

	var to, cc, flags []string
	json.Unmarshal([]byte(cached.To), &to)
	json.Unmarshal([]byte(cached.Cc), &cc)
	json.Unmarshal([]byte(cached.Flags), &flags)

	var attachments []AttachmentInfo
	if cached.Attachments != "" {
		json.Unmarshal([]byte(cached.Attachments), &attachments)
	}
	if attachments == nil {
		attachments = []AttachmentInfo{}
	}

	return &EmailDetail{
		UID:         cached.UID,
		Subject:     cached.Subject,
		From:        cached.From,
		FromName:    cached.FromName,
		FromAddr:    cached.FromAddr,
		To:          to,
		Cc:          cc,
		Date:        cached.Date,
		BodyText:    cached.BodyText,
		BodyHTML:    cached.BodyHTML,
		Flags:       flags,
		Attachments: attachments,
	}, nil
}

// SaveNote creates or updates a note.
func (m *DBManager) SaveNote(note *CachedNote) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Save(note).Error
}

// LoadNotes retrieves all notes ordered by updated_at descending.
func (m *DBManager) LoadNotes() ([]CachedNote, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var notes []CachedNote
	if err := db.Order("updated_at DESC").Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// LoadNote retrieves a single note by ID.
func (m *DBManager) LoadNote(id string) (*CachedNote, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var note CachedNote
	if err := db.Where("id = ?", id).First(&note).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

// DeleteNote removes a note by ID.
func (m *DBManager) DeleteNote(id string) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Where("id = ?", id).Delete(&CachedNote{}).Error
}
