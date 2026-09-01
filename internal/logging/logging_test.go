package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
		ok   bool
	}{
		{"", slog.LevelInfo, true},
		{"info", slog.LevelInfo, true},
		{"debug", slog.LevelDebug, true},
		{"warn", slog.LevelWarn, true},
		{"warning", slog.LevelWarn, true},
		{"error", slog.LevelError, true},
		{"ERROR", slog.LevelError, true},
		{" loud ", slog.LevelInfo, false},
		{"trace", slog.LevelInfo, false},
	}
	for _, c := range cases {
		got, err := parseLevel(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("parseLevel(%q) = %v, %v; want %v, nil", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("parseLevel(%q) accepted; want error", c.in)
		}
	}
}

func TestParseFormat(t *testing.T) {
	for _, ok := range []string{"", " ", "json", "text", "JSON"} {
		if _, err := parseFormat(ok); err != nil {
			t.Errorf("parseFormat(%q) = %v; want nil", ok, err)
		}
	}
	for _, bad := range []string{"logfmt", "xml", "jsonl"} {
		if _, err := parseFormat(bad); err == nil {
			t.Errorf("parseFormat(%q) accepted; want error", bad)
		}
	}
}

func TestSetupDefaultsAndFileSink(t *testing.T) {
	dir := t.TempDir()
	logger, eff, closer, err := Setup(Options{Dir: dir})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()

	if eff.Level != "info" || eff.Format != "json" {
		t.Errorf("effective = %+v; want info/json", eff)
	}
	logger.Info("hello vivy", "run", "r-1")

	data, err := os.ReadFile(filepath.Join(dir, FilePrefix+"."+time.Now().Format("2006-01-02")))
	if err != nil {
		t.Fatalf("read daily file: %v", err)
	}
	line := string(data)
	if !strings.Contains(line, `"msg":"hello vivy"`) ||
		!strings.Contains(line, `"run":"r-1"`) ||
		!strings.Contains(line, `"level":"INFO"`) ||
		!strings.Contains(line, `"source"`) {
		t.Errorf("json line missing msg/run/level/source: %s", line)
	}
}

