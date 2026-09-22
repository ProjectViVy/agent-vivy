package sqlite

import (
	"context"
	"testing"

	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

func TestCommitSessionRewindRollsBackMarkerWhenEventFails(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "s", Title: "s", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.ExecContext(ctx, `CREATE TRIGGER fail_history_event BEFORE INSERT ON run_events BEGIN SELECT RAISE(ABORT, 'event failed'); END`); err != nil {
		t.Fatal(err)
	}
	marker := storage.SessionTruncation{SessionID: "s", CutoffMessageID: "m", TailMessageID: "m", Reason: storage.TruncationRewind, CreatedAt: 2}
	_, err := b.CommitSessionRewind(ctx, marker, domain.RunEvent{RunID: "tr", Type: domain.EventSessionTruncated, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected injected event failure")
	}
	if _, ok, err := b.LatestSessionTruncation(ctx, "s"); err != nil || ok {
		t.Fatalf("marker survived rollback: ok=%v err=%v", ok, err)
	}
}

func TestCommitSessionForkRollsBackChildWhenEventFails(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "source", Title: "source", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	definition, err := b.CreateCustomMask(ctx, mask.CreateRequest{
		OperationID: "00000000-0000-4000-8000-000000000091",
		Name: "rollback mask", Body: "copied before the injected failure",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetMaskSelection(ctx, mask.SetSelectionRequest{SessionID: "source", MaskID: definition.ID, ExpectedRevision: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.ExecContext(ctx, `CREATE TRIGGER fail_history_event BEFORE INSERT ON run_events BEGIN SELECT RAISE(ABORT, 'event failed'); END`); err != nil {
		t.Fatal(err)
	}
	child := domain.Session{ID: "child", Title: "child", CreatedAt: 1}
	message := domain.Message{ID: "copy", SessionID: "child", Role: domain.RoleUser, Content: "hello", CreatedAt: 1}
	markers := []storage.SessionTruncation{{SessionID: "source", Reason: storage.TruncationFork, ForkSessionID: "child"}}
	event := domain.RunEvent{RunID: "fork-event", Type: domain.EventSessionForked, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)}
	if _, err := b.CommitSessionFork(ctx, child, []domain.Message{message}, markers, []domain.RunEvent{event}); err == nil {
		t.Fatal("expected injected event failure")
	}
	if _, err := b.GetSession(ctx, "child"); err != storage.ErrNotFound {
		t.Fatalf("child survived rollback: %v", err)
	}
	var copied int
	if err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_mask_selections WHERE session_id = ?`, "child").Scan(&copied); err != nil {
		t.Fatal(err)
	}
	if copied != 0 {
		t.Fatalf("copied mask selection survived rollback: %d", copied)
	}
}

func TestCommitSessionEditRollsBackEveryRowWhenEventFails(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "s", Title: "s", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.ExecContext(ctx, `CREATE TRIGGER fail_history_event BEFORE INSERT ON run_events BEGIN SELECT RAISE(ABORT, 'event failed'); END`); err != nil {
		t.Fatal(err)
	}
	marker := storage.SessionTruncation{SessionID: "s", CutoffMessageID: "old", TailMessageID: "old", Reason: storage.TruncationEdit, CreatedAt: 2}
	message := domain.Message{ID: "new", SessionID: "s", RunID: "run", Role: domain.RoleUser, Content: "replacement", CreatedAt: 2}
	run := domain.Run{ID: "run", SessionID: "s", Status: domain.RunAccepted, CreatedAt: 2}
	event := domain.RunEvent{RunID: "run", Type: domain.EventRunStarted, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)}
	if _, err := b.CommitSessionEdit(ctx, marker, message, run, event); err == nil {
		t.Fatal("expected injected event failure")
	}
	if markers, err := b.ListViewTruncations(ctx, "s"); err != nil || len(markers) != 0 {
		t.Fatalf("markers survived: %+v %v", markers, err)
	}
	if messages, err := b.ListMessages(ctx, "s"); err != nil || len(messages) != 0 {
		t.Fatalf("messages survived: %+v %v", messages, err)
	}
	if _, err := b.GetRun(ctx, "run"); err != storage.ErrNotFound {
		t.Fatalf("run survived: %v", err)
	}
}
