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
// VIVY_LOG_FORMAT the LOG_FORMAT analogue. The VIVY_WORKER_LOG_* family
// is set by the supervisor when spawning `vivy worker` children and
// carries the parent's validated log settings (LOGGING.md §3).
const (
	EnvLevel  = "VIVY_LOG_LEVEL"
	EnvFormat = "VIVY_LOG_FORMAT"

	EnvWorkerLogDir    = "VIVY_WORKER_LOG_DIR"
	EnvWorkerLogLevel  = "VIVY_WORKER_LOG_LEVEL"
	EnvWorkerLogFormat = "VIVY_WORKER_LOG_FORMAT"
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

	level, err := resolveLevel(opts.Level)
	if err != nil {
		return nil, Effective{}, nil, err
	}

	format, err := resolveFormat(opts.Format)
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

// SetupWorker builds the file sink for one `vivy worker` child process.
// The supervisor exports the parent's validated log settings through the
// VIVY_WORKER_LOG_* environment; an unset dir disables file logging and
// returns a nil logger (the worker protocol then runs as before, with
// diagnostics invisible). The child never writes stdout (the protocol
// owns it) and never rotates or sweeps files: each process appends to
// its own <dir>/vivy.log.worker-<pid>, and the parent's startup
// retention sweep — matching the vivy.log prefix — prunes the file once
// the worker is gone and its mtime ages out.
func SetupWorker() (*slog.Logger, io.Closer, string, error) {
	dir := strings.TrimSpace(os.Getenv(EnvWorkerLogDir))
	if dir == "" {
		return nil, nil, "", nil
	}

	levelStr := strings.TrimSpace(os.Getenv(EnvWorkerLogLevel))
	if levelStr == "" {
		levelStr = strings.TrimSpace(os.Getenv(EnvLevel))
	}
	level, err := parseLevel(levelStr)
	if err != nil {
		return nil, nil, "", err
	}

	formatStr := strings.TrimSpace(os.Getenv(EnvWorkerLogFormat))
	if formatStr == "" {
		formatStr = strings.TrimSpace(os.Getenv(EnvFormat))
	}
	format, err := parseFormat(formatStr)
	if err != nil {
		return nil, nil, "", err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, "", fmt.Errorf("logging: create %s: %w", dir, err)
	}
	name := filepath.Join(dir, fmt.Sprintf("%s.worker-%d", FilePrefix, os.Getpid()))
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, "", fmt.Errorf("logging: open %s: %w", name, err)
	}

	hopts := &slog.HandlerOptions{Level: level, AddSource: true}
	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(f, hopts)
	} else {
		h = slog.NewJSONHandler(f, hopts)
	}
	return slog.New(h), f, name, nil
}

// ResolveEffective applies the env overrides exactly as Setup does and
// returns the canonical lowercase level and format, without opening a
// sink. The supervisor uses it to hand the parent's effective log
// settings to worker children so their per-worker file sink matches the
// parent process (LOGGING.md §3).
func ResolveEffective(level, format string) (Effective, error) {
	l, err := resolveLevel(level)
	if err != nil {
		return Effective{}, err
	}
	f, err := resolveFormat(format)
	if err != nil {
		return Effective{}, err
	}
	return Effective{Level: strings.ToLower(l.String()), Format: f}, nil
}

// resolveLevel applies the VIVY_LOG_LEVEL override above the configured
// value and parses the result strictly.
func resolveLevel(configured string) (slog.Level, error) {
	levelStr := configured
	if v := strings.TrimSpace(os.Getenv(EnvLevel)); v != "" {
		levelStr = v
	}
	return parseLevel(levelStr)
}

// resolveFormat applies the VIVY_LOG_FORMAT override above the configured
// value and parses the result strictly.
func resolveFormat(configured string) (string, error) {
	formatStr := configured
	if v := strings.TrimSpace(os.Getenv(EnvFormat)); v != "" {
		formatStr = v
	}
	return parseFormat(formatStr)
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