func TestSetupTextFormat(t *testing.T) {
	dir := t.TempDir()
	logger, eff, closer, err := Setup(Options{Dir: dir, Format: "text"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()
	if eff.Format != "text" {
		t.Errorf("format = %q; want text", eff.Format)
	}
	logger.Warn("text line")
	data, err := os.ReadFile(filepath.Join(dir, FilePrefix+"."+time.Now().Format("2006-01-02")))
	if err != nil {
		t.Fatalf("read daily file: %v", err)
	}
	if !strings.Contains(string(data), "level=WARN") || !strings.Contains(string(data), `msg="text line"`) {
		t.Errorf("text line not as expected: %s", data)
	}
}

func TestSetupEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvLevel, "debug")
	t.Setenv(EnvFormat, "text")
	_, eff, closer, err := Setup(Options{Dir: dir, Level: "info", Format: "json"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closer.Close()
	if eff.Level != "debug" || eff.Format != "text" {
		t.Errorf("effective = %+v; want debug/text from env", eff)
	}
}

func TestSetupInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	if _, _, _, err := Setup(Options{Dir: dir, Level: "verbose"}); err == nil {
		t.Error("Setup accepted invalid level")
	}
	if _, _, _, err := Setup(Options{Dir: dir, Format: "xml"}); err == nil {
		t.Error("Setup accepted invalid format")
	}
	if _, _, _, err := Setup(Options{Dir: ""}); err == nil {
		t.Error("Setup accepted empty dir")
	}
	t.Setenv(EnvLevel, "loud")
	if _, _, _, err := Setup(Options{Dir: dir}); err == nil {
		t.Error("Setup accepted invalid VIVY_LOG_LEVEL")
	}
}

func TestDailyFileRollover(t *testing.T) {
	dir := t.TempDir()
	day1 := time.Date(2026, 8, 29, 23, 59, 0, 0, time.Local)
	clock := day1
	f := newDailyFile(dir, FilePrefix, func() time.Time { return clock })
	if _, err := f.Write([]byte("day one\n")); err != nil {
		t.Fatalf("write day one: %v", err)
	}
	clock = day1.Add(2 * time.Hour) // next local day
	if _, err := f.Write([]byte("day two\n")); err != nil {
		t.Fatalf("write day two: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	d1, err := os.ReadFile(filepath.Join(dir, FilePrefix+".2026-08-29"))
	if err != nil || string(d1) != "day one\n" {
		t.Errorf("day one file = %q, %v", d1, err)
	}
	d2, err := os.ReadFile(filepath.Join(dir, FilePrefix+".2026-08-30"))
	if err != nil || string(d2) != "day two\n" {
		t.Errorf("day two file = %q, %v", d2, err)
	}
}

func TestCleanOldLogs(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, FilePrefix+".2026-01-01")
	fresh := filepath.Join(dir, FilePrefix+"."+time.Now().Format("2006-01-02"))
	unrelated := filepath.Join(dir, "other.txt")
	for _, p := range []string{old, fresh, unrelated} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
	}
	past := time.Now().AddDate(0, 0, -40)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := cleanOldLogs(dir, FilePrefix, 30, time.Now); err != nil {
		t.Fatalf("cleanOldLogs: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old file survived: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh file removed: %v", err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("unrelated file removed: %v", err)
	}

	// keepDays <= 0 keeps everything (re-seed: the sweep above removed it).
	if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
		t.Fatalf("re-seed %s: %v", old, err)
	}
	if err := cleanOldLogs(dir, FilePrefix, 0, time.Now); err != nil {
		t.Fatalf("cleanOldLogs keep-all: %v", err)
	}
	if _, err := os.Stat(old); os.IsNotExist(err) {
		t.Error("keep-all removed the old file")
	}
}

func TestResolveEffective(t *testing.T) {
	eff, err := ResolveEffective("warning", "text")
	if err != nil || eff.Level != "warn" || eff.Format != "text" {
		t.Fatalf("ResolveEffective(warning, text) = %+v, %v; want warn/text", eff, err)
	}
	// Negative cases before the env overrides: env wins over config, so
	// setting VIVY_LOG_* first would mask the invalid configured values.
	if _, err := ResolveEffective("loud", "json"); err == nil {
		t.Error("ResolveEffective accepted an invalid level")
	}
	if _, err := ResolveEffective("info", "xml"); err == nil {
		t.Error("ResolveEffective accepted an invalid format")
	}
	t.Setenv(EnvLevel, "debug")
	t.Setenv(EnvFormat, "json")
	eff, err = ResolveEffective("warn", "text")
	if err != nil || eff.Level != "debug" || eff.Format != "json" {
		t.Fatalf("ResolveEffective with env override = %+v, %v; want debug/json", eff, err)
	}
}

func TestSetupWorkerDisabledWithoutDir(t *testing.T) {
	t.Setenv(EnvWorkerLogDir, "")
	logger, closer, path, err := SetupWorker()
	if err != nil || logger != nil || closer != nil || path != "" {
		t.Fatalf("SetupWorker without dir = %v, %v, %q, %v; want a disabled sink", logger, closer, path, err)
	}
}

func TestSetupWorkerFileSink(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvWorkerLogDir, dir)
	t.Setenv(EnvWorkerLogLevel, "debug")
	t.Setenv(EnvWorkerLogFormat, "text")
	logger, closer, path, err := SetupWorker()
	if err != nil {
		t.Fatalf("SetupWorker: %v", err)
	}
	if logger == nil || closer == nil {
		t.Fatal("SetupWorker returned a nil sink although the dir env is set")
	}
	defer closer.Close()
	want := filepath.Join(dir, fmt.Sprintf("%s.worker-%d", FilePrefix, os.Getpid()))
	if path != want {
		t.Fatalf("path = %q; want %q", path, want)
	}
	logger.Info("worker hello", "run", "r-1")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read worker file: %v", err)
	}
	line := string(data)
	if !strings.Contains(line, `msg="worker hello"`) ||
		!strings.Contains(line, "level=INFO") ||
		!strings.Contains(line, "run=r-1") ||
		!strings.Contains(line, "source") {
		t.Errorf("text line missing msg/level/run/source: %s", line)
	}
}

func TestSetupWorkerStrictAndFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvWorkerLogDir, dir)
	t.Setenv(EnvWorkerLogLevel, "loud")
	if _, _, _, err := SetupWorker(); err == nil {
		t.Fatal("SetupWorker accepted an invalid VIVY_WORKER_LOG_LEVEL")
	}
	t.Setenv(EnvWorkerLogLevel, "")
	t.Setenv(EnvLevel, "error")
	logger, closer, _, err := SetupWorker()
	if err != nil {
		t.Fatalf("SetupWorker: %v", err)
	}
	defer closer.Close()
	logger.Warn("below threshold")
	data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%s.worker-%d", FilePrefix, os.Getpid())))
	if err != nil {
		t.Fatalf("read worker file: %v", err)
	}
	if strings.Contains(string(data), "below threshold") {
		t.Errorf("VIVY_LOG_LEVEL fallback not applied: %s", data)
	}
}
