package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// Reminder is a single reminder exposed to the frontend.
type Reminder struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Epoch seconds — the project-wide timestamp convention, and what the mail date
	// detector produces when a user turns a highlighted moment into a reminder.
	Time      int64 `json:"time"`
	Done      bool  `json:"done"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// RemindersService provides CRUD operations for reminders stored in the local
// SQLite database.
type RemindersService struct {
	db *DBManager
}

func NewRemindersService() *RemindersService {
	return &RemindersService{
		db: NewDBManager(),
	}
}

// SaveReminder creates or updates a reminder. An empty ID means "new": one is
// generated, so callers that just have a title and a time can save directly.
func (s *RemindersService) SaveReminder(reminder Reminder) (Reminder, error) {
	if reminder.Title == "" {
		return reminder, fmt.Errorf("reminder title is required")
	}
	if reminder.ID == "" {
		id, err := newReminderID()
		if err != nil {
			return reminder, fmt.Errorf("failed to generate reminder ID: %w", err)
		}
		reminder.ID = id
	}

	cr := CachedReminder{
		ID:    reminder.ID,
		Title: reminder.Title,
		Time:  reminder.Time,
		Done:  reminder.Done,
	}
	if err := s.db.SaveReminder(&cr); err != nil {
		return reminder, fmt.Errorf("failed to save reminder: %w", err)
	}
	return s.toReminder(cr), nil
}

// LoadReminders returns every reminder, soonest first.
func (s *RemindersService) LoadReminders() ([]Reminder, error) {
	cached, err := s.db.LoadReminders()
	if err != nil {
		return nil, fmt.Errorf("failed to load reminders: %w", err)
	}
	reminders := make([]Reminder, len(cached))
	for i, cr := range cached {
		reminders[i] = s.toReminder(cr)
	}
	return reminders, nil
}

// DeleteReminder removes a reminder by ID.
func (s *RemindersService) DeleteReminder(id string) error {
	if err := s.db.DeleteReminder(id); err != nil {
		return fmt.Errorf("failed to delete reminder: %w", err)
	}
	return nil
}

// SetReminderDone marks a reminder done (or not done).
func (s *RemindersService) SetReminderDone(id string, done bool) error {
	if id == "" {
		return fmt.Errorf("reminder ID is required")
	}
	if err := s.db.SetReminderDone(id, done); err != nil {
		return fmt.Errorf("failed to update reminder: %w", err)
	}
	return nil
}

func (s *RemindersService) toReminder(cr CachedReminder) Reminder {
	return Reminder{
		ID:        cr.ID,
		Title:     cr.Title,
		Time:      cr.Time,
		Done:      cr.Done,
		CreatedAt: cr.CreatedAt,
		UpdatedAt: cr.UpdatedAt,
	}
}

func newReminderID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
