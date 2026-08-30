package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// E3 (AS-9): a secret held only in the environment must never reach the
// SQLite file or any persisted event payload. The test path never touches
// a provider, so any appearance of the canary below would prove that a
// runtime or storage component snapshots the environment.

const secretCanary = "sk-canary-e3-must-not-leak"

func TestSecretsNeverReachStorage(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", secretCanary)
	t.Setenv("ANTHROPIC_API_KEY", secretCanary)
	ctx := context.Background()

	dbPath := filepath.Join(t.TempDir(), "leak.db")
	backend, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	model := NewScriptedModel(schema.AssistantMessage("a complete reply", nil))
	eng, err := NewEngine(ctx, model, ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Sink: newTestSink(),
	})

	runID, err := svc.Run(ctx, "sess-leak", "audit turn")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	// Every persisted event payload must stay canary-free.
	events := replayAll(t, backend, runID)
	if len(events) == 0 {
		t.Fatal("journal is empty after a completed run")
	}
	for _, ev := range events {
		if bytes.Contains(ev.Payload, []byte(secretCanary)) {
			t.Fatalf("event %s leaked the secret canary", ev.Type)
		}
	}

	// Close first so all pages are flushed, then scan the raw file: this
	// covers runs, sessions, messages, journal and blobs alike.
	if err := backend.Close(); err != nil {
		t.Fatalf("close backend: %v", err)
	}
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read sqlite file: %v", err)
	}
	if bytes.Contains(raw, []byte(secretCanary)) {
		t.Fatal("the SQLite file contains the secret canary")
	}

	// Reopen sanity: the journal stays readable after flush.
	reopened, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer reopened.Close()
	if again := replayAll(t, reopened, runID); len(again) != len(events) {
		t.Fatalf("journal after reopen = %d events, want %d", len(again), len(events))
	}
}
