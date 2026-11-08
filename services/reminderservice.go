package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Reminder is a single reminder exposed to the frontend.
type Reminder struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Epoch seconds — the project-wide timestamp convention, and what the mail date
	// detector produces when a user turns a highlighted moment into a reminder.
	Time                  int64 `json:"time"`
	Done                  bool  `json:"done"`
	NotificationAttempted bool  `json:"-"`
	CreatedAt             int64 `json:"created_at"`
	UpdatedAt             int64 `json:"updated_at"`
}

// RemindersService provides CRUD operations for reminders stored in the local
// SQLite database.
type RemindersService struct {
	db       *DBManager
	mu       sync.RWMutex
	notifier func(Reminder) error
	stop     chan struct{}
	start    sync.Once
	stopOnce sync.Once
}

func NewRemindersService() *RemindersService {
	return &RemindersService{
		db:   NewDBManager(),
		stop: make(chan struct{}),
	}
}

// SetNotifier registers the native notification sender used for due reminders.
//
//wails:ignore
func (s *RemindersService) SetNotifier(notifier func(Reminder) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifier = notifier
}

// Start checks for due reminders immediately and then every 15 seconds.
//
//wails:ignore
func (s *RemindersService) Start() {
	s.start.Do(func() {
		go func() {
			s.dispatchDue(time.Now())
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.dispatchDue(time.Now())
				case <-s.stop:
					return
				}
			}
		}()
	})
}

// Stop stops the due-reminder scheduler.
//
//wails:ignore
func (s *RemindersService) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

func (s *RemindersService) dispatchDue(now time.Time) {
	cached, err := s.db.LoadReminders()
	if err != nil {
		slog.Error("reminder: loading reminders for notifications failed", "error", err)
		return
	}
	s.mu.RLock()
	notifier := s.notifier
	s.mu.RUnlock()
	if notifier == nil {
		return
	}

	for _, reminder := range cached {
		if reminder.Done {
			continue
		}
		// Rows created before notification support have a NULL marker. Preserve
		// upcoming reminders, but do not replay already-overdue legacy reminders.
		if reminder.NotificationAttempted == nil {
			attempted := reminder.Time <= now.Unix()
			if err := s.db.SetReminderNotificationAttempted(reminder.ID, attempted); err != nil {
				slog.Error("reminder: failed to mark legacy reminder as handled",
					"reminder_id", reminder.ID, "error", err)
			}
			continue
		}
		if *reminder.NotificationAttempted || reminder.Time > now.Unix() {
			continue
		}
		if err := notifier(s.toReminder(reminder)); err != nil {
			slog.Warn("reminder: failed to send due notification",
				"reminder_id", reminder.ID, "error", err)
		}
		if err := s.db.SetReminderNotificationAttempted(reminder.ID, true); err != nil {
			slog.Error("reminder: failed to record notification attempt",
				"reminder_id", reminder.ID, "error", err)
		}
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

	attempted := reminder.NotificationAttempted || reminder.Time <= time.Now().Unix()
	cr := CachedReminder{
		ID:                    reminder.ID,
		Title:                 reminder.Title,
		Time:                  reminder.Time,
		Done:                  reminder.Done,
		NotificationAttempted: &attempted,
	}
	if !reminder.NotificationAttempted && reminder.Time <= time.Now().Unix() {
		// Date mentions that are already in the past can be saved as notes, but
		// must not fire immediately as notifications.
		reminder.NotificationAttempted = true
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
		ID:                    cr.ID,
		Title:                 cr.Title,
		Time:                  cr.Time,
		Done:                  cr.Done,
		NotificationAttempted: cr.NotificationAttempted != nil && *cr.NotificationAttempted,
		CreatedAt:             cr.CreatedAt,
		UpdatedAt:             cr.UpdatedAt,
	}
}

func newReminderID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
