package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"runtime"
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
// SQLite database and registers their scheduled native notifications.
type RemindersService struct {
	db *DBManager

	mu       sync.RWMutex
	schedule func(Reminder) error
	cancel   func(string) error
	timers   map[string]*time.Timer
	start    sync.Once
}

func NewRemindersService() *RemindersService {
	return &RemindersService{
		db:     NewDBManager(),
		timers: make(map[string]*time.Timer),
	}
}

// SetNotificationHandlers registers the native notification scheduler and
// cancellation function.
//
//wails:ignore
func (s *RemindersService) SetNotificationHandlers(
	schedule func(Reminder) error,
	cancel func(string) error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schedule = schedule
	s.cancel = cancel
}

// Start restores scheduled notifications after Wails has initialized its
// notification service.
//
//wails:ignore
func (s *RemindersService) Start() {
	s.start.Do(func() {
		if err := s.restoreNotifications(); err != nil {
			slog.Error("reminder: restoring scheduled notifications failed", "error", err)
		}
	})
}

// Stop releases in-process bookkeeping timers without canceling notifications
// already registered with the operating system.
//
//wails:ignore
func (s *RemindersService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, timer := range s.timers {
		timer.Stop()
		delete(s.timers, id)
	}
}

func (s *RemindersService) restoreNotifications() error {
	cached, err := s.db.LoadReminders()
	if err != nil {
		return fmt.Errorf("loading reminders: %w", err)
	}

	now := time.Now()
	for _, row := range cached {
		reminder := s.toReminder(row)
		if row.NotificationAttempted == nil {
			// Existing upcoming reminders predate the notification marker and
			// should still be scheduled. Do not replay old, overdue reminders.
			attempted := row.Time <= now.Unix()
			if err := s.db.SetReminderNotificationAttempted(row.ID, attempted); err != nil {
				slog.Error("reminder: failed to migrate notification state",
					"reminder_id", row.ID, "error", err)
				continue
			}
			reminder.NotificationAttempted = attempted
		}

		if reminder.Done || reminder.NotificationAttempted {
			s.cancelReminderNotification(reminder.ID)
			continue
		}

		if reminder.Time <= now.Unix() && runtime.GOOS == "darwin" {
			// macOS keeps native requests while the app is closed and delivers
			// them itself. A past pending request has therefore already fired.
			if err := s.db.SetReminderNotificationAttempted(reminder.ID, true); err != nil {
				slog.Error("reminder: failed to record delivered notification",
					"reminder_id", reminder.ID, "error", err)
			}
			continue
		}

		if err := s.syncReminderNotification(reminder); err != nil {
			slog.Warn("reminder: failed to restore scheduled notification",
				"reminder_id", reminder.ID, "error", err)
		}
	}
	return nil
}

