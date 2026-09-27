package services

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Config writes go through a temporary file plus a rename: the file must end up
// with the new content and the requested permissions, and no .tmp-* leftovers may
// remain next to it (the config also holds the encrypted account passwords).
func TestWriteFileAtomicReplacesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("old\n"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := writeFileAtomic(path, []byte("new\n"), 0600); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "new\n" {
		t.Errorf("content = %q, want %q", got, "new\n")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("perm = %o, want 600", perm)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("temporary file left behind: %s", entry.Name())
		}
	}
}

// Every mutating config method is a read-modify-write, so two settings saved at
// the same time must not clobber each other. Before configWriteMu this lost
// whichever update read first.
func TestConcurrentConfigFieldSavesKeepEveryUpdate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := &ConfigService{}
	if err := svc.SaveConfigField("minimize_when_close", "true"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = svc.SaveConfigField("language", "zh-CN")
		}()
		go func() {
			defer wg.Done()
			_ = svc.SaveConfigField("theme", "Dark")
		}()
	}
	wg.Wait()

	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg.Language != "zh-CN" {
		t.Errorf("Language = %q, want zh-CN", cfg.Language)
	}
	if cfg.Theme != "Dark" {
		t.Errorf("Theme = %q, want Dark", cfg.Theme)
	}
	// An untouched field written before the race has to survive it.
	if !cfg.MinimizeWhenClose {
		t.Error("MinimizeWhenClose was clobbered by a concurrent save")
	}
}
