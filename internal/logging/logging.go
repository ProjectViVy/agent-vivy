// Package logging is the single init point for the Vivy kernel's slog
// output. It resolves level and format from config with environment
// overrides (VIVY_LOG_LEVEL, VIVY_LOG_FORMAT), mirrors the stream to
// stdout and a daily-rotated file, and prunes rotated files past their
// retention window. Callers install the returned logger with
// slog.SetDefault and hold the closer for process lifetime.
package logging

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Environment overrides, mirroring the reference harness convention of
// env above config: VIVY_LOG_LEVEL is the RUST_LOG analogue,
// VIVY_LOG_FORMAT the LOG_FORMAT analogue.
const (
	EnvLevel  = "VIVY_LOG_LEVEL"
	EnvFormat = "VIVY_LOG_FORMAT"
)

// FilePrefix names the rotating file family <prefix>.YYYY-MM-DD.
const FilePrefix = "vivy.log"

// Options describes one Setup call. Empty Level/Format mean the
// built-in defaults (info / json); Dir must resolve to the log sink
// directory (config default: <data_dir>/logs).
type Options struct {
	Level         string
	Format        string
	Dir           string
	RetentionDays int
	Stdout        bool
}

// Effective reports the resolved settings so the caller can log the
// startup milestone with the real values instead of the raw config.
type Effective struct {
	Level  string
	Format string
}

// Setup builds the process logger from opts. Level and format accept
// the documented enums; the env overrides win when set and are parsed
// strictly, so an ops typo aborts startup with a clear message instead
// of silently keeping the configured value. The file layer is written
// synchronously (no buffering), so the closer is an orderly-shutdown
// formality rather than a flush guarantee.
func Setup(opts Options) (*slog.Logger, Effective, io.Closer, error) {
	if strings.TrimSpace(opts.Dir) == "" {
		return nil, Effective{}, nil, errors.New("logging: dir is required")
	}

	levelStr := opts.Level
	if v := strings.TrimSpace(os.Getenv(EnvLevel)); v != "" {
		levelStr = v
	}
	level, err := parseLevel(levelStr)
	if err != nil {
		return nil, Effective{}, nil, err
	}

	formatStr := opts.Format
	if v := strings.TrimSpace(os.Getenv(EnvFormat)); v != "" {
		formatStr = v
	}
	format, err := parseFormat(formatStr)
	if err != nil {
		return nil, Effective{}, nil, err
	}

	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, Effective{}, nil, fmt.Errorf("logging: create %s: %w", opts.Dir, err)
	}
	f := newDailyFile(opts.Dir, FilePrefix, time.Now)
	if err := cleanOldLogs(opts.Dir, FilePrefix, opts.RetentionDays, time.Now); err != nil {
		_ = f.Close()
		return nil, Effective{}, nil, err
	}

	var w io.Writer = f
	if opts.Stdout {
		w = io.MultiWriter(os.Stdout, f)
	}
	hopts := &slog.HandlerOptions{Level: level, AddSource: true}
	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(w, hopts)
	} else {
		h = slog.NewJSONHandler(w, hopts)
	}

	return slog.New(h),
		Effective{Level: strings.ToLower(level.String()), Format: format},
		f,
		nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logging: invalid level %q (want debug|info|warn|error)", s)
	}
}

func parseFormat(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "json":
		return "json", nil
	case "text":
		return "text", nil
	default:
		return "", fmt.Errorf("logging: invalid format %q (want json|text)", s)
	}
}

// dailyFile appends to <dir>/<prefix>.YYYY-MM-DD and switches files when
// the local date rolls over. Safe for concurrent use.
type dailyFile struct {
	mu     sync.Mutex
	dir    string
	prefix string
	now    func() time.Time

	f   *os.File
	day string
}

func newDailyFile(dir, prefix string, now func() time.Time) *dailyFile {
	return &dailyFile{dir: dir, prefix: prefix, now: now}
}

func (d *dailyFile) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	day := d.now().Format("2006-01-02")
	if d.f == nil || day != d.day {
		if err := d.open(day); err != nil {
			return 0, err
		}
	}
	return d.f.Write(p)
}

func (d *dailyFile) open(day string) error {
	if d.f != nil {
		_ = d.f.Close()
		d.f = nil
	}
	name := filepath.Join(d.dir, fmt.Sprintf("%s.%s", d.prefix, day))
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("logging: open %s: %w", name, err)
	}
	d.f, d.day = f, day
	return nil
}

func (d *dailyFile) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.f == nil {
		return nil
	}
	err := d.f.Close()
	d.f = nil
	return err
}

// cleanOldLogs deletes rotated files older than keepDays by mtime.
// keepDays <= 0 keeps everything. Individual remove failures are
// skipped (a file locked by another process must not abort startup);
// the returned error only reports an unreadable directory so tests can
// assert the sweep ran.
func cleanOldLogs(dir, prefix string, keepDays int, now func() time.Time) error {
	if keepDays <= 0 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("logging: read %s: %w", dir, err)
	}
	cutoff := now().AddDate(0, 0, -keepDays)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, prefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
	return nil
}