// SaveReminder creates or updates a reminder. An empty ID means "new": one is
// generated, so callers that just have a title and a time can save directly.
func (s *RemindersService) SaveReminder(reminder Reminder) (Reminder, error) {
	if reminder.Title == "" {
		return reminder, fmt.Errorf("reminder title is required")
	}

	var previous *CachedReminder
	if reminder.ID != "" {
		rows, err := s.db.LoadReminders()
		if err != nil {
			return reminder, fmt.Errorf("failed to load existing reminders: %w", err)
		}
		for i := range rows {
			if rows[i].ID == reminder.ID {
				previous = &rows[i]
				break
			}
		}
	} else {
		id, err := newReminderID()
		if err != nil {
			return reminder, fmt.Errorf("failed to generate reminder ID: %w", err)
		}
		reminder.ID = id
	}

	attempted := reminder.Time <= time.Now().Unix()
	if previous != nil && previous.NotificationAttempted != nil {
		attempted = *previous.NotificationAttempted
		if reminder.Time != previous.Time && reminder.Time > time.Now().Unix() {
			attempted = false
		}
	}

	cr := CachedReminder{
		ID:                    reminder.ID,
		Title:                 reminder.Title,
		Time:                  reminder.Time,
		Done:                  reminder.Done,
		NotificationAttempted: &attempted,
	}
	if err := s.db.SaveReminder(&cr); err != nil {
		return reminder, fmt.Errorf("failed to save reminder: %w", err)
	}
	saved := s.toReminder(cr)
	if err := s.syncReminderNotification(saved); err != nil {
		slog.Warn("reminder: saved reminder but failed to update its notification",
			"reminder_id", saved.ID, "error", err)
	}
	return saved, nil
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

// DeleteReminder removes a reminder by ID and cancels its pending notification.
func (s *RemindersService) DeleteReminder(id string) error {
	if err := s.db.DeleteReminder(id); err != nil {
		return fmt.Errorf("failed to delete reminder: %w", err)
	}
	s.cancelReminderNotification(id)
	return nil
}

// SetReminderDone marks a reminder done (or not done) and updates its
// notification registration accordingly.
func (s *RemindersService) SetReminderDone(id string, done bool) error {
	if id == "" {
		return fmt.Errorf("reminder ID is required")
	}
	if err := s.db.SetReminderDone(id, done); err != nil {
		return fmt.Errorf("failed to update reminder: %w", err)
	}

	rows, err := s.db.LoadReminders()
	if err != nil {
		return fmt.Errorf("failed to load updated reminder: %w", err)
	}
	for _, row := range rows {
		if row.ID == id {
			if err := s.syncReminderNotification(s.toReminder(row)); err != nil {
				slog.Warn("reminder: updated completion state but failed to update notification",
					"reminder_id", id, "error", err)
			}
			break
		}
	}
	return nil
}

func (s *RemindersService) syncReminderNotification(reminder Reminder) error {
	s.stopTimer(reminder.ID)
	s.mu.RLock()
	schedule := s.schedule
	cancel := s.cancel
	s.mu.RUnlock()
	if cancel != nil {
		if err := cancel(reminder.ID); err != nil {
			return fmt.Errorf("cancel existing notification: %w", err)
		}
	}

	if reminder.Done || reminder.NotificationAttempted || schedule == nil {
		return nil
	}
	if err := schedule(reminder); err != nil {
		return fmt.Errorf("schedule notification: %w", err)
	}

	if reminder.Time <= time.Now().Unix() {
		if err := s.db.SetReminderNotificationAttempted(reminder.ID, true); err != nil {
			return fmt.Errorf("record immediate notification: %w", err)
		}
		return nil
	}

	delay := time.Until(time.Unix(reminder.Time, 0))
	if delay <= 0 {
		return nil
	}
	var timer *time.Timer
	timer = time.AfterFunc(delay, func() {
		s.mu.Lock()
		if s.timers[reminder.ID] != timer {
			s.mu.Unlock()
			return
		}
		delete(s.timers, reminder.ID)
		s.mu.Unlock()

		if err := s.db.SetReminderNotificationAttemptedAt(reminder.ID, reminder.Time); err != nil {
			slog.Error("reminder: failed to record notification delivery",
				"reminder_id", reminder.ID, "error", err)
		}
	})
	s.mu.Lock()
	if existing := s.timers[reminder.ID]; existing != nil {
		existing.Stop()
	}
	s.timers[reminder.ID] = timer
	s.mu.Unlock()
	return nil
}

func (s *RemindersService) cancelReminderNotification(id string) {
	s.stopTimer(id)
	s.mu.RLock()
	cancel := s.cancel
	s.mu.RUnlock()
	if cancel == nil {
		return
	}
	if err := cancel(id); err != nil {
		slog.Warn("reminder: failed to cancel pending notification",
			"reminder_id", id, "error", err)
	}
}

func (s *RemindersService) stopTimer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if timer := s.timers[id]; timer != nil {
		timer.Stop()
		delete(s.timers, id)
	}
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
