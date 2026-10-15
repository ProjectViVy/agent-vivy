package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"github.com/cloudwego/eino/schema"
)

func issue58CompletedText(t *testing.T, f *issue58Wire, sid, id, text string) storage.SubjectInteraction {
	t.Helper()
	ctx := context.Background()
	source := conformance.Issue58Source(domain.SessionID(sid), false)
	if _, ok := f.sources[source.SessionID]; !ok {
		mustCreateSession(t, f.b, source.SessionID)
	}
	f.sources[source.SessionID] = source
	a := conformance.Issue58Admission(source, id)
	a.Message.Content = text
	if _, err := f.b.CommitRunAdmission(ctx, a); err != nil {
		t.Fatal(err)
	}
	seq, err := f.b.Append(ctx, storage.Commit{RunID: a.Run.ID, Events: []domain.RunEvent{{RunID: a.Run.ID, Type: domain.EventRunCompleted, PayloadVersion: 1, Payload: []byte(`{}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.b.SetRunStatus(ctx, a.Run.ID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	digest, err := storage.SubjectEvidenceDigest([]domain.Message{a.Message})
	if err != nil {
		t.Fatal(err)
	}
	row, err := f.b.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, storage.SubjectInteraction{ID: storage.SubjectInteractionID(a.Run.ID, seq), SessionID: source.SessionID, RunID: a.Run.ID, TerminalSeq: seq, SourceDigest: digest})
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestIssue58ContinuousRangeCompaction(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	texts := []string{"first-" + strings.Repeat("A", 9000), "second-" + strings.Repeat("B", 9000), "latest-complete"}
	for i, text := range texts {
		issue58CompletedText(t, f, "fold-source", string(rune('a'+i)), text)
	}
	f.reply = func(int) *schema.Message { return schema.AssistantMessage("folded exact prefix", nil) }
	result, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{})
	if err != nil {
		t.Fatalf("continuous range unavailable: %v", err)
	}
	head, err := f.owner.ReadRecap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if head.CoveredThrough != 2 || head.Cause != "summary" || result.Skipped {
		t.Fatalf("head=%+v result=%+v", head, result)
	}
	requests := f.capture()
	if len(requests) != 1 || !strings.Contains(requests[0], texts[0]) || !strings.Contains(requests[0], texts[1]) || strings.Contains(requests[0], texts[2]) {
		t.Fatalf("not exact oldest complete prefix: %v", requests)
	}
	var wire map[string]any
	json.Unmarshal([]byte(requests[0]), &wire)
	if wire["max_tokens"] != float64(3072) {
		t.Fatalf("summary output bound missing: %s", requests[0])
	}
	for _, row := range []string{"a", "b", "c"} {
		stored, _, err := f.b.ReadSubjectRunEvidence(ctx, conformanceBinding(), domain.RunID(row))
		if err != nil || len(stored) != 1 {
			t.Fatalf("original lost: %s %v", row, err)
		}
	}
	if _, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(f.capture()) != 1 {
		t.Fatal("unchanged explicit compact repeated model work")
	}
	issue58CompletedText(t, f, "fold-source", "d", strings.Repeat("D", 10000))
	if _, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	after, _ := f.owner.ReadRecap(ctx)
	if after.CoveredThrough != 3 || after.Sources.After != 2 {
		t.Fatalf("fold repeated covered range: %+v", after)
	}
	if strings.Contains(f.capture()[1], texts[0]) || !strings.Contains(f.capture()[1], texts[2]) {
		t.Fatal("successive fold duplicated or lost experience")
	}
}

func TestIssue58ZeroNewReconcile(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	row := f.completed(t, "archived-A", "zero-A")
	head := issue58SeedSummary(t, f, row, "")
	marker := storage.SessionTruncation{SessionID: row.SessionID, RunID: row.RunID, Reason: storage.TruncationRewind, CutoffMessageID: "user_zero-A", TailMessageID: "user_zero-A", CreatedAt: 1}
	if err := f.b.RecordSessionTruncation(ctx, marker); err != nil {
		t.Fatal(err)
	}
	if err := f.b.ArchiveRetainedSession(ctx, row.SessionID); err != nil {
		t.Fatal(err)
	}
	f.reply = func(int) *schema.Message {
		return schema.AssistantMessage("Historical claim was superseded only in archived A.", nil)
	}
	if _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatalf("zero-range correction unavailable: %v", err)
	}
	next, err := f.owner.ReadRecap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next.Cause != "reconcile" || next.CoveredThrough != head.CoveredThrough || next.InvalidationEpoch != head.InvalidationEpoch || next.Sources.After != next.Sources.Through {
		t.Fatalf("reconcile changed coverage/epoch %+v", next)
	}
	receipt, err := decodeRecapReceipt(next.Sources.Digest)
	if err != nil || len(receipt.CorrectionRefs) != 1 || receipt.CorrectionRefs[0].Targets[0].InteractionID != row.ID {
		t.Fatalf("missing exact correction provenance %+v %v", receipt, err)
	}
	if _, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(f.capture()) != 1 {
		t.Fatal("unchanged reconciliation not a no-op")
	}
	if _, err = f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: next.Revision, OperationID: "clear-after-reconcile", Cause: "clear"}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(f.capture()) != 1 {
		t.Fatal("clear replayed covered evidence")
	}
}
