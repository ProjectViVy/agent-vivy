package divacognitive

import (
	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/sdk/port/observer"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestMemoryLoopCaptureRejoinsActualLegacyReceiptAfterReopen(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.CreateSession(ctx, domain.Session{ID: "actual-legacy-session"}); err != nil {
		t.Fatal(err)
	}
	run := domain.Run{ID: "actual-legacy-run", SessionID: "actual-legacy-session", Kind: domain.RunKindPrimary, Status: domain.RunCompleted}
	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendMessage(ctx, domain.Message{ID: "actual-legacy-user", SessionID: run.SessionID, RunID: run.ID, Role: domain.RoleUser, Content: "original durable user fact", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	input := cognitivecontract.FactoryInput{Config: config.Config{Storage: config.Storage{DataDir: t.TempDir()}}, GenerationID: "receipt-recovery-test"}
	bundle, err := Open(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	old, err := bundle.Sink().Capture(ctx, runtime.CognitiveCapture{SessionID: string(run.SessionID), RunID: run.ID, EventID: string(run.ID) + ":7", Phase: "completed", Content: "收到", OccurredAt: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := bundle.Close(); err != nil {
		t.Fatal(err)
	}
	bundle, err = Open(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	defer bundle.Close()
	provider := runtime.NewCognitiveCaptureProvider(store, bundle.Sink(), nil)
	event := observer.NewRunEvent(observer.NewEventID(string(run.ID), 7), "run.completed", 10, json.RawMessage(`{"summary":"收到","session_id":"actual-legacy-session"}`))
	receipt, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil || receipt.ReceiptID != old.IngestionID {
		t.Fatalf("actual accepted legacy receipt stranded: old=%+v got=%+v err=%v", old, receipt, err)
	}
	// The original changed-payload conflict contract remains enforced.
	_, err = bundle.Sink().Capture(ctx, runtime.CognitiveCapture{SessionID: string(run.SessionID), RunID: run.ID, EventID: string(run.ID) + ":7", Phase: "completed", Content: "different source", OccurredAt: 10})
	if err == nil {
		t.Fatal("receipt lookup silently weakened changed-payload conflict")
	}
}
