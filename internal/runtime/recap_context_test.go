package runtime

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"github.com/ProjectViVy/laputa/laputa/actmem"
)

func TestIssue58SharedRecentActualRequest(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	for _, sid := range []string{"source-A", "source-B"} {
		source := conformance.Issue58Source(domain.SessionID(sid), false)
		f.sources[source.SessionID] = source
		mustCreateSession(t, f.b, source.SessionID)
		admission := conformance.Issue58Admission(source, sid+"-equal-original")
		admission.Message.Content = "equal original text"
		if _, err := f.b.CommitRunAdmission(ctx, admission); err != nil {
			t.Fatal(err)
		}
		seq, err := f.b.Append(ctx, storage.Commit{RunID: admission.Run.ID, Events: []domain.RunEvent{{RunID: admission.Run.ID, Type: domain.EventRunCompleted, PayloadVersion: 1, Payload: []byte(`{}`)}}})
		if err != nil {
			t.Fatal(err)
		}
		if err = f.b.SetRunStatus(ctx, admission.Run.ID, domain.RunCompleted); err != nil {
			t.Fatal(err)
		}
		digest, err := storage.SubjectEvidenceDigest([]domain.Message{admission.Message})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.b.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, storage.SubjectInteraction{Sequence: 0, ID: storage.SubjectInteractionID(admission.Run.ID, seq), SessionID: source.SessionID, RunID: admission.Run.ID, TerminalSeq: seq, SourceDigest: digest}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{OperationID: "explicit-recap", Cause: "edit", Text: "shared resident recap"}); err != nil {
		t.Fatal(err)
	}
	first := f.run(t, "current once")
	if first.Status != domain.RunCompleted {
		t.Fatalf("first %+v", first)
	}
	f.restart(t)
	second := f.run(t, "after restart current")
	if second.Status != domain.RunCompleted {
		t.Fatalf("second %+v", second)
	}
	requests := f.capture()
	if len(requests) != 2 {
		t.Fatalf("requests=%d", len(requests))
	}
	for _, wire := range requests {
		if strings.Count(wire, "equal original text") != 2 || strings.Count(wire, "current once") != 1 || strings.Count(wire, "shared resident recap") != 1 {
			t.Fatalf("canonical identity dedup/restart lost exact context: %s", wire)
		}
	}
	if strings.Count(requests[1], "after restart current") != 1 {
		t.Fatal("current ID duplicated on restart")
	}
}

// This fixture binds the real C1 owner to the same subject identity. It has no
// inference or alternate evidence store; production receives its bundle port.
type issue58BoundOwner struct {
	store   *actmem.Store
	binding cognitivecontract.RecapBinding
	root    string
}

func issue58Owner(t *testing.T) *issue58BoundOwner {
	t.Helper()
	root := t.TempDir()
	store, err := actmem.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	source := conformance.Issue58Source("main-reader", true)
	return &issue58BoundOwner{root: root, store: store, binding: cognitivecontract.RecapBinding{SubjectID: source.SubjectID, DomainID: source.DomainID}}
}
func (o *issue58BoundOwner) ReadRecap(ctx context.Context) (cognitivecontract.RecapHead, error) {
	return o.store.ReadRecap(ctx, o.binding)
}
func (o *issue58BoundOwner) CommitRecap(ctx context.Context, c cognitivecontract.RecapCommit) (cognitivecontract.RecapHead, error) {
	c.Binding = o.binding
	return o.store.CommitRecap(ctx, c)
}
func (o *issue58BoundOwner) LookupRecapOperation(ctx context.Context, id string) (cognitivecontract.RecapHead, bool, error) {
	return o.store.LookupRecapOperation(ctx, o.binding, id)
}
func (o *issue58BoundOwner) ListRecapHistory(ctx context.Context, before uint64, limit int) ([]cognitivecontract.RecapHead, error) {
	return o.store.ListRecapHistory(ctx, o.binding, before, limit)
}
func (o *issue58BoundOwner) ReadRecapRevision(ctx context.Context, rev uint64) (cognitivecontract.RecapHead, error) {
	return o.store.ReadRecapRevision(ctx, o.binding, rev)
}

// A finished run with an unresolved tool operation must not stop the subject's
// next model request: it is excluded, the healthy run's originals still ship.
func TestIssue58ExcludedRunDoesNotBlockRecapContext(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	stuckSource := conformance.Issue58Source("source-stuck", false)
	healthySource := conformance.Issue58Source("source-healthy", false)
	for _, source := range []storage.SubjectSourceBinding{stuckSource, healthySource} {
		f.sources[source.SessionID] = source
		mustCreateSession(t, f.b, source.SessionID)
	}
	conformance.Issue58Stuck(t, f.b, stuckSource, "stuck-run")
	healthy := conformance.Issue58Completed(t, f.b, healthySource, "healthy-run")
	if _, err := f.b.SealSubjectInteraction(ctx, healthySource.SubjectHistoryBinding, healthy); err != nil {
		t.Fatal(err)
	}
	run := f.run(t, "current after stuck")
	if run.Status != domain.RunCompleted {
		t.Fatalf("stuck run blocked the next request: %+v", run)
	}
	requests := f.capture()
	if len(requests) != 1 || !strings.Contains(requests[0], "exact source healthy-run") || strings.Contains(requests[0], "exact source stuck-run") {
		t.Fatalf("wrong context for excluded run: %v", requests)
	}
	rows, err := f.b.ListSubjectInteractions(ctx, healthySource.SubjectHistoryBinding, 0, 10, 10)
	if err != nil || len(rows) < 2 {
		t.Fatalf("interactions %+v %v", rows, err)
	}
	excluded := 0
	for _, row := range rows {
		if row.Disposition == storage.SubjectInteractionExcluded {
			excluded++
		}
	}
	if excluded != 1 {
		t.Fatalf("excluded interactions = %d, want 1", excluded)
	}
}
