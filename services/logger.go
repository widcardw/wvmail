package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// InitLogger configures the standard library structured logger (slog) to write
// to a rolling file in the log directory. It also cleans up old log files
// according to the retention policy. If cfg is nil, defaults are used.
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
	retention := 30

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
		if cfg.RetentionDays > 0 {
			retention = cfg.RetentionDays
		}
	}

	logPath := filepath.Join(logDir, "app.log")
	w, err := newRotatingWriter(logPath, maxSize)
	if err != nil {
		return fmt.Errorf("log writer: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	// Clean old logs in background
	go cleanOldLogs(logDir, retention)

	slog.Info("logger initialized", "level", level, "max_size_mb", maxSize, "retention_days", retention)
	return nil
}

// rotatingWriter is an io.Writer that rotates the log file when it exceeds
// the configured size. Old files are renamed to app.log.1, app.log.2, etc.
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

func (w *rotatingWriter) rotate() {
	for i := 999; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", w.path, i)
		newPath := fmt.Sprintf("%s.%d", w.path, i+1)
		if _, err := os.Stat(oldPath); err == nil {
			os.Rename(oldPath, newPath)
		}
	}
	os.Rename(w.path, w.path+".1")
}

// Close implements io.Closer.
func (w *rotatingWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// cleanOldLogs removes log files older than the given number of days.
func cleanOldLogs(logDir string, retentionDays int) {
	// Run once on start, then once per day
	for {
		cleanOnce(logDir, retentionDays)
		time.Sleep(24 * time.Hour)
	}
}

func cleanOnce(logDir string, retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		slog.Warn("failed to read log dir for cleanup", "error", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "app.log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(logDir, e.Name())
			if err := os.Remove(path); err != nil {
				slog.Warn("failed to remove old log", "path", path, "error", err)
			}
		}
	}
}

// ListLogFiles returns the names of log files in the log directory, newest first.
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
		if !e.IsDir() && strings.HasPrefix(e.Name(), "app.log") {
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
