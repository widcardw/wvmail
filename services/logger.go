package services

// Logging conventions (keep every slog call in services/ aligned with these):
//
//   - Message: a short lowercase phrase describing the event, optionally
//     prefixed with the subsystem ("imap: login failed"). Prefixes in use:
//     imap:, sync:, tray:, cache:, body:, decode:, network:, logger:.
//   - Attributes: snake_case keys, always the same word for the same thing —
//     "account", "folder", "uid", "component" (UI area), and "error" for the
//     wrapped error (always last).
//   - "account" takes the EmailAccount itself, not its ID: EmailAccount.LogValue
//     renders it as account.email / account.name, so a line names the mailbox a
//     human recognises (and never its passwords).
//   - Levels: Debug for per-message internals, Info for state changes the
//     user can act on, Warn for recoverable failures, Error for lost work.
//   - Never log secrets (passwords, API keys) or full mail bodies in production.
//     Development builds additionally log complete LLM prompts/responses from
//     llm_debug_dev.go; the production build compiles those calls to no-ops.
//
// Line format is "[time][LEVEL] message key=value …" (see appHandler), not
// slog's default time=/level= key=value text — the prefix tags are noise for
// human eyes, so only the payload fields follow the message.
//
// The logger writes to the rolling file logs/app.log under the config dir and
// mirrors everything to stderr so `wails3 dev` shows live output. Rotation
// happens by size (LogConfig.MaxSizeMB); once RotateArchiveCount rotated files
// exist they are packed into a single gzip'd tar (app-logs-<timestamp>.tar.gz)
// and the originals are removed. Files older than LogConfig.RetentionDays are
// deleted; RetentionDays <= 0 disables expiry entirely.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RotateArchiveCount is how many rotated files (app.log.1 … app.log.N) are
// packed into one archive before rotation continues.
const RotateArchiveCount = 10

// allowedRetentionDays are the retention options offered by Settings → About.
// 0 means "never remove expired logs".
var allowedRetentionDays = []int{0, 7, 30, 60, 90, 180, 365}

// normalizeRetentionDays maps the configured value onto the Settings → About
// options. 0 is itself a valid option ("never remove expired logs") and the
// default for configs without an explicit value; any other off-list value
// snaps back to 30 days so hand-edited files cannot silently disable expiry.
func normalizeRetentionDays(days int) int {
	for _, d := range allowedRetentionDays {
		if d == days {
			return days
		}
	}
	return 30
}

// appHandler is a slog.Handler that writes lines as
//
//	[2026-01-02 15:04:05][INFO] message key=value key2=value
//
// instead of slog's default "time=… level=… msg=…" text format — the prefix
// tags carry no information a human can't read off the brackets, so the line
// leads with the message and appends payload fields after it. With debug level
// (AddSource) the caller's file:line is inserted as a third bracket tag.
type appHandler struct {
	opts  *slog.HandlerOptions
	attrs []slog.Attr
	group string
	out   io.Writer
	mu    *sync.Mutex
}

func newAppHandler(out io.Writer, opts *slog.HandlerOptions) *appHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &appHandler{opts: opts, out: out, mu: &sync.Mutex{}}
}

func (h *appHandler) Enabled(_ context.Context, l slog.Level) bool {
	min := slog.LevelInfo
	if h.opts.Level != nil {
		min = h.opts.Level.Level()
	}
	return l >= min
}

func (h *appHandler) Handle(_ context.Context, r slog.Record) error {
	buf := &bytes.Buffer{}
	buf.WriteByte('[')
	buf.WriteString(r.Time.Format("2006-01-02 15:04:05"))
	buf.WriteString("][")
	buf.WriteString(r.Level.String())
	buf.WriteByte(']')
	if src := sourceTag(h.opts, r); src != "" {
		buf.WriteByte('[')
		buf.WriteString(src)
		buf.WriteByte(']')
	}
	buf.WriteByte(' ')
	buf.WriteString(r.Message)
	for _, a := range h.attrs {
		appendAttr(buf, a, h.group)
	}
	r.Attrs(func(a slog.Attr) bool {
		appendAttr(buf, a, h.group)
		return true
	})
	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf.Bytes())
	return err
}

