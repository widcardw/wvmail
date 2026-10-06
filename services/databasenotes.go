package services

// Cached notes.

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
