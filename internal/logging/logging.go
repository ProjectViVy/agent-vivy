// Package logging is the single init point for the Vivy kernel's slog
// output. It resolves level and format from config with environment
// overrides (VIVY_LOG_LEVEL, VIVY_LOG_FORMAT, VIVY_LOG_CONSOLE_FORMAT),
// mirrors the stream to the console and a daily-rotated file, and
// prunes rotated files past their retention window. Console and file
// formats are independent: the file keeps the configured json|text
// contract while the console resolves auto|pretty|json|text, with auto
// selecting pretty on a real terminal and JSON when redirected.
// Callers install the returned logger with slog.SetDefault and hold
// the closer for process lifetime.
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

	"github.com/mastwet/prettylog"
	"golang.org/x/term"
)

// Environment overrides, mirroring the reference harness convention of
// env above config: VIVY_LOG_LEVEL is the RUST_LOG analogue,
// VIVY_LOG_FORMAT the LOG_FORMAT analogue, VIVY_LOG_CONSOLE_FORMAT the
// console-sink analogue.
const (
	EnvLevel         = "VIVY_LOG_LEVEL"
	EnvFormat        = "VIVY_LOG_FORMAT"
	EnvConsoleFormat = "VIVY_LOG_CONSOLE_FORMAT"
)

// resolveConsole reports the console sink writer and whether it is a
// real terminal. Reassigned only by package tests; production code
// always sees os.Stdout and its descriptor.
var resolveConsole = func() (io.Writer, bool) {
	return os.Stdout, term.IsTerminal(int(os.Stdout.Fd()))
}

// FilePrefix names the rotating file family <prefix>.YYYY-MM-DD.
const FilePrefix = "vivy.log"

// Options describes one Setup call. Empty Level/Format mean the
// built-in defaults (info / json); Dir must resolve to the log sink
// directory (config default: <data_dir>/logs).
type Options struct {
	Level         string
	Format        string
	ConsoleFormat string
	Dir           string
	RetentionDays int
	Stdout        bool
}

// Effective reports the resolved settings so the caller can log the
// startup milestone with the real values instead of the raw config.
// Console is the resolved console format ("pretty", "json", or
// "text"), or "off" when the console sink is disabled.
type Effective struct {
	Level   string
	Format  string
	Console string
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

	consoleFormat, err := resolveConsoleFormat(opts.ConsoleFormat)
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

	hopts := &slog.HandlerOptions{Level: level, AddSource: true}
	handlers := []slog.Handler{newSinkHandler(format, f, hopts)}
	consoleEff := "off"
	if opts.Stdout {
		w, tty := resolveConsole()
		consoleEff = resolveAutoFormat(consoleFormat, tty)
		handlers = append(handlers, newSinkHandler(consoleEff, w, hopts))
	}
	// The redactor wraps the fan-out so every sink receives the same
	// sanitized record; the standard MultiHandler does not compete with
	// either sink's format.
	h := newRedactingHandler(slog.NewMultiHandler(handlers...))

	return slog.New(h),
		Effective{Level: strings.ToLower(level.String()), Format: format, Console: consoleEff},
		f,
		nil
}

// ResolveEffective applies the env overrides exactly as Setup does and
// returns the canonical lowercase level and format, without opening a sink.
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

// resolveConsoleFormat applies the VIVY_LOG_CONSOLE_FORMAT override
// above the configured value and parses the result strictly.
func resolveConsoleFormat(configured string) (string, error) {
	formatStr := configured
	if v := strings.TrimSpace(os.Getenv(EnvConsoleFormat)); v != "" {
		formatStr = v
	}
	return parseConsoleFormat(formatStr)
}

func parseConsoleFormat(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return "auto", nil
	case "pretty", "json", "text":
		f := strings.ToLower(strings.TrimSpace(s))
		return f, nil
	default:
		return "", fmt.Errorf("logging: invalid console_format %q (want auto|pretty|json|text)", s)
	}
}

// resolveAutoFormat turns the "auto" console policy into a concrete
// format: pretty on a real terminal, JSON when redirected. Explicit
// values pass through untouched.
func resolveAutoFormat(format string, tty bool) string {
	if format != "auto" {
		return format
	}
	if tty {
		return "pretty"
	}
	return "json"
}

// newSinkHandler builds one sink handler for the given format. The
// pretty sink is prettylog on the console writer; it detects ANSI
// support (including NO_COLOR) on that writer itself.
func newSinkHandler(format string, w io.Writer, hopts *slog.HandlerOptions) slog.Handler {
	switch format {
	case "pretty":
		opts := []prettylog.Option{prettylog.WithLevel(hopts.Level)}
		if hopts.AddSource {
			opts = append(opts, prettylog.WithSource(true))
		}
		return prettylog.NewHandler(w, opts...)
	case "text":
		return slog.NewTextHandler(w, hopts)
	default:
		return slog.NewJSONHandler(w, hopts)
	}
}

// NewBootstrap builds the pre-config logger used before config.yaml is
// parsed. It shares the console selection and redaction seam with
// Setup but has no file sink: pretty on a terminal stderr, JSON when
// redirected, so early startup failures stay readable without
// polluting a machine-consumed stdout.
func NewBootstrap(stderr *os.File) *slog.Logger {
	w := io.Writer(stderr)
	if w == nil {
		w = io.Discard
	}
	format := "json"
	if stderr != nil && term.IsTerminal(int(stderr.Fd())) {
		format = "pretty"
	}
	return slog.New(newRedactingHandler(newSinkHandler(format, w, &slog.HandlerOptions{})))
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
