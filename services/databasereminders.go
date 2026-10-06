package services

// Cached reminders.

// SaveReminder creates or updates a reminder.
func (m *DBManager) SaveReminder(reminder *CachedReminder) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Save(reminder).Error
}

// LoadReminders retrieves every reminder, soonest first.
func (m *DBManager) LoadReminders() ([]CachedReminder, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}
	var reminders []CachedReminder
	if err := db.Order("time ASC").Find(&reminders).Error; err != nil {
		return nil, err
	}
	return reminders, nil
}

// DeleteReminder removes a reminder by ID.
func (m *DBManager) DeleteReminder(id string) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Where("id = ?", id).Delete(&CachedReminder{}).Error
}

// SetReminderDone flips the done flag of a single reminder.
func (m *DBManager) SetReminderDone(id string, done bool) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Model(&CachedReminder{}).Where("id = ?", id).Update("done", done).Error
}
