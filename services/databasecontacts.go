package services

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Cached contacts.

// SaveContact creates or updates a local address-book entry.
func (m *DBManager) SaveContact(contact *CachedContact) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Save(contact).Error
}

// LoadContacts retrieves every saved contact in display order.
func (m *DBManager) LoadContacts() ([]CachedContact, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var contacts []CachedContact
	if err := db.Order("LOWER(name) ASC, LOWER(email) ASC").Find(&contacts).Error; err != nil {
		return nil, err
	}
	return contacts, nil
}

// LoadContactByEmail finds one contact by its normalized address.
func (m *DBManager) LoadContactByEmail(email string) (*CachedContact, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var contact CachedContact
	if err := db.Where("email = ?", email).First(&contact).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &contact, nil
}

// LoadContactByID finds one contact by its primary key.
func (m *DBManager) LoadContactByID(id string) (*CachedContact, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var contact CachedContact
	if err := db.Where("id = ?", id).First(&contact).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &contact, nil
}

// DeleteContact removes a contact by ID.
func (m *DBManager) DeleteContact(id string) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Where("id = ?", id).Delete(&CachedContact{}).Error
}

// DeleteContactAndExclude removes a contact and records its address in one
// transaction. The exclusion is what stops the next incoming message from
// silently recreating a contact the user just deleted.
func (m *DBManager) DeleteContactAndExclude(id, email string) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).Delete(&CachedContact{}).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&CachedContactExclusion{Email: strings.ToLower(strings.TrimSpace(email))}).
			Error
	})
}

func (m *DBManager) AddContactExclusion(email string) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&CachedContactExclusion{Email: strings.ToLower(strings.TrimSpace(email))}).
		Error
}

func (m *DBManager) RemoveContactExclusion(email string) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Where("email = ?", strings.ToLower(strings.TrimSpace(email))).
		Delete(&CachedContactExclusion{}).
		Error
}

func (m *DBManager) LoadContactExclusionEmails() (map[string]bool, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var rows []CachedContactExclusion
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}
	excluded := make(map[string]bool, len(rows))
	for _, row := range rows {
		excluded[row.Email] = true
	}
	return excluded, nil
}

func (m *DBManager) LoadContactCollectionState(
	accountID, folderName string,
) (*CachedContactCollectionState, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var state CachedContactCollectionState
	if err := db.Where(
		"account_id = ? AND folder_name = ?",
		accountID, folderName,
	).First(&state).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (m *DBManager) SaveContactCollectionState(state *CachedContactCollectionState) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Save(state).Error
}
