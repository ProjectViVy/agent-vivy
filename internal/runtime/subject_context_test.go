package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
)

type issue58ContextService interface {
	CaptureSubjectContext(context.Context, domain.SessionID) (storage.SubjectContextSnapshot, error)
}

func captureIssue58(t *testing.T, s *Service) issue58ContextService {
	t.Helper()
	q, ok := any(s).(issue58ContextService)
	if !ok {
		t.Fatal("service cannot capture subject context after domain repair")
	}
	return q
}
func issue58Sources(svc *Service, sources map[domain.SessionID]storage.SubjectSourceBinding) {
	issue58AuthorizeReader(svc, storage.SubjectSourceBinding{})
	svc.deps.SubjectSources = func(_ context.Context, sid domain.SessionID) (*storage.SubjectSourceBinding, error) {
		s, ok := sources[sid]
		if !ok {
			return nil, nil
		}
		return &s, nil
	}
}
func TestIssue58SubjectDomainPendingRepair(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "pending.db")
	svc, b := openD15CrashService(t, path)
	conformance.Issue58Identity(t, b)
	sources := map[domain.SessionID]storage.SubjectSourceBinding{}
	for _, name := range []string{"pending-archived-A", "main-reader"} {
		source := conformance.Issue58Source(domain.SessionID(name), name == "main-reader")
		sources[source.SessionID] = source
		mustCreateSession(t, b, source.SessionID)
		a := conformance.Issue58Admission(source, name+"-run")
		if _, err := b.CommitRunAdmission(ctx, a); err != nil {
			t.Fatal(err)
		}
		m := newEventMapper(a.Run.ID, 64<<10)
		events := []domain.RunEvent{m.build(domain.EventModelDelta, payloadModelDelta{Delta: name + " exact final"})}
		completion := m.build(domain.EventModelCompleted, v2Completion(name+" exact final"))
		completion.PayloadVersion = 2
		events = append(events, completion, m.build(domain.EventRunCompleted, payloadRunCompleted{}))
		if _, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: events}); err != nil {
			t.Fatal(err)
		}
		if err := b.SetRunStatus(ctx, a.Run.ID, domain.RunCompleted); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.ArchiveRetainedSession(ctx, "pending-archived-A"); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	svc, b = openD15CrashService(t, path)
	t.Cleanup(func() { b.Close() })
	svc.deps.Sink = newTestSink()
	issue58Sources(svc, sources)
	q := captureIssue58(t, svc)
	snapshot, err := q.CaptureSubjectContext(ctx, "main-reader")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Through != 2 {
		t.Fatalf("domain repair omitted archived terminal source %+v", snapshot)
	}
	for _, source := range sources {
		rows, err := b.ListMessages(ctx, source.SessionID)
		if err != nil || len(rows) != 2 {
			t.Fatalf("projection not recovered for %s: %+v %v", source.SessionID, rows, err)
		}
	}
	again, err := q.CaptureSubjectContext(ctx, "main-reader")
	if err != nil || again.Through != 2 || again.DispositionDigest != snapshot.DispositionDigest {
		t.Fatalf("repair not idempotent %+v %v", again, err)
	}
	delete(sources, "pending-archived-A")
	deniedBeforeMarker, err := q.CaptureSubjectContext(ctx, "main-reader")
	if !errors.Is(err, storage.ErrSubjectBinding) || deniedBeforeMarker.Through != 0 {
		t.Fatalf("covered archived source without marker bypassed current grant %+v %v", deniedBeforeMarker, err)
	}
	restored := conformance.Issue58Source("pending-archived-A", false)
	sources[restored.SessionID] = restored
	restoredSnapshot, err := q.CaptureSubjectContext(ctx, "main-reader")
	if err != nil || restoredSnapshot.Through != 2 {
		t.Fatalf("exact restored grant not accepted %+v %v", restoredSnapshot, err)
	}
	changed := restored
	changed.Grant.Version++
	sources[restored.SessionID] = changed
	if denied, err := q.CaptureSubjectContext(ctx, "main-reader"); !errors.Is(err, storage.ErrSubjectBinding) || denied.Through != 0 {
		t.Fatalf("changed covered-source grant accepted %+v %v", denied, err)
	}
	delete(sources, "pending-archived-A")
	// A participant is checked even when archived and no new interaction exists:
	marker := storage.SessionTruncation{SessionID: "pending-archived-A", CutoffMessageID: "user_pending-archived-A-run", TailMessageID: "user_pending-archived-A-run", Reason: storage.TruncationRewind}
	if err := b.RecordSessionTruncation(ctx, marker); err != nil {
		t.Fatal(err)
	}
	denied, err := q.CaptureSubjectContext(ctx, "main-reader")
	if !errors.Is(err, storage.ErrSubjectBinding) || denied.Through != 0 {
		t.Fatalf("archived revocation leaked partial capture %+v %v", denied, err)
	}
}
func TestIssue58SubjectContextScope(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("runtime-context-source", false)
	mustCreateSession(t, b, source.SessionID)
	sources := map[domain.SessionID]storage.SubjectSourceBinding{source.SessionID: source}
	issue58Sources(svc, sources)
	in := conformance.Issue58Completed(t, b, source, "runtime-context-run")
	if _, err := b.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, in); err != nil {
		t.Fatal(err)
	}
	marker := storage.SessionTruncation{SessionID: source.SessionID, CutoffMessageID: "user_runtime-context-run", TailMessageID: "user_runtime-context-run", Reason: storage.TruncationRewind}
	if err := b.RecordSessionTruncation(ctx, marker); err != nil {
		t.Fatal(err)
	}
	q := captureIssue58(t, svc)
	snapshot, err := q.CaptureSubjectContext(ctx, "main-reader")
	if err != nil || len(snapshot.Dispositions) != 1 {
		t.Fatalf("capture %+v %v", snapshot, err)
	}
	changed := source
	changed.Grant.Version++
	sources[source.SessionID] = changed
	denied, err := q.CaptureSubjectContext(ctx, "main-reader")
	if !errors.Is(err, storage.ErrSubjectBinding) || denied.Through != 0 {
		t.Fatalf("changed source grant leaked %+v %v", denied, err)
	}
	if _, err := q.CaptureSubjectContext(ctx, source.SessionID); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("ordinary source read escalated: %v", err)
	}
}

