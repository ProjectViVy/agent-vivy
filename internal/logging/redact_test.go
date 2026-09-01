package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactVocabulary(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"key sk-live-abcdefghijkl end", "key [REDACTED_SECRET] end"},
		{"Authorization: Bearer abcdefghijklmnop", "Authorization: [REDACTED_SECRET]"},
		{"mail alice@example.com", "mail [REDACTED_EMAIL]"},
		// Ordinary fields survive, including near-misses without a token
		// boundary ("task-" has no \b before "sk-").
		{"run r-1 model claude-3 path /tmp/a.txt task-123456789012",
			"run r-1 model claude-3 path /tmp/a.txt task-123456789012"},
	}
	for _, tc := range cases {
		if got := Redact(tc.in); got != tc.want {
			t.Errorf("Redact(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRedactingHandlerMessageAndAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newRedactingHandler(slog.NewJSONHandler(&buf, nil)))
	logger = logger.With("password", "hunter2")
	logger.Info("call with sk-live-abcdefghijkl now",
		"api_key", "supersecret-value",
		"run_id", "r-1",
		slog.Group("channel",
			slog.String("bot_token", "123456:ABCdefGHIjklMNOpqrSTUvwx"),
			slog.String("name", "telegram"),
		),
	)
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("parse log line: %v", err)
	}
	if msg, _ := line["msg"].(string); strings.Contains(msg, "sk-live") {
		t.Fatalf("message kept the token: %q", msg)
	}
	if got, _ := line["api_key"].(string); got != valueMarker {
		t.Fatalf("api_key = %q, want %q", got, valueMarker)
	}
	if got, _ := line["password"].(string); got != valueMarker {
		t.Fatalf("password (WithAttrs) = %q, want %q", got, valueMarker)
	}
	if got, _ := line["run_id"].(string); got != "r-1" {
		t.Fatalf("run_id = %q, want untouched r-1", got)
	}
	channel, _ := line["channel"].(map[string]any)
	if channel == nil {
		t.Fatalf("group missing: %s", buf.String())
	}
	if got, _ := channel["bot_token"].(string); got != valueMarker {
		t.Fatalf("channel.bot_token = %q, want %q", got, valueMarker)
	}
	if got, _ := channel["name"].(string); got != "telegram" {
		t.Fatalf("channel.name = %q, want untouched telegram", got)
	}
}

func TestRedactingHandlerAuthorizationKeyCaseInsensitive(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newRedactingHandler(slog.NewJSONHandler(&buf, nil)))
	logger.Warn("rejected", "Authorization", "Bearer abcdefghijklmnop")
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("parse log line: %v", err)
	}
	if got, _ := line["Authorization"].(string); got != valueMarker {
		t.Fatalf("Authorization = %q, want %q", got, valueMarker)
	}
}

// TestSetupSinkRedacts is the real-path acceptance: through the product
// init path, a credential interpolated into a log message never reaches
// the file sink.
func TestSetupSinkRedacts(t *testing.T) {
	dir := t.TempDir()
	logger, _, closer, err := Setup(Options{Dir: dir, Level: "info", Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	logger.Error("provider call failed", "err", "401 for sk-live-abcdefghijkl")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, FilePrefix+"*"))
	if err != nil || len(files) == 0 {
		t.Fatalf("log files = %v err=%v", files, err)
	}
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "sk-live") {
			t.Fatalf("%s kept the credential", name)
		}
		if !strings.Contains(string(data), secretMarker) {
			t.Fatalf("%s lacks the redaction marker", name)
		}
	}
}
