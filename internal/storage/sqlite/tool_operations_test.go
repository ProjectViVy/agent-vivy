package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func toolOperationFixture(t *testing.T, path string) (*Backend, domain.RunID) {
	t.Helper()
	backend, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ctx := context.Background()
	sessionID := domain.SessionID("session-tool-operation")
	runID := domain.RunID("run-tool-operation")
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, Title: "operation", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{{Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)}}}); err != nil {
		t.Fatal(err)
	}
	return backend, runID
}

func operationDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func testToolOperation(runID domain.RunID, id string) domain.ToolOperation {
	args := []byte(`{"text":"same arguments"}`)
	return domain.ToolOperation{
		RunID: runID, OperationID: id, ToolName: "write_note",
		RequestDigest:            operationDigest("request:" + string(args)),
		MiddlewareInputArguments: append([]byte(nil), args...),
		ArgumentsDigest:          operationDigest(string(args)), EffectiveArguments: args,
	}
}

func TestToolOperationAdmissionIdentityAndConcurrentClaim(t *testing.T) {
	backend, runID := toolOperationFixture(t, filepath.Join(t.TempDir(), "operations.db"))
	store, ok := any(backend).(storage.ToolOperationStore)
	if !ok {
		t.Fatal("SQLite backend does not implement ToolOperationStore")
	}
	ctx := context.Background()
	const workers = 24
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	admitErr := error(nil)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, inserted, _, err := store.AdmitToolOperation(ctx, testToolOperation(runID, "call-stable"))
			mu.Lock()
			defer mu.Unlock()
			if inserted {
				created++
			}
			if err != nil && admitErr == nil {
				admitErr = err
			}
		}()
	}
	wg.Wait()
	if admitErr != nil || created != 1 {
		t.Fatalf("concurrent admission: created=%d err=%v, want one durable admission", created, admitErr)
	}

	conflict := testToolOperation(runID, "call-stable")
	conflict.EffectiveArguments = []byte(`{"text":"changed"}`)
	conflict.ArgumentsDigest = operationDigest(string(conflict.EffectiveArguments))
	if _, _, _, err := store.AdmitToolOperation(ctx, conflict); !errors.Is(err, storage.ErrToolOperationConflict) {
		t.Fatalf("same key with changed effective arguments = %v, want ErrToolOperationConflict", err)
	}
	conflict = testToolOperation(runID, "call-stable")
	conflict.MiddlewareInputArguments = []byte(`{"text":"changed before middleware"}`)
	if _, _, _, err := store.AdmitToolOperation(ctx, conflict); !errors.Is(err, storage.ErrToolOperationConflict) {
		t.Fatalf("same key with changed middleware input = %v, want ErrToolOperationConflict", err)
	}

	// Identical arguments do not collapse two distinct logical tool calls.
	second := testToolOperation(runID, "call-distinct")
	if _, inserted, _, err := store.AdmitToolOperation(ctx, second); err != nil || !inserted {
		t.Fatalf("distinct same-argument admission: inserted=%v err=%v", inserted, err)
	}

	claimed := 0
	var claimMu sync.Mutex
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, acquired, _, err := store.ClaimToolOperation(ctx, runID, "call-stable", string(rune('a'+i)))
			if err != nil {
				t.Errorf("ClaimToolOperation: %v", err)
				return
			}
			if acquired {
				claimMu.Lock()
				claimed++
				claimMu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if claimed != 1 {
		t.Fatalf("concurrent claims acquired=%d, want exactly one", claimed)
	}
	if got, err := store.GetToolOperation(ctx, runID, "call-stable"); err != nil || got.State != domain.ToolOperationClaimed {
		t.Fatalf("claimed operation = %+v, err=%v", got, err)
	}
	if got, err := store.GetToolOperation(ctx, runID, "call-distinct"); err != nil || got.State != domain.ToolOperationAdmitted {
		t.Fatalf("second distinct operation = %+v, err=%v", got, err)
	}
}

func TestToolOperationCompletionFailureAndRestartRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.db")
	backend, runID := toolOperationFixture(t, path)
	store, ok := any(backend).(storage.ToolOperationStore)
	if !ok {
		t.Fatal("SQLite backend does not implement ToolOperationStore")
	}
	ctx := context.Background()
	owner := "worker-1"
	if _, _, _, err := store.AdmitToolOperation(ctx, testToolOperation(runID, "call-uncertain")); err != nil {
		t.Fatal(err)
	}
	if _, claimed, _, err := store.ClaimToolOperation(ctx, runID, "call-uncertain", owner); err != nil || !claimed {
		t.Fatalf("claim uncertain operation: claimed=%v err=%v", claimed, err)
	}
	if _, _, _, err := store.AdmitToolOperation(ctx, testToolOperation(runID, "call-complete")); err != nil {
		t.Fatal(err)
	}
	if _, claimed, _, err := store.ClaimToolOperation(ctx, runID, "call-complete", owner); err != nil || !claimed {
		t.Fatalf("claim completed operation: claimed=%v err=%v", claimed, err)
	}

	_, err := backend.db.ExecContext(ctx, `CREATE TRIGGER fail_operation_completion BEFORE INSERT ON run_events
		WHEN NEW.type = 'tool.operation' AND json_extract(NEW.payload, '$.state') = 'completed'
		BEGIN SELECT RAISE(ABORT, 'completion write failed'); END`)
	if err != nil {
		t.Fatalf("install completion failure trigger: %v", err)
	}
	if _, _, err := store.CompleteToolOperation(ctx, runID, "call-complete", owner, "durable result", ""); err == nil {
		t.Fatal("completion write failure was ignored")
	}
	if got, err := store.GetToolOperation(ctx, runID, "call-complete"); err != nil || got.State != domain.ToolOperationClaimed {
		t.Fatalf("failed completion changed operation state: %+v err=%v", got, err)
	}
	if _, err := backend.db.ExecContext(ctx, `DROP TRIGGER fail_operation_completion`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CompleteToolOperation(ctx, runID, "call-complete", owner, "durable result", ""); err != nil {
		t.Fatalf("CompleteToolOperation after trigger removal: %v", err)
	}

	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	recovered, ok := any(reopened).(storage.ToolOperationStore)
	if !ok {
		t.Fatal("reopened SQLite backend does not implement ToolOperationStore")
	}
	uncertain, err := recovered.GetToolOperation(ctx, runID, "call-uncertain")
	if err != nil || uncertain.State != domain.ToolOperationClaimed {
		t.Fatalf("claimed operation after restart = %+v, err=%v; want fenced claimed state", uncertain, err)
	}
	if _, acquired, _, err := recovered.ClaimToolOperation(ctx, runID, "call-uncertain", "worker-after-restart"); err != nil || acquired {
		t.Fatalf("claimed operation was replayable after restart: acquired=%v err=%v", acquired, err)
	}
	completed, err := recovered.GetToolOperation(ctx, runID, "call-complete")
	if err != nil || completed.State != domain.ToolOperationCompleted || completed.Result != "durable result" {
		t.Fatalf("completed operation after restart = %+v, err=%v", completed, err)
	}

	iterator, err := reopened.Replay(ctx, runID, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = iterator.Close() }()
	completedEvents := 0
	for iterator.Next() {
		var payload struct {
			State string `json:"state"`
		}
		event := iterator.Value().Event
		if event.Type == domain.EventToolOperation && json.Unmarshal(event.Payload, &payload) == nil && payload.State == string(domain.ToolOperationCompleted) {
			completedEvents++
		}
	}
	if iterator.Err() != nil {
		t.Fatal(iterator.Err())
	}
	if completedEvents != 1 {
		t.Fatalf("durable completion events = %d, want exactly 1", completedEvents)
	}
}
