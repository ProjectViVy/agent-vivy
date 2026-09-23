package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
)

func TestRecoveredGoalRefUsesFoldedWorkReplay(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "goal-recovery.db"))
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID domain.SessionID = "sess-goal-recovery"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-1", Revision: 1}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, RequestID: "create", RequestHash: "hash-create",
		Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "ship it", MaxRounds: 2,
	}); err != nil {
		t.Fatalf("CommitWork create: %v", err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 1, RequestID: "admit", RequestHash: "hash-admit",
		Kind:      domain.WorkEventGoalRoundAdmitted,
		Admission: domain.GoalRunAdmission{SessionID: sessionID, Goal: ref, Round: 1, RunID: "run-1"},
	}); err != nil {
		t.Fatalf("CommitWork admit: %v", err)
	}
	svc := &Service{deps: ServiceDeps{Work: backend}}
	got, ok := svc.recoveredGoalRef(ctx, domain.Run{ID: "run-1", SessionID: sessionID})
	if !ok || got != ref {
		t.Fatalf("recoveredGoalRef = %+v, %t; want %+v", got, ok, ref)
	}
}

func TestRecoveredGoalRefRejectsCorruptLaterReplayPage(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "goal-recovery-later-page.db")
	backend, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const sessionID domain.SessionID = "sess-goal-recovery-later-page"
	if err := backend.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-1", Revision: 1}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, RequestID: "create", RequestHash: "hash-create",
		Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "ship it", MaxRounds: 2,
	}); err != nil {
		t.Fatalf("CommitWork create: %v", err)
	}
	if _, err := backend.CommitWork(ctx, domain.WorkMutation{
		SessionID: sessionID, ExpectedVersion: 1, RequestID: "admit", RequestHash: "hash-admit",
		Kind:      domain.WorkEventGoalRoundAdmitted,
		Admission: domain.GoalRunAdmission{SessionID: sessionID, Goal: ref, Round: 1, RunID: "run-1"},
	}); err != nil {
		t.Fatalf("CommitWork admit: %v", err)
	}

	// The admission is on page one. Valid pause/resume events fill that page;
	// seq 1001 begins page two and will then be corrupted.
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open fixture SQL: %v", err)
	}
	defer func() { _ = db.Close() }()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin fixture SQL: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	for seq := 3; seq <= goalRecoveryPageSize+1; seq++ {
		kind := domain.WorkEventGoalPaused
		if seq%2 == 0 {
			kind = domain.WorkEventGoalResumed
		}
		requestID := fmt.Sprintf("transition-%d", seq)
		mutation := domain.WorkMutation{
			SessionID: sessionID, ExpectedVersion: domain.WorkVersion(seq - 1),
			RequestID: requestID, RequestHash: requestID, Kind: kind, Goal: ref,
		}
		payload, err := json.Marshal(struct {
			Mutation  domain.WorkMutation
			Admission domain.GoalRunAdmission
		}{Mutation: mutation})
		if err != nil {
			t.Fatalf("marshal transition %d: %v", seq, err)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO session_work_events (session_id, work_seq, kind, payload_version, request_id, request_hash, created_at, payload) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			sessionID, seq, kind, domain.WorkPayloadVersion, requestID, requestID, int64(seq), payload); err != nil {
			t.Fatalf("insert transition %d: %v", seq, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit fixture SQL: %v", err)
	}

	svc := &Service{deps: ServiceDeps{Work: backend}}
	got, ok := svc.recoveredGoalRef(ctx, domain.Run{ID: "run-1", SessionID: sessionID})
	if !ok || got != ref {
		t.Fatalf("recoveredGoalRef across valid pages = %+v, %t; want %+v", got, ok, ref)
	}
	if _, err := db.ExecContext(ctx,
		"UPDATE session_work_events SET payload_version = ? WHERE session_id = ? AND work_seq = ?",
		domain.WorkPayloadVersion+1, sessionID, goalRecoveryPageSize+1); err != nil {
		t.Fatalf("corrupt later page: %v", err)
	}
	got, ok = svc.recoveredGoalRef(ctx, domain.Run{ID: "run-1", SessionID: sessionID})
	if ok || got != (domain.GoalRef{}) {
		t.Fatalf("recoveredGoalRef across corrupt later page = %+v, %t; want no authority", got, ok)
	}
}
