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

func TestReminderNotificationsFireOnceWhenDue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := NewRemindersService()
	now := time.Now()
	due, err := svc.SaveReminder(Reminder{
		Title: "评审",
		Time:  now.Unix() + 1,
	})
	if err != nil {
		t.Fatalf("SaveReminder: %v", err)
	}
	future, err := svc.SaveReminder(Reminder{
		Title: "站会",
		Time:  now.Unix() + 3600,
	})
	if err != nil {
		t.Fatalf("SaveReminder(future): %v", err)
	}
	done, err := svc.SaveReminder(Reminder{
		Title: "已完成",
		Time:  now.Unix() - 1,
		Done:  true,
	})
	if err != nil {
		t.Fatalf("SaveReminder(done): %v", err)
	}

	var notified []string
	svc.SetNotifier(func(reminder Reminder) error {
		notified = append(notified, reminder.ID)
		return nil
	})

	svc.dispatchDue(now.Add(2 * time.Second))
	svc.dispatchDue(now.Add(3 * time.Second))
	if len(notified) != 1 || notified[0] != due.ID {
		t.Fatalf("notifications = %v, want only due reminder %q", notified, due.ID)
	}
	if future.NotificationAttempted || !done.NotificationAttempted {
		t.Fatalf("incorrect notification state: future=%+v done=%+v", future, done)
	}
}

func TestPastReminderDoesNotNotifyAndLegacyReminderIsNotReplayed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := NewRemindersService()
	past, err := svc.SaveReminder(Reminder{Title: "过去的提醒", Time: time.Now().Unix() - 1})
	if err != nil {
		t.Fatalf("SaveReminder(past): %v", err)
	}
	if !past.NotificationAttempted {
		t.Fatal("saving a reminder already in the past should suppress its notification")
	}

	legacyPast := CachedReminder{ID: "legacy-past", Title: "旧提醒", Time: time.Now().Unix() - 60}
	legacyFuture := CachedReminder{ID: "legacy-future", Title: "未来旧提醒", Time: time.Now().Unix() + 60}
	if err := svc.db.SaveReminder(&legacyPast); err != nil {
		t.Fatalf("SaveReminder(legacy past): %v", err)
	}
	if err := svc.db.SaveReminder(&legacyFuture); err != nil {
		t.Fatalf("SaveReminder(legacy future): %v", err)
	}

	var notified []string
	svc.SetNotifier(func(reminder Reminder) error {
		notified = append(notified, reminder.ID)
		return nil
	})
	svc.dispatchDue(time.Now())
	if len(notified) != 0 {
		t.Fatalf("notified %v reminders, want none yet", notified)
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
	svc.dispatchDue(time.Now().Add(61 * time.Second))
	if len(notified) != 1 || notified[0] != legacyFuture.ID {
		t.Fatalf("notifications = %v, want future legacy reminder %q", notified, legacyFuture.ID)
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
