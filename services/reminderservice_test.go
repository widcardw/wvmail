package services

import "testing"

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
