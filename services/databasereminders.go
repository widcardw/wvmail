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

// SetReminderNotificationAttempted records whether the notification was handled,
// preventing it from being emitted again after restarting the app.
func (m *DBManager) SetReminderNotificationAttempted(id string, attempted bool) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Model(&CachedReminder{}).
		Where("id = ?", id).
		Update("notification_attempted", attempted).Error
}

// SetReminderNotificationAttemptedAt only marks the reminder if it still has
// the scheduled time captured by the timer. This prevents a timer racing with an
// edit from marking the new schedule as already delivered.
func (m *DBManager) SetReminderNotificationAttemptedAt(id string, scheduledTime int64) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Model(&CachedReminder{}).
		Where("id = ? AND time = ? AND done = ?", id, scheduledTime, false).
		Update("notification_attempted", true).Error
}
