package services

import (
	"fmt"

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
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var emails []CachedEmail
	if err := db.Order("uid DESC").Find(&emails).Error; err != nil {
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
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var bodies []CachedEmailBody
	if err := db.Select("account_id", "folder_name", "uid", "body_text").Find(&bodies).Error; err != nil {
		return nil, err
	}
	return bodies, nil
}

func (m *DBManager) LoadEmailTagOverrides() ([]EmailTagOverride, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var overrides []EmailTagOverride
	if err := db.Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
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
