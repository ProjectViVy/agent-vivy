package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubConsole swaps the package-local console seam for a capture buffer and
// a fake terminal flag, restoring the real stdout-backed seam afterwards.
func stubConsole(t *testing.T, tty bool) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := resolveConsole
	resolveConsole = func() (io.Writer, bool) { return buf, tty }
	t.Cleanup(func() { resolveConsole = prev })
	return buf
}

func dailyLogPath(dir string) string {
	return filepath.Join(dir, FilePrefix+"."+time.Now().Format("2006-01-02"))
}

func readDailyLog(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(dailyLogPath(dir))
	if err != nil {
		t.Fatalf("read daily file: %v", err)
	}
	return string(data)
}

func TestSetupSeparatesConsoleAndFileFormat(t *testing.T) {
	dir := t.TempDir()
	console := stubConsole(t, true) // fake terminal selects pretty
	logger, eff, closer, err := Setup(Options{Dir: dir, Stdout: true})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()

	if eff.Format != "json" || eff.Console != "pretty" {
		t.Errorf("effective = %+v; want file json / console pretty", eff)
	}
	logger.Info("hello vivy", "run", "r-1")

	line := readDailyLog(t, dir)
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &rec); err != nil {
		t.Fatalf("file sink is not valid JSON: %v\n%s", err, line)
	}
	if rec["msg"] != "hello vivy" || rec["run"] != "r-1" {
		t.Errorf("file record = %v", rec)
	}

	out := console.String()
	if strings.Contains(out, `"msg":`) {
		t.Errorf("console emitted JSON layout on a terminal: %q", out)
	}
	if !strings.Contains(out, "hello vivy") || !strings.Contains(out, "r-1") {
		t.Errorf("pretty console line missing message/attrs: %q", out)
	}
}

func TestSetupConsoleAutoRedirectedIsJSON(t *testing.T) {
	dir := t.TempDir()
	console := stubConsole(t, false) // redirected: not a terminal
	logger, eff, closer, err := Setup(Options{Dir: dir, Stdout: true})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()
	if eff.Console != "json" {
		t.Errorf("effective console = %q; want json when redirected", eff.Console)
	}
	logger.Info("piped", "k", "v")

	out := console.String()
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rec); err != nil {
		t.Fatalf("redirected console is not JSON: %v\n%s", err, out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("redirected console emitted ANSI codes: %q", out)
	}
}

func TestSetupConsoleExplicitFormats(t *testing.T) {
	for _, c := range []struct {
		format  string
		wantEff string
		pretty  bool
	}{
		{"pretty", "pretty", true},
		{"json", "json", false},
		{"text", "text", false},
	} {
		t.Run(c.format, func(t *testing.T) {
			dir := t.TempDir()
			console := stubConsole(t, false) // explicit overrides beat detection
			logger, eff, closer, err := Setup(Options{Dir: dir, Stdout: true, ConsoleFormat: c.format})
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}
			defer closer.Close()
			if eff.Console != c.wantEff {
				t.Errorf("effective console = %q; want %q", eff.Console, c.wantEff)
			}
			logger.Info("explicit", "a", 1)
			out := console.String()
			isJSON := strings.Contains(out, `"msg":`)
			if c.pretty && isJSON {
				t.Errorf("console_format=%s emitted JSON: %q", c.format, out)
			}
			if !c.pretty && c.format == "json" && !isJSON {
				t.Errorf("console_format=json did not emit JSON: %q", out)
			}
			if c.format == "text" && !strings.Contains(out, "msg=explicit") {
				t.Errorf("console_format=text did not emit logfmt: %q", out)
			}
		})
	}
}