// forceLog bypasses the configured minimum level. It exists only for
// development-build diagnostics that must be visible even when the user chose
// warn/error; production builds compile those callers to no-ops.
func forceLog(message string, attrs ...slog.Attr) {
	record := slog.NewRecord(time.Now(), slog.LevelDebug, message, 0)
	record.AddAttrs(attrs...)
	_ = slog.Default().Handler().Handle(context.Background(), record)
}

// sourceTag renders the caller's file:line when AddSource is on.
func sourceTag(opts *slog.HandlerOptions, r slog.Record) string {
	if !opts.AddSource || r.PC == 0 {
		return ""
	}
	frames := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := frames.Next()
	if f.File == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
}

// appendAttr writes " key=value", flattening group attrs into dotted keys.
func appendAttr(buf *bytes.Buffer, a slog.Attr, group string) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	key := a.Key
	if group != "" {
		key = group + "." + key
	}
	if a.Value.Kind() == slog.KindGroup {
		sub := key
		for _, ga := range a.Value.Group() {
			appendAttr(buf, ga, sub)
		}
		return
	}
	buf.WriteByte(' ')
	buf.WriteString(key)
	buf.WriteByte('=')
	appendValue(buf, a.Value)
}

func appendValue(buf *bytes.Buffer, v slog.Value) {
	var s string
	if err, ok := v.Any().(error); ok {
		s = err.Error()
	} else {
		s = v.String()
	}
	if s == "" {
		buf.WriteString(`""`)
		return
	}
	if strings.ContainsAny(s, " \t\n\"=") {
		buf.WriteString(strconv.Quote(s))
		return
	}
	buf.WriteString(s)
}

func (h *appHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *appHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := *h
	if h.group != "" {
		next.group = h.group + "." + name
	} else {
		next.group = name
	}
	return &next
}

