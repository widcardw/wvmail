package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Date/time detection inside mail bodies is on by default. `readConfig` unmarshals
// on top of `defaultConfig`, so a config file written before the option existed
// (key absent) keeps the default instead of silently turning the feature off.
func TestDetectDatesInMailDefaultsToOn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, "Library", "Application Support", "wvmail")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := "language: zh-CN\ntheme: Dark\nminimize_when_close: false\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(legacy), 0600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cfg, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if !cfg.DetectDatesInMail {
		t.Error("a config without detect_dates_in_mail must still enable detection")
	}
}

// The mail view reads the flag from the preferences store, which is filled from
// GetConfig; the settings switch writes it through SaveConfigField.
func TestDetectDatesInMailFieldRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := &ConfigService{}
	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if !cfg.DetectDatesInMail {
		t.Fatal("a fresh config must detect dates in mail")
	}

	if err := svc.SaveConfigField("detect_dates_in_mail", "false"); err != nil {
		t.Fatalf("SaveConfigField(false): %v", err)
	}
	cfg, err = svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig after save: %v", err)
	}
	if cfg.DetectDatesInMail {
		t.Error("detection must stay off after saving false")
	}

	if err := svc.SaveConfigField("detect_dates_in_mail", "true"); err != nil {
		t.Fatalf("SaveConfigField(true): %v", err)
	}
	cfg, err = svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig after re-enable: %v", err)
	}
	if !cfg.DetectDatesInMail {
		t.Error("detection must come back after saving true")
	}
}

func TestDetectDatesInMailRejectsGarbage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := (&ConfigService{}).SaveConfigField("detect_dates_in_mail", "maybe")
	if err == nil {
		t.Fatal("expected an error for a non-boolean value")
	}
	if !strings.Contains(err.Error(), "detect_dates_in_mail") {
		t.Errorf("error should name the field, got %q", err.Error())
	}
}

// Turning a default-on flag off has to survive a reload: reading the config starts
// from the defaults, so a false value that `omitempty` dropped would come back as
// true. Same contract for the external-safety scan.
func TestDefaultOnFlagsPersistBeingTurnedOff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := &ConfigService{}
	if err := svc.SaveConfigField("scan_external_safety", "false"); err != nil {
		t.Fatalf("SaveConfigField(scan_external_safety): %v", err)
	}
	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg.ScanExternalSafety {
		t.Error("scan_external_safety = true after saving false")
	}
	if !cfg.DetectDatesInMail {
		t.Error("turning one flag off must not touch the other")
	}
}
