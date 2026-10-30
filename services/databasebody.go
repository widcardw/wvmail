package services

import (
	"encoding/json"
)

// Cached mail bodies: one serialized row per message, holding the rendered text,
// the flags and the attachment list.

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
		AccountID:       accountID,
		FolderName:      folderName,
		UID:             detail.UID,
		MessageID:       detail.MessageID,
		Subject:         detail.Subject,
		From:            detail.From,
		FromName:        detail.FromName,
		FromAddr:        detail.FromAddr,
		To:              string(toJSON),
		Cc:              string(ccJSON),
		Date:            detail.Date,
		BodyText:        detail.BodyText,
		BodyHTML:        detail.BodyHTML,
		BodyIsPlainText: detail.BodyIsPlainText,
		Flags:           string(flagsJSON),
		Attachments:     string(attachmentsJSON),
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
		UID:             cached.UID,
		MessageID:       cached.MessageID,
		Subject:         cached.Subject,
		From:            cached.From,
		FromName:        cached.FromName,
		FromAddr:        cached.FromAddr,
		To:              to,
		Cc:              cc,
		Date:            cached.Date,
		BodyText:        cached.BodyText,
		BodyHTML:        cached.BodyHTML,
		BodyIsPlainText: cached.BodyIsPlainText,
		Flags:           flags,
		Attachments:     attachments,
	}, nil
}
