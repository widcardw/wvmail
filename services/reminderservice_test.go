package services

import (
	"testing"
	"time"
)

// Reminders are created from the reminder page and from a date clicked inside an
// email body. The service owns ID generation so both paths can just send a title
// and an epoch time; the list must come back ordered so the UI can render it as-is.
func TestReminderServiceRoundTrip(t *testing.T) {
	// Keep the test away from the real ~/Library/Application Support/wvmail cache.
	t.Setenv("HOME", t.TempDir())

	svc := NewRemindersService()

	later, err := svc.SaveReminder(Reminder{Title: "评审", Time: 1_800_000_000})
	if err != nil {
		t.Fatalf("SaveReminder(later): %v", err)
	}
	if later.ID == "" {
		t.Error("SaveReminder must assign an ID when none was given")
	}

	sooner, err := svc.SaveReminder(Reminder{Title: "站会", Time: 1_700_000_000})
	if err != nil {
		t.Fatalf("SaveReminder(sooner): %v", err)
	}

	got, err := svc.LoadReminders()
	if err != nil {
		t.Fatalf("LoadReminders: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(reminders) = %d, want 2", len(got))
	}
	if got[0].ID != sooner.ID || got[1].ID != later.ID {
		t.Errorf("reminders must be ordered by time ascending, got %q then %q", got[0].Title, got[1].Title)
	}
	if got[0].Time != 1_700_000_000 || got[0].Done {
		t.Errorf("round-trip lost fields: %+v", got[0])
	}

	// Ticking a reminder off is a targeted update: the other row must not move.
	if err := svc.SetReminderDone(sooner.ID, true); err != nil {
		t.Fatalf("SetReminderDone: %v", err)
	}
	got, err = svc.LoadReminders()
	if err != nil {
		t.Fatalf("LoadReminders after toggle: %v", err)
	}
	if !got[0].Done {
		t.Error("toggled reminder must be done")
	}
	if got[1].Done {
		t.Error("other reminder must stay untouched")
	}

	// Saving an existing ID updates in place instead of creating a second row.
	if _, err := svc.SaveReminder(Reminder{ID: sooner.ID, Title: "站会（改）", Time: 1_900_000_000, Done: true}); err != nil {
		t.Fatalf("SaveReminder(update): %v", err)
	}
	got, err = svc.LoadReminders()
	if err != nil {
		t.Fatalf("LoadReminders after update: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("update must not insert a row, len = %d", len(got))
	}
	// The edited time now sorts after the other one.
	if got[0].Title != "评审" || got[1].Title != "站会（改）" {
		t.Errorf("update did not reorder correctly: %+v", got)
	}

	if err := svc.DeleteReminder(sooner.ID); err != nil {
		t.Fatalf("DeleteReminder: %v", err)
	}
	got, err = svc.LoadReminders()
	if err != nil {
		t.Fatalf("LoadReminders after delete: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len(reminders) = %d, want 1", len(got))
	}
}

func TestReminderServiceValidation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := NewRemindersService()

	if _, err := svc.SaveReminder(Reminder{Time: 1}); err == nil {
		t.Error("a reminder without a title must be rejected")
	}
	if err := svc.SetReminderDone("", true); err == nil {
		t.Error("SetReminderDone without an ID must be rejected")
	}
}

func TestReminderNotificationsScheduleAndCancel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := NewRemindersService()
	defer svc.Stop()
	var scheduled []Reminder
	var canceled []string
	svc.SetNotificationHandlers(func(reminder Reminder) error {
		scheduled = append(scheduled, reminder)
		return nil
	}, func(id string) error {
		canceled = append(canceled, id)
		return nil
	})

	reminder, err := svc.SaveReminder(Reminder{
		Title: "评审",
		Time:  time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("SaveReminder: %v", err)
	}
	if len(scheduled) != 1 || scheduled[0].ID != reminder.ID {
		t.Fatalf("scheduled = %+v, want reminder %q", scheduled, reminder.ID)
	}

	if err := svc.SetReminderDone(reminder.ID, true); err != nil {
		t.Fatalf("SetReminderDone(true): %v", err)
	}
	if len(canceled) != 2 || canceled[1] != reminder.ID {
		t.Fatalf("canceled = %v, want reminder %q", canceled, reminder.ID)
	}
	if len(scheduled) != 1 {
		t.Fatalf("completed reminder was rescheduled: %+v", scheduled)
	}

	if err := svc.SetReminderDone(reminder.ID, false); err != nil {
		t.Fatalf("SetReminderDone(false): %v", err)
	}
	if len(scheduled) != 2 || scheduled[1].ID != reminder.ID {
		t.Fatalf("reopened reminder was not rescheduled: %+v", scheduled)
	}

	if err := svc.DeleteReminder(reminder.ID); err != nil {
		t.Fatalf("DeleteReminder: %v", err)
	}
	if len(canceled) != 4 || canceled[3] != reminder.ID {
		t.Fatalf("canceled = %v, want final cancellation for %q", canceled, reminder.ID)
	}
}

func TestPastReminderDoesNotNotifyAndLegacyReminderIsNotReplayed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := NewRemindersService()
	defer svc.Stop()
	past, err := svc.SaveReminder(Reminder{Title: "过去的提醒", Time: time.Now().Unix() - 1})
	if err != nil {
		t.Fatalf("SaveReminder(past): %v", err)
	}
	if !past.NotificationAttempted {
		t.Fatal("saving a reminder already in the past should suppress its notification")
	}

	legacyPast := CachedReminder{ID: "legacy-past", Title: "旧提醒", Time: time.Now().Unix() - 60}
	legacyFuture := CachedReminder{ID: "legacy-future", Title: "未来旧提醒", Time: time.Now().Unix() + 3600}
	if err := svc.db.SaveReminder(&legacyPast); err != nil {
		t.Fatalf("SaveReminder(legacy past): %v", err)
	}
	if err := svc.db.SaveReminder(&legacyFuture); err != nil {
		t.Fatalf("SaveReminder(legacy future): %v", err)
	}

	var scheduled []string
	svc.SetNotificationHandlers(func(reminder Reminder) error {
		scheduled = append(scheduled, reminder.ID)
		return nil
	}, func(string) error {
		return nil
	})
	svc.Start()
	svc.Start()
	if len(scheduled) != 1 || scheduled[0] != legacyFuture.ID {
		t.Fatalf("scheduled %v reminders, want one registration for future legacy reminder %q", scheduled, legacyFuture.ID)
	}
	migrated, err := svc.db.LoadReminders()
	if err != nil {
		t.Fatalf("LoadReminders after legacy migration: %v", err)
	}
	for _, reminder := range migrated {
		if reminder.ID == legacyFuture.ID && (reminder.NotificationAttempted == nil || *reminder.NotificationAttempted) {
			t.Fatalf("upcoming legacy reminder should become eligible, got %+v", reminder.NotificationAttempted)
		}
	}
	reminders, err := svc.db.LoadReminders()
	if err != nil {
		t.Fatalf("LoadReminders: %v", err)
	}
	for _, reminder := range reminders {
		if reminder.ID == legacyPast.ID && (reminder.NotificationAttempted == nil || !*reminder.NotificationAttempted) {
			t.Fatal("overdue legacy reminder should be marked handled")
		}
	}
}
