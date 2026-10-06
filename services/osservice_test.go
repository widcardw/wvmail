package services

import "testing"

// Settings → Quit is a one-way action, so the only thing worth pinning down is
// that the button cannot become a silent no-op: the service must call whatever
// main wired to it, and must say so instead of panicking when nothing was wired.
func TestOsServiceQuitApp(t *testing.T) {
	t.Run("calls the injected quit path once", func(t *testing.T) {
		calls := 0
		svc := NewOsService(func() { calls++ })

		if err := svc.QuitApp(); err != nil {
			t.Fatalf("QuitApp reported %v", err)
		}
		if calls != 1 {
			t.Errorf("expected the quit path to run exactly once, ran %d times", calls)
		}
	})

	t.Run("an unwired service reports instead of panicking", func(t *testing.T) {
		if err := (&OsService{}).QuitApp(); err == nil {
			t.Error("a service with no quit path must not report success")
		}
	})
}
