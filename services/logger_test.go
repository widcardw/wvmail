package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// forceRotation writes a chunk larger than the rotation threshold so the next
// Write call rolls the file over.
func forceRotation(t *testing.T, w *rotatingWriter) {
	t.Helper()
	chunk := bytes.Repeat([]byte("x\n"), 600*1024) // ~1.2 MB > 1 MB threshold
	if _, err := w.Write(chunk); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestRotatingWriterArchivesAfterTenRotations(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "app.log")
	w, err := newRotatingWriter(base, 1) // 1 MB per file
	if err != nil {
		t.Fatalf("newRotatingWriter: %v", err)
	}
	defer w.Close()

	// 12 writes → 11 rotations. The 11th sees app.log.10 and archives
	// app.log.1 … app.log.10 before shifting.
	for i := 0; i < 12; i++ {
		forceRotation(t, w)
	}

	archives, err := filepath.Glob(filepath.Join(dir, "app-logs-*.tar.gz"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("expected exactly 1 archive, got %d: %v", len(archives), archives)
	}

	// The archived originals must be gone; the freshly rotated app.log.1 stays.
	for i := 2; i <= 10; i++ {
		p := fmt.Sprintf("%s.%d", base, i)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("archived file still on disk: %s", p)
		}
	}
	if _, err := os.Stat(base + ".1"); err != nil {
		t.Errorf("expected fresh app.log.1: %v", err)
	}

	// The archive must contain app.log.1 … app.log.10.
	names := readArchiveNames(t, archives[0])
	if len(names) != 10 {
		t.Fatalf("expected 10 entries in archive, got %d: %v", len(names), names)
	}
	for i := 1; i <= 10; i++ {
		want := fmt.Sprintf("app.log.%d", i)
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("archive missing %s", want)
		}
	}
}

func readArchiveNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		names = append(names, hdr.Name)
	}
	return names
}

func TestNormalizeRetentionDays(t *testing.T) {
	cases := map[int]int{
		0:   0,   // "never remove" is a first-class option
		7:   7,   // offered option
		30:  30,  // offered option
		365: 365, // offered option
		14:  30,  // off-list snaps back to 30
		-5:  30,
	}
	for in, want := range cases {
		if got := normalizeRetentionDays(in); got != want {
			t.Errorf("normalizeRetentionDays(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestAppHandlerFormat(t *testing.T) {
	var buf bytes.Buffer
	h := newAppHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(h)

	logger.Info("imap: login failed", "account", "a1", "error", io.EOF)
	logger.Warn("cache: slow folder", "folder", "INBOX/Sent Messages")

	line := buf.String()
	// Leading tags, no slog key=value noise.
	if !strings.HasPrefix(line, "[2") || !strings.Contains(line, "][INFO] imap: login failed") {
		t.Errorf("unexpected format: %q", line)
	}
	if strings.Contains(line, "time=") || strings.Contains(line, "level=") || strings.Contains(line, "msg=") {
		t.Errorf("default slog tags leaked into line: %q", line)
	}
	if !strings.Contains(line, " account=a1") || !strings.Contains(line, " error=EOF") {
		t.Errorf("payload fields missing: %q", line)
	}
	// Values with spaces are quoted so fields stay parseable.
	if !strings.Contains(buf.String(), ` folder="INBOX/Sent Messages"`) {
		t.Errorf("spaced value not quoted: %q", buf.String())
	}
}

// An account logged by value must read as the mailbox, not as the UUID that keys
// config.yaml — and it must never print the credentials it carries.
func TestAppHandlerAccountValue(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newAppHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	account := EmailAccount{
		ID:       "0f3a1c2e-uuid",
		Name:     "SZU",
		Email:    "2300271084@email.szu.edu.cn",
		IMAPPass: "super-secret",
		SMTPPass: "also-secret",
	}
	logger.Info("idle: watching inbox", "account", account, "folder", "INBOX")

	line := buf.String()
	if !strings.Contains(line, "account.email=2300271084@email.szu.edu.cn") {
		t.Errorf("email address missing from line: %q", line)
	}
	if !strings.Contains(line, "account.name=SZU") {
		t.Errorf("display name missing from line: %q", line)
	}
	if strings.Contains(line, account.ID) {
		t.Errorf("account ID leaked into line: %q", line)
	}
	for _, secret := range []string{account.IMAPPass, account.SMTPPass} {
		if strings.Contains(line, secret) {
			t.Errorf("secret %q leaked into line: %q", secret, line)
		}
	}
}

func TestForceLogBypassesConfiguredLevel(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)

	var buf bytes.Buffer
	slog.SetDefault(slog.New(newAppHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelError,
	})))
	forceLog("llm: full response", slog.String("response", "complete payload"))

	if !strings.Contains(buf.String(), "complete payload") {
		t.Fatalf("forced diagnostic was filtered: %q", buf.String())
	}
}

func TestCleanOnceRespectsNeverRemove(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().AddDate(0, 0, -60)
	for _, name := range []string{"app.log", "app.log.3", "app-logs-20250101-000000.tar.gz", "other.log"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("log"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}

	// Retention 0 = never: nothing is removed.
	cleanOnce(dir, 0)
	for _, name := range []string{"app.log", "app.log.3", "app-logs-20250101-000000.tar.gz"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("retention 0 removed %s: %v", name, err)
		}
	}

	// Retention 7 removes expired app* files (including archives), nothing else.
	cleanOnce(dir, 7)
	for _, name := range []string{"app.log", "app.log.3", "app-logs-20250101-000000.tar.gz"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("retention 7 kept expired %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "other.log")); err != nil {
		t.Errorf("retention 7 removed non-log file other.log: %v", err)
	}
}

func TestWriteLogArchiveSkipsMissingSegments(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "app.log")
	// Only segments 2 and 4 exist (gaps from an earlier failed archive).
	for _, i := range []int{2, 4} {
		if err := os.WriteFile(fmt.Sprintf("%s.%d", base, i), []byte(strings.Repeat("log ", i)), 0644); err != nil {
			t.Fatalf("write segment: %v", err)
		}
	}
	archivePath := filepath.Join(dir, "app-logs-test.tar.gz")
	if err := writeLogArchive(archivePath, base, 5); err != nil {
		t.Fatalf("writeLogArchive: %v", err)
	}
	names := readArchiveNames(t, archivePath)
	if len(names) != 2 {
		t.Fatalf("expected 2 entries (segments 2 and 4), got %v", names)
	}
}