// InitLogger configures the standard library structured logger (slog) to write
// to a rolling file in the log directory (mirrored to stderr). It also cleans
// up old log files according to the retention policy. If cfg is nil, defaults
// are used.
func InitLogger(cfg *LogConfig) error {
	dir, err := configDir()
	if err != nil {
		return fmt.Errorf("log dir: %w", err)
	}
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("mkdir logs: %w", err)
	}

	level := slog.LevelInfo
	maxSize := 10 // MB
	// Default retention: 0 = never remove expired logs. A config always
	// carries an explicit value; this only covers a nil cfg.
	retention := 0

	if cfg != nil {
		switch strings.ToLower(cfg.Level) {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
		if cfg.MaxSizeMB > 0 {
			maxSize = cfg.MaxSizeMB
		}
		retention = normalizeRetentionDays(cfg.RetentionDays)
	}

	logPath := filepath.Join(logDir, "app.log")
	w, err := newRotatingWriter(logPath, maxSize)
	if err != nil {
		return fmt.Errorf("log writer: %w", err)
	}

	logger := slog.New(newAppHandler(io.MultiWriter(w, os.Stderr), &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// Clean old logs in background
	go cleanOldLogs(logDir, retention)

	slog.Info("logger initialized", "level", level, "max_size_mb", maxSize,
		"retention_days", retention, "archive_after_files", RotateArchiveCount)
	return nil
}

// rotatingWriter is an io.Writer that rotates the log file when it exceeds
// the configured size. Old files are renamed to app.log.1, app.log.2, etc.
// Once RotateArchiveCount rotated files exist they are packed into
// app-logs-<timestamp>.tar.gz and the originals are deleted.
type rotatingWriter struct {
	path     string
	maxBytes int64
	file     *os.File
	size     int64
}

func newRotatingWriter(path string, maxSizeMB int) (*rotatingWriter, error) {
	maxBytes := int64(maxSizeMB) * 1024 * 1024
	w := &rotatingWriter{path: path, maxBytes: maxBytes}
	fi, err := os.Stat(path)
	if err == nil {
		w.size = fi.Size()
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	w.file = f
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	if w.size+int64(len(p)) > w.maxBytes && w.size > 0 {
		w.file.Close()
		w.rotate()
		w.size = 0
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// rotate renames app.log to app.log.1, shifting older rotations up. When
// RotateArchiveCount rotated files already exist, the newest batch is packed
// into an archive first so only RotateArchiveCount rotated files stay on disk.
func (w *rotatingWriter) rotate() {
	oldest := fmt.Sprintf("%s.%d", w.path, RotateArchiveCount)
	if _, err := os.Stat(oldest); err == nil {
		archiveRotatedLogs(w.path, RotateArchiveCount)
	}
	// Shift app.log.(N-1) → app.log.N, app.log.(N-2) → app.log.(N-1), …
	for i := RotateArchiveCount - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", w.path, i)
		newPath := fmt.Sprintf("%s.%d", w.path, i+1)
		if _, err := os.Stat(oldPath); err == nil {
			os.Rename(oldPath, newPath)
		}
	}
	os.Rename(w.path, w.path+".1")
}

// archiveRotatedLogs packs app.log.1 … app.log.count into
// app-logs-<timestamp>.tar.gz and deletes the originals. On failure the oldest
// rotated file is removed anyway so the set stays bounded; a warning is
// written to whatever logger is active.
func archiveRotatedLogs(base string, count int) {
	dir := filepath.Dir(base)
	stamp := time.Now().Format("20060102-150405")
	archivePath := filepath.Join(dir, fmt.Sprintf("app-logs-%s.tar.gz", stamp))

	err := writeLogArchive(archivePath, base, count)
	if err != nil {
		slog.Error("logger: packing rotated logs failed", "archive", archivePath, "error", err)
		// Keep the set bounded: drop only the oldest file, keep the rest.
		os.Remove(fmt.Sprintf("%s.%d", base, count))
		return
	}
	for i := 1; i <= count; i++ {
		os.Remove(fmt.Sprintf("%s.%d", base, i))
	}
	slog.Info("logger: packed rotated logs", "archive", filepath.Base(archivePath), "files", count)
}

// writeLogArchive writes app.log.1 … app.log.count into a gzip'd tar.
func writeLogArchive(archivePath, base string, count int) error {
	out, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s.%d", filepath.Base(base), i)
		data, err := os.ReadFile(filepath.Join(filepath.Dir(base), name))
		if err != nil {
			if os.IsNotExist(err) {
				continue // gaps are fine after a failed earlier archive
			}
			return fmt.Errorf("read %s: %w", name, err)
		}
		hdr := &tar.Header{
			Name:    name,
			Mode:    0644,
			Size:    int64(len(data)),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("tar header %s: %w", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			return fmt.Errorf("tar write %s: %w", name, err)
		}
	}
	return nil
}

// Close implements io.Closer.
func (w *rotatingWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// cleanOldLogs removes log files (including packed archives) older than the
// given number of days. retentionDays <= 0 disables expiry.
func cleanOldLogs(logDir string, retentionDays int) {
	if retentionDays <= 0 {
		slog.Info("logger: log expiry disabled, keeping all old logs")
		return
	}
	// Run once on start, then once per day
	for {
		cleanOnce(logDir, retentionDays)
		time.Sleep(24 * time.Hour)
	}
}

func cleanOnce(logDir string, retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		slog.Warn("logger: failed to read log dir for cleanup", "error", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "app") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(logDir, e.Name())
			if err := os.Remove(path); err != nil {
				slog.Warn("logger: failed to remove old log", "path", path, "error", err)
			}
		}
	}
}

// ListLogFiles returns the names of log files (including packed archives) in
// the log directory, newest first.
func ListLogFiles() []string {
	dir, err := configDir()
	if err != nil {
		return nil
	}
	logDir := filepath.Join(dir, "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "app") {
			files = append(files, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files
}

// noopLogger is a slog handler that discards all output. Used before InitLogger
// is called to avoid panics from nil logger.
type noopHandler struct{}

func (noopHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (noopHandler) Handle(context.Context, slog.Record) error { return nil }
func (noopHandler) WithAttrs(attrs []slog.Attr) slog.Handler  { return noopHandler{} }
func (noopHandler) WithGroup(name string) slog.Handler        { return noopHandler{} }

func init() {
	// Set a no-op logger so early init code can call slog without crashing.
	slog.SetDefault(slog.New(noopHandler{}))
}
