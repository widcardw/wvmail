package services

import "gorm.io/gorm"

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
