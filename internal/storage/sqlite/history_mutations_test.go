package sqlite

import (
	"context"
	"testing"

	"agent-vivy/internal/domain"
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
	if _, err := b.db.ExecContext(ctx, `CREATE TRIGGER fail_history_event BEFORE INSERT ON run_events BEGIN SELECT RAISE(ABORT, 'event failed'); END`); err != nil {
		t.Fatal(err)
	}
	child := domain.Session{ID: "child", Title: "child", CreatedAt: 1}
	message := domain.Message{ID: "copy", SessionID: "child", Role: domain.RoleUser, Content: "hello", CreatedAt: 1}
	event := domain.RunEvent{RunID: "fork-event", Type: domain.EventSessionForked, CreatedAt: 2, PayloadVersion: 1, Payload: []byte(`{}`)}
	if _, err := b.CommitSessionFork(ctx, child, []domain.Message{message}, nil, []domain.RunEvent{event}); err == nil {
		t.Fatal("expected injected event failure")
	}
	if _, err := b.GetSession(ctx, "child"); err != storage.ErrNotFound {
		t.Fatalf("child survived rollback: %v", err)
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
