package services

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Closing the main window can hide the app to the tray instead of quitting. The
// flag has to survive a config round-trip, and a config written before it existed
// must keep quitting (the zero value), which is what Main relies on when it reads
// the flag on every close.
func TestMinimizeWhenCloseRoundTrip(t *testing.T) {
	in := AppConfig{MinimizeWhenClose: true}

	data, err := yaml.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out AppConfig
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.MinimizeWhenClose {
		t.Error("MinimizeWhenClose was lost in the round-trip")
	}
}

func TestMinimizeWhenCloseDefaultsToQuitting(t *testing.T) {
	// "minimize_when_close" is absent from configs written by older versions.
	var cfg AppConfig
	if err := yaml.Unmarshal([]byte("language: zh-CN\ntheme: Dark\n"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.MinimizeWhenClose {
		t.Error("closing the window must quit unless the user opted into the tray")
	}
}

// The tray decision is read from disk on every window close, so the round-trip
// through SaveConfigField is the contract Main depends on.
func TestMinimizeWhenCloseFieldRoundTrip(t *testing.T) {
	// Keep the test away from the real ~/Library/Application Support/wvmail.
	t.Setenv("HOME", t.TempDir())

	svc := &ConfigService{}
	if svc.MinimizeWhenClose() {
		t.Fatal("a fresh config must quit on close")
	}
	if err := svc.SaveConfigField("minimize_when_close", "true"); err != nil {
		t.Fatalf("SaveConfigField: %v", err)
	}
	if !svc.MinimizeWhenClose() {
		t.Error("MinimizeWhenClose() = false after saving true")
	}
	if err := svc.SaveConfigField("minimize_when_close", "false"); err != nil {
		t.Fatalf("SaveConfigField: %v", err)
	}
	if svc.MinimizeWhenClose() {
		t.Error("MinimizeWhenClose() = true after saving false")
	}
}

func TestMinimizeWhenCloseFieldRejectsGarbage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := (&ConfigService{}).SaveConfigField("minimize_when_close", "yes please")
	if err == nil {
		t.Fatal("expected an error for a non-boolean value")
	}
	if !strings.Contains(err.Error(), "minimize_when_close") {
		t.Errorf("error should name the field, got %q", err.Error())
	}
}
