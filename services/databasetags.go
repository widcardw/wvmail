package services

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Rule-based mail tags and per-message overrides.

func (m *DBManager) LoadMailTags() ([]CachedMailTag, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var tags []CachedMailTag
	if err := db.Order("created_at ASC").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (m *DBManager) SaveMailTag(tag *CachedMailTag) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Save(tag).Error
}

func (m *DBManager) DeleteMailTag(id string) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id = ?", id).Delete(&EmailTagOverride{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&CachedMailTag{}).Error
	})
}

func (m *DBManager) LoadTaggingEmails() ([]CachedEmail, error) {
	return m.LoadTaggingEmailsForScopes(nil)
}

func (m *DBManager) LoadTaggingEmailsForScopes(scopes []EmailTagFilterScope) ([]CachedEmail, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var emails []CachedEmail
	if err := scopeTagQuery(db, scopes).Order("uid DESC").Find(&emails).Error; err != nil {
		return nil, err
	}
	return emails, nil
}

func (m *DBManager) LoadCachedEmail(accountID, folderName string, uid uint32) (*CachedEmail, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var email CachedEmail
	if err := db.Where(
		"account_id = ? AND folder_name = ? AND uid = ?",
		accountID, folderName, uid,
	).First(&email).Error; err != nil {
		return nil, err
	}
	return &email, nil
}

func (m *DBManager) LoadTaggingBodies() ([]CachedEmailBody, error) {
	return m.LoadTaggingBodiesForScopes(nil)
}

func (m *DBManager) LoadTaggingBodiesForScopes(scopes []EmailTagFilterScope) ([]CachedEmailBody, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var bodies []CachedEmailBody
	if err := scopeTagQuery(db, scopes).
		Select("account_id", "folder_name", "uid", "body_text").
		Find(&bodies).Error; err != nil {
		return nil, err
	}
	return bodies, nil
}

func (m *DBManager) LoadEmailTagOverrides() ([]EmailTagOverride, error) {
	return m.LoadEmailTagOverridesForScopes(nil)
}

func (m *DBManager) LoadEmailTagOverridesForScopes(scopes []EmailTagFilterScope) ([]EmailTagOverride, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var overrides []EmailTagOverride
	if err := scopeTagQuery(db, scopes).Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
}

// scopeTagQuery limits a tag-related table read to the account/folder pairs the
// current view can contain. An empty scope list deliberately means "all cached
// mail"; callers such as the Starred view pair that with a flag filter.
func scopeTagQuery(db *gorm.DB, scopes []EmailTagFilterScope) *gorm.DB {
	if len(scopes) == 0 {
		return db
	}
	clauses := make([]string, 0, len(scopes))
	args := make([]any, 0, len(scopes)*2)
	for _, scope := range scopes {
		clauses = append(clauses, "(account_id = ? AND folder_name = ?)")
		args = append(args, scope.AccountID, scope.FolderName)
	}
	return db.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func (m *DBManager) ReplaceEmailTagOverrides(
	accountID, folderName string,
	uid uint32,
	overrides []EmailTagOverride,
) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"account_id = ? AND folder_name = ? AND uid = ?",
			accountID, folderName, uid,
		).Delete(&EmailTagOverride{}).Error; err != nil {
			return err
		}
		if len(overrides) == 0 {
			return nil
		}
		if err := tx.Create(&overrides).Error; err != nil {
			return fmt.Errorf("save email tag overrides: %w", err)
		}
		return nil
	})
}
