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
			page, err := b.ReplayWork(ctx, sessionID, 1, 1)
			if err != nil || len(page) != 1 || page[0].Seq != 2 {
				t.Fatalf("ReplayWork valid page = %+v, %v; want seq 2", page, err)
			}
			if _, err := b.db.ExecContext(ctx, tc.query, tc.arg); err != nil {
				t.Fatalf("corrupt persisted work row: %v", err)
			}
			if _, err := b.ReadWork(ctx, sessionID); !errors.Is(err, tc.want) {
				t.Fatalf("ReadWork error = %v, want %v", err, tc.want)
			}
			if _, err := b.ReplayWork(ctx, sessionID, 1, 1); !errors.Is(err, tc.want) {
				t.Fatalf("ReplayWork error = %v, want %v", err, tc.want)
			}
		})
	}
}
