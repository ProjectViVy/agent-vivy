package app

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

// Expose exactly the ordinary RunStore interface so NewService cannot discover
// PrimaryRunStore through the concrete backend in this continuity-only embedder.
type issue58NarrowRuns struct{ storage.RunStore }

type issue58ObservedJournal struct {
	storage.Journal
	writes atomic.Int64
}

func (j *issue58ObservedJournal) Append(ctx context.Context, commit storage.Commit) (domain.EventSeq, error) {
	j.writes.Add(1)
	return j.Journal.Append(ctx, commit)
}

type issue58ObservedModel struct{ calls atomic.Int64 }

func (m *issue58ObservedModel) Stream(ctx context.Context, input []*domain.Message) (domain.Stream[*domain.Message], error) {
	m.calls.Add(1)
	return testsupport.NewEchoModel().Stream(ctx, input)
}

func TestIssue58ContinuityOnlyCompositionUsesSelectedAdmission(t *testing.T) {
	for _, test := range []struct {
		name                 string
		eligible, continuity bool
	}{
		{name: "eligible ordinary Run rejects", eligible: true},
		{name: "unshared ordinary Run keeps fallback"},
		{name: "eligible continuity request commits pins", eligible: true, continuity: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			b, err := sqlite.Open(ctx, t.TempDir()+"/core.db")
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			conformance.Issue58Identity(t, b)
			source := conformance.Issue58Source("continuity-only-session", true)
			if err := b.CreateSession(ctx, domain.Session{ID: source.SessionID}); err != nil {
				t.Fatal(err)
			}
			var main cognitivecontract.MainConversationResolver
			if test.eligible {
				main = &issue58Main{binding: cognitivecontract.MainConversationBinding{SubjectID: source.SubjectID, DomainID: source.DomainID, ConversationID: source.ConversationID, ActiveSessionID: source.SessionID, PolicyID: source.PolicyID, ToolPolicyRef: source.ToolPolicyRef, SchemaVersion: source.SchemaVersion, Grant: source.Grant}}
			}
			ts, err := tools.Builtin(b).Resolve(nil)
			if err != nil {
				t.Fatal(err)
			}
			model := &issue58ObservedModel{}
			eng, err := runtime.NewEngine(ctx, runtime.WrapModel(model), ts, runtime.EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
			if err != nil {
				t.Fatal(err)
			}
			journal := &issue58ObservedJournal{Journal: b}
			var out, errw bytes.Buffer
			svc := runtime.NewService(eng, "test", "test", runtime.ServiceDeps{Journal: journal, Runs: issue58NarrowRuns{RunStore: b}, Messages: b, Sessions: b, Continuity: b, Sink: newHeadlessSink(&out, &errw), MainConversation: main, Recap: issue58FixtureRecap(t, main)})
			svc.SetCatalog(issue58FixtureCatalog())
			defer func() { svc.CancelAll(); svc.WaitIdle(context.Background()) }()
			var rid domain.RunID
			var runErr error
			if test.continuity {
				rid, runErr = svc.RunWithOptions(ctx, source.SessionID, "continuity request", runtime.RunOptions{Continuity: &domain.ContinuityInput{RequestID: "subject-continuity-only"}})
			} else {
				rid, runErr = svc.Run(ctx, source.SessionID, "ordinary request")
			}
			if !svc.WaitIdle(ctx) {
				t.Fatal("run did not settle")
			}
			messages, err := b.ListMessages(ctx, source.SessionID)
			if err != nil {
				t.Fatal(err)
			}
			runs, err := b.ListRunsBySession(ctx, source.SessionID)
			if err != nil {
				t.Fatal(err)
			}
			if test.eligible && !test.continuity {
				if !errors.Is(runErr, storage.ErrSubjectBinding) || rid != "" {
					t.Errorf("eligible ordinary Run bypassed atomic subject admission: %s, %v", rid, runErr)
				}
				if len(messages) != 0 || len(runs) != 0 || journal.writes.Load() != 0 || model.calls.Load() != 0 {
					t.Errorf("rejection followed side effects: messages=%d runs=%d Journal writes=%d provider calls=%d", len(messages), len(runs), journal.writes.Load(), model.calls.Load())
				}
				if _, err := b.GetSubjectSource(ctx, source.SessionID); !errors.Is(err, storage.ErrNotFound) {
					t.Errorf("failed admission persisted misleading source: %v", err)
				}
				return
			}
			if runErr != nil || rid == "" || len(messages) != 2 || len(runs) != 1 || model.calls.Load() == 0 {
				t.Fatalf("supported selected path did not run: %s %v messages=%d runs=%d provider calls=%d", rid, runErr, len(messages), len(runs), model.calls.Load())
			}
			pin, err := b.GetRunSubjectSource(ctx, rid)
			if test.eligible {
				if err != nil || pin.SubjectHistoryBinding != source.SubjectHistoryBinding {
					t.Fatalf("real continuity admission did not persist subject pins: %+v %v", pin, err)
				}
			} else if !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("unshared fallback acquired subject pins: %+v %v", pin, err)
			}
		})
	}
}