func TestIssue58SubjectDomainPendingRepairFailure(t *testing.T) {
	for _, mode := range []string{"projection", "missing-terminal", "missing-status", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			svc, b, _ := newTestService(t, testsupport.NewEchoModel())
			conformance.Issue58Identity(t, b)
			sources := map[domain.SessionID]storage.SubjectSourceBinding{}
			issue58Sources(svc, sources)
			for _, id := range []string{"a-good-pending", "z-bad-pending"} {
				if mode == "missing-status" && id == "a-good-pending" {
					continue
				}
				source := conformance.Issue58Source(domain.SessionID(id), false)
				sources[source.SessionID] = source
				mustCreateSession(t, b, source.SessionID)
				a := conformance.Issue58Admission(source, id)
				if _, err := b.CommitRunAdmission(ctx, a); err != nil {
					t.Fatal(err)
				}
				m := newEventMapper(a.Run.ID, 64<<10)
				events := []domain.RunEvent{m.build(domain.EventModelDelta, payloadModelDelta{Delta: "exact"})}
				completion := m.build(domain.EventModelCompleted, v2Completion("exact"))
				completion.PayloadVersion = 2
				if id == "z-bad-pending" && mode == "projection" {
					completion.Payload = []byte(`{"content_length":5,"content_sha256":"wrong"}`)
				}
				events = append(events, completion)
				if id != "z-bad-pending" || mode != "missing-terminal" {
					events = append(events, m.build(domain.EventRunCompleted, payloadRunCompleted{}))
				}
				if _, err := b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: events}); err != nil {
					t.Fatal(err)
				}

				if id != "z-bad-pending" || mode != "missing-status" {
					if err := b.SetRunStatus(ctx, a.Run.ID, domain.RunCompleted); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := b.ArchiveRetainedSession(ctx, "z-bad-pending"); err != nil {
				t.Fatal(err)
			}
			if mode == "revoked" {
				delete(sources, "z-bad-pending")
			}
			got, err := captureIssue58(t, svc).CaptureSubjectContext(ctx, "main-reader")
			if err == nil || got.Through != 0 {
				t.Fatalf("incomplete domain accepted %+v %v", got, err)
			}
			high, err := b.CaptureSubjectHistory(ctx, conformance.Issue58Source("a-good-pending", false).SubjectHistoryBinding)
			if err != nil || high != 0 {
				t.Fatalf("sealed before all projections repaired: %d %v", high, err)
			}
		})
	}
}

