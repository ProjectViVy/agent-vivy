package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
)

func TestReplayWorkValidatesPersistedLifecycleAcrossPages(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		arg   any
		want  error
	}{
		{
			name:  "invalid returned payload",
			query: "UPDATE session_work_events SET payload = ? WHERE session_id = 'sess-replay' AND work_seq = 2",
			arg:   `{"Mutation":{"SessionID":"sess-replay","Goal":{"ID":"other-goal","Revision":1}}}`,
			want:  domain.ErrStaleGoalReference,
		},
		{
			name:  "invalid earlier schema",
			query: "UPDATE session_work_events SET payload_version = ? WHERE session_id = 'sess-replay' AND work_seq = 1",
			arg:   domain.WorkPayloadVersion + 1,
			want:  domain.ErrUnsupportedWorkPayloadVersion,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			b, err := Open(ctx, filepath.Join(t.TempDir(), "replay.db"))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			t.Cleanup(func() { _ = b.Close() })
			const sessionID domain.SessionID = "sess-replay"
			if err := b.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
				t.Fatalf("CreateSession: %v", err)
			}
			ref := domain.GoalRef{ID: "goal-1", Revision: 1}
			if _, err := b.CommitWork(ctx, domain.WorkMutation{
				SessionID: sessionID, RequestID: "create", RequestHash: "hash-create",
				Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "ship it", MaxRounds: 2,
			}); err != nil {
				t.Fatalf("CommitWork create: %v", err)
			}
			if _, err := b.CommitWork(ctx, domain.WorkMutation{
				SessionID: sessionID, ExpectedVersion: 1, RequestID: "pause", RequestHash: "hash-pause",
				Kind: domain.WorkEventGoalPaused, Goal: ref,
			}); err != nil {
				t.Fatalf("CommitWork pause: %v", err)
			}
			page, cursor, err := b.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 1)
			if err != nil || len(page) != 1 || page[0].Seq != 1 || cursor.Version != 1 {
				t.Fatalf("ReplayWork valid first page = %+v / %+v, %v; want seq 1", page, cursor, err)
			}
			page, _, err = b.ReplayWork(ctx, sessionID, cursor, 1)
			if err != nil || len(page) != 1 || page[0].Seq != 2 {
				t.Fatalf("ReplayWork valid second page = %+v, %v; want seq 2", page, err)
			}
			if _, err := b.db.ExecContext(ctx, tc.query, tc.arg); err != nil {
				t.Fatalf("corrupt persisted work row: %v", err)
			}
			if _, err := b.ReadWork(ctx, sessionID); !errors.Is(err, tc.want) {
				t.Fatalf("ReadWork error = %v, want %v", err, tc.want)
			}
			if tc.name == "invalid earlier schema" {
				cursor = domain.WorkState{SessionID: sessionID}
			}
			if _, _, err := b.ReplayWork(ctx, sessionID, cursor, 1); !errors.Is(err, tc.want) {
				t.Fatalf("ReplayWork error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestReplayWorkReadsOnlyRequestedPage(t *testing.T) {
	ctx := context.Background()
	b, err := Open(ctx, filepath.Join(t.TempDir(), "bounded-replay.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	const sessionID domain.SessionID = "sess-bounded-replay"
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-1", Revision: 1}
	for i, mutation := range []domain.WorkMutation{
		{Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "ship it", MaxRounds: 2},
		{Kind: domain.WorkEventGoalPaused, Goal: ref},
		{Kind: domain.WorkEventGoalResumed, Goal: ref},
	} {
		mutation.SessionID = sessionID
		mutation.ExpectedVersion = domain.WorkVersion(i)
		mutation.RequestID = []string{"create", "pause", "resume"}[i]
		mutation.RequestHash = mutation.RequestID
		if _, err := b.CommitWork(ctx, mutation); err != nil {
			t.Fatalf("CommitWork %s: %v", mutation.Kind, err)
		}
	}
	if _, err := b.db.ExecContext(ctx,
		"UPDATE session_work_events SET payload_version = ? WHERE session_id = ? AND work_seq = 3",
		domain.WorkPayloadVersion+1, sessionID); err != nil {
		t.Fatalf("corrupt future page: %v", err)
	}
	cursor := domain.WorkState{SessionID: sessionID}
	for _, seq := range []domain.WorkSeq{1, 2} {
		events, next, err := b.ReplayWork(ctx, sessionID, cursor, 1)
		if err != nil || len(events) != 1 || events[0].Seq != seq || next.Version != domain.WorkVersion(seq) {
			t.Fatalf("ReplayWork after %d = %+v / %+v, %v; want only seq %d", cursor.Version, events, next, err, seq)
		}
		cursor = next
	}
	if _, _, err := b.ReplayWork(ctx, sessionID, cursor, 1); !errors.Is(err, domain.ErrUnsupportedWorkPayloadVersion) {
		t.Fatalf("ReplayWork corrupt third page = %v, want unsupported version", err)
	}
}

func TestReplayWorkRejectsMismatchedCursorAndGap(t *testing.T) {
	ctx := context.Background()
	b, err := Open(ctx, filepath.Join(t.TempDir(), "cursor-replay.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	const sessionID domain.SessionID = "sess-cursor-replay"
	if err := b.CreateSession(ctx, domain.Session{ID: sessionID, CreatedAt: 1}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	ref := domain.GoalRef{ID: "goal-1", Revision: 1}
	for i, mutation := range []domain.WorkMutation{
		{Kind: domain.WorkEventGoalCreated, Goal: ref, Objective: "ship it", MaxRounds: 2},
		{Kind: domain.WorkEventGoalPaused, Goal: ref},
		{Kind: domain.WorkEventGoalResumed, Goal: ref},
	} {
		mutation.SessionID = sessionID
		mutation.ExpectedVersion = domain.WorkVersion(i)
		mutation.RequestID = []string{"create", "pause", "resume"}[i]
		mutation.RequestHash = mutation.RequestID
		if _, err := b.CommitWork(ctx, mutation); err != nil {
			t.Fatalf("CommitWork %s: %v", mutation.Kind, err)
		}
	}
	_, cursor, err := b.ReplayWork(ctx, sessionID, domain.WorkState{SessionID: sessionID}, 1)
	if err != nil || cursor.Version != 1 {
		t.Fatalf("ReplayWork first page cursor = %+v, %v", cursor, err)
	}
	if _, _, err := b.ReplayWork(ctx, "other-session", cursor, 1); !errors.Is(err, domain.ErrInvalidWorkCursor) {
		t.Fatalf("ReplayWork cross-session cursor = %v, want ErrInvalidWorkCursor", err)
	}
	future := cursor
	future.Version = 99
	if _, _, err := b.ReplayWork(ctx, sessionID, future, 1); !errors.Is(err, domain.ErrInvalidWorkCursor) {
		t.Fatalf("ReplayWork nonexistent cursor version = %v, want ErrInvalidWorkCursor", err)
	}
	if _, err := b.db.ExecContext(ctx, "DELETE FROM session_work_events WHERE session_id = ? AND work_seq = 2", sessionID); err != nil {
		t.Fatalf("remove middle work row: %v", err)
	}
	if _, _, err := b.ReplayWork(ctx, sessionID, cursor, 1); !errors.Is(err, domain.ErrNonContiguousWorkSeq) {
		t.Fatalf("ReplayWork gap = %v, want ErrNonContiguousWorkSeq", err)
	}
}