func TestSetupConsoleRespectsNoColor(t *testing.T) {
	dir := t.TempDir()
	console := stubConsole(t, true)
	t.Setenv("NO_COLOR", "1")
	logger, _, closer, err := Setup(Options{Dir: dir, Stdout: true, ConsoleFormat: "pretty"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()
	logger.Info("plain please")
	if strings.Contains(console.String(), "\x1b[") {
		t.Errorf("NO_COLOR console emitted ANSI: %q", console.String())
	}
}

func TestSetupStdoutDisabledKeepsFileOnly(t *testing.T) {
	dir := t.TempDir()
	console := stubConsole(t, true)
	logger, eff, closer, err := Setup(Options{Dir: dir, Stdout: false})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()
	if eff.Console != "off" {
		t.Errorf("effective console = %q; want off when Stdout=false", eff.Console)
	}
	logger.Info("file only")
	if console.Len() != 0 {
		t.Errorf("console received output with Stdout=false: %q", console.String())
	}
	if !strings.Contains(readDailyLog(t, dir), `"msg":"file only"`) {
		t.Error("file sink lost the record")
	}
}

func TestSetupConsoleLevelFiltering(t *testing.T) {
	dir := t.TempDir()
	console := stubConsole(t, true)
	logger, _, closer, err := Setup(Options{Dir: dir, Stdout: true, Level: "warn"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()
	logger.Debug("dropped")
	logger.Info("also dropped")
	logger.Warn("kept")
	out := console.String()
	if strings.Contains(out, "dropped") {
		t.Errorf("console leaked sub-level records: %q", out)
	}
	if !strings.Contains(out, "kept") {
		t.Errorf("console lost warn record: %q", out)
	}
}

func TestSetupConsoleAttrsAndGroups(t *testing.T) {
	dir := t.TempDir()
	console := stubConsole(t, true)
	logger, _, closer, err := Setup(Options{Dir: dir, Stdout: true, ConsoleFormat: "json"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()
	logger.WithGroup("req").With("id", 7).Info("grouped", "user", "u-1")
	out := console.String()
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rec); err != nil {
		t.Fatalf("console JSON unparseable: %v\n%s", err, out)
	}
	req, ok := rec["req"].(map[string]any)
	if !ok || req["id"] != float64(7) || req["user"] != "u-1" {
		t.Errorf("grouped attrs not preserved on console: %v", rec)
	}
}

func TestSetupConsoleFormatValidation(t *testing.T) {
	dir := t.TempDir()
	if _, _, _, err := Setup(Options{Dir: dir, Stdout: true, ConsoleFormat: "fancy"}); err == nil {
		t.Error("Setup accepted invalid console_format")
	}
	if _, _, closer, err := Setup(Options{Dir: dir, Stdout: true, ConsoleFormat: " AUTO "}); err != nil {
		closer.Close()
		t.Errorf("Setup rejected padded uppercase auto: %v", err)
	}
	t.Setenv(EnvConsoleFormat, "bogus")
	if _, _, _, err := Setup(Options{Dir: dir, Stdout: true, ConsoleFormat: "pretty"}); err == nil {
		t.Error("Setup accepted invalid VIVY_LOG_CONSOLE_FORMAT")
	}
}

func TestSetupConsoleFormatEnvOverride(t *testing.T) {
	dir := t.TempDir()
	stubConsole(t, false)
	t.Setenv(EnvConsoleFormat, "pretty")
	_, eff, closer, err := Setup(Options{Dir: dir, Stdout: true, ConsoleFormat: "json"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()
	if eff.Console != "pretty" {
		t.Errorf("effective console = %q; want pretty from env override", eff.Console)
	}
}

type secretValuer struct{}

func (secretValuer) LogValue() slog.Value { return slog.StringValue("sk-abcdef1234567890") }

type secretError struct{}

func (secretError) Error() string { return "dial failed: Bearer abcdef1234567890xyz" }

type secretStringer struct{}

func (secretStringer) String() string { return "token=pk-abcdef1234567890" }

func TestRedactionResolvedValuesAcrossBothSinks(t *testing.T) {
	dir := t.TempDir()
	console := stubConsole(t, true)
	logger, _, closer, err := Setup(Options{Dir: dir, Stdout: true})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()

	logger.Error("outer Bearer abcdef1234567890xyz failed",
		slog.Any("nested", slog.GroupValue(
			slog.Any("valuer", secretValuer{}),
			slog.Any("err", secretError{}),
			slog.String("str", fmt.Sprint(secretStringer{})),
		)),
		slog.Any("plain", fmt.Errorf("wrap: %w", secretError{})),
	)

	for name, sink := range map[string]string{"console": console.String(), "file": readDailyLog(t, dir)} {
		for _, leak := range []string{"sk-abcdef1234567890", "abcdef1234567890xyz", "pk-abcdef1234567890"} {
			if strings.Contains(sink, leak) {
				t.Errorf("%s sink leaked secret %q: %s", name, leak, sink)
			}
		}
		if !strings.Contains(sink, "[REDACTED") {
			t.Errorf("%s sink shows no redaction marker: %s", name, sink)
		}
	}
	_ = errors.New // keep errors import if unused by future edits
}

func TestNewBootstrapConsoleSelection(t *testing.T) {
	// Bootstrap has no file: it mirrors the console decision on the given
	// descriptor. A real *os.File pipe is non-TTY, so output must be JSON
	// and ANSI-free; the redactor still applies.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	logger := NewBootstrap(w)
	logger.Error("boot failed", "err", "Bearer abcdef1234567890xyz")
	_ = w.Close()
	data, _ := io.ReadAll(r)
	line := string(data)
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &rec); err != nil {
		t.Fatalf("bootstrap output not JSON on a pipe: %v\n%s", err, line)
	}
	if strings.Contains(line, "abcdef1234567890xyz") || strings.Contains(line, "\x1b[") {
		t.Errorf("bootstrap leaked secret or ANSI: %q", line)
	}
}