func TestIssue58SubjectDomainPendingRepairActiveAndNoContextExcluded(t *testing.T) {
	f := newShellFixture(t, domain.ApprovalPolicyAuto)
	ctx := context.Background()
	conformance.Issue58Identity(t, f.backend)
	source := conformance.Issue58Source(f.sessionID, false)
	sources := map[domain.SessionID]storage.SubjectSourceBinding{source.SessionID: source}
	issue58Sources(f.service, sources)
	rid, err := f.service.RunShell(ctx, f.sessionID, "printf 'excluded'", ShellRunOptions{NoContext: true})
	if err != nil {
		t.Fatal(err)
	}
	if !f.service.WaitIdle(ctx) {
		t.Fatal("shell did not settle")
	}
	if _, err := f.backend.GetRunSubjectSource(ctx, rid); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("NoContext became pinned %v", err)
	}
	active := conformance.Issue58Admission(source, "context-pending-active")
	if _, err := f.backend.CommitRunAdmission(ctx, active); err != nil {
		t.Fatal(err)
	}
	pending, err := f.backend.ListSubjectPendingRuns(ctx, source.SubjectHistoryBinding)
	if err != nil || len(pending) != 0 {
		t.Fatalf("active/NoContext entered pending set %+v %v", pending, err)
	}
	snapshot, err := captureIssue58(t, f.service).CaptureSubjectContext(ctx, "main-reader")
	if err != nil || snapshot.Through != 0 || len(snapshot.Dispositions) != 0 {
		t.Fatalf("excluded runs consumed ceiling %+v %v", snapshot, err)
	}
}

func TestIssue58SubjectContextNestedCopyAuthorization(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.RunIssue58SubjectContextCopyScope(t, b)
	sources := map[domain.SessionID]storage.SubjectSourceBinding{}
	for _, id := range []domain.SessionID{"context-copy-parent", "context-copy-child", "context-copy-nested"} {
		sources[id] = conformance.Issue58Source(id, false)
	}
	issue58Sources(svc, sources)
	snapshot, err := captureIssue58(t, svc).CaptureSubjectContext(ctx, "main-reader")
	if err != nil || len(snapshot.Dispositions) != 2 {
		t.Fatalf("nested authorized capture %+v %v", snapshot, err)
	}
	delete(sources, "context-copy-child")
	denied, err := captureIssue58(t, svc).CaptureSubjectContext(ctx, "main-reader")
	if !errors.Is(err, storage.ErrSubjectBinding) || denied.Through != 0 || len(denied.Sources) != 0 {
		t.Fatalf("intermediate copy grant omitted %+v %v", denied, err)
	}
}

type issue58PendingBarrier struct {
	*sqlite.Backend
	afterPending func()
}

func (b *issue58PendingBarrier) ListSubjectPendingRuns(ctx context.Context, binding storage.SubjectHistoryBinding) ([]storage.SubjectPendingRun, error) {
	rows, err := b.Backend.ListSubjectPendingRuns(ctx, binding)
	if err == nil && b.afterPending != nil {
		f := b.afterPending
		b.afterPending = nil
		f()
	}
	return rows, err
}
func TestIssue58SubjectDomainPendingRepairConcurrentTerminal(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("context-between-phases", false)
	mustCreateSession(t, b, source.SessionID)
	sources := map[domain.SessionID]storage.SubjectSourceBinding{source.SessionID: source}
	issue58Sources(svc, sources)
	barrier := &issue58PendingBarrier{Backend: b, afterPending: func() { conformance.Issue58Completed(t, b, source, "context-new-terminal") }}
	svc.deps.Sessions = barrier
	snapshot, err := captureIssue58(t, svc).CaptureSubjectContext(ctx, "main-reader")
	if !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) || snapshot.Through != 0 || len(snapshot.Sources) != 0 {
		t.Fatalf("new terminal between pending set and capture was silently omitted %+v %v", snapshot, err)
	}
	// A fresh attempt repairs the newly discovered finite pending set.
	again, err := captureIssue58(t, svc).CaptureSubjectContext(ctx, "main-reader")
	if err != nil || again.Through != 1 {
		t.Fatalf("retry did not recover terminal %+v %v", again, err)
	}
}
