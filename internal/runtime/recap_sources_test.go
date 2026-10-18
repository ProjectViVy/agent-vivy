package runtime

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/storage"
	"github.com/cloudwego/eino/schema"
)

func TestIssue58RecapSourcesExactHistorical(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	first := strings.Repeat("exact-first-", 1000)
	second := strings.Repeat("exact-second-", 1000)
	a := issue58CompletedText(t, f, "source", "source-a", first)
	b := issue58CompletedText(t, f, "source", "source-b", second)
	issue58CompletedText(t, f, "source", "source-c", "latest-uncovered")
	f.reply = func(int) *schema.Message { return schema.AssistantMessage("summary, never an original", nil) }
	if _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	head, err := f.owner.ReadRecap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if head.CoveredThrough != 2 {
		t.Fatalf("fixture coverage %d", head.CoveredThrough)
	}
	if _, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: head.Revision, OperationID: "edit-source-test", Cause: "edit", Text: "new current text"}); err != nil {
		t.Fatal(err)
	}
	requests := len(f.capture())
	f.svc.engine = nil // Source expansion must not depend on model or index availability.
	sources, err := f.svc.ReadRecapSources(ctx, "main-reader", "", "", head.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 || sources[0].InteractionID != a.ID || sources[1].InteractionID != b.ID || len(sources[0].Messages) != 1 || sources[0].Messages[0].Content != first || sources[1].Messages[0].Content != second {
		t.Fatalf("historical originals not exact: %+v", sources)
	}
	if len(f.capture()) != requests {
		t.Fatal("source read invoked model")
	}
	if _, err := f.svc.ReadRecapSources(ctx, "main-reader", "", "", head.Revision+1); err == nil {
		t.Fatal("human edit borrowed an automatic source certificate")
	}
	restored, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: head.Revision + 1, OperationID: "restore-source-test", Cause: "restore", RestoredFrom: head.Revision})
	if err != nil {
		t.Fatal(err)
	}
	restoredSources, err := f.svc.ReadRecapSources(ctx, "main-reader", "", "", restored.Revision)
	if err != nil || len(restoredSources) != 2 || restoredSources[0].SourceRevision != head.Revision || restoredSources[0].Messages[0].Content != first {
		t.Fatalf("restore lost its exact historical source: %+v %v", restoredSources, err)
	}
	source := f.sources["source"]
	source.Grant.Version++
	f.sources["source"] = source
	if got, err := f.svc.ReadRecapSources(ctx, "main-reader", "", "", head.Revision); err == nil || got != nil {
		t.Fatalf("revoked source leaked: %v %v", got, err)
	}
}

func TestIssue58RecapSourcesHistoricalCorrectionSubset(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	a := f.completed(t, "old-source", "corrected-a")
	issue58SeedSummary(t, f, a, "")
	if err := f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: a.SessionID, RunID: a.RunID, Reason: storage.TruncationRewind, CutoffMessageID: "user_corrected-a", TailMessageID: "user_corrected-a", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	f.reply = func(int) *schema.Message { return schema.AssistantMessage("corrected historical claim", nil) }
	if _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	historical, err := f.owner.ReadRecap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b := f.completed(t, "later-source", "corrected-b")
	if err := f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: b.SessionID, RunID: b.RunID, Reason: storage.TruncationRewind, CutoffMessageID: "user_corrected-b", TailMessageID: "user_corrected-b", CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	requests := len(f.capture())
	sources, err := f.svc.ReadRecapSources(ctx, "main-reader", "", "", historical.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].InteractionID != a.ID || len(sources[0].Messages) != 1 || sources[0].Messages[0].ID != "user_corrected-a" {
		t.Fatalf("historical receipt expanded current correction set: %+v", sources)
	}
	if len(f.capture()) != requests {
		t.Fatal("historical verification invoked a model")
	}
	receipt, err := decodeRecapReceipt(historical.Sources.Digest)
	if err != nil {
		t.Fatal(err)
	}
	receipt.CorrectionEvidenceDigest = strings.Repeat("0", 64)
	encoded, err := encodeRecapReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	bad, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: historical.Revision, OperationID: "changed-correction", Cause: "reconcile", Text: "bad certificate", Sources: cognitivecontract.SourceRange{After: receipt.Range.After, Through: receipt.Range.Through, Digest: encoded}})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.svc.ReadRecapSources(ctx, "main-reader", "", "", bad.Revision); err == nil || got != nil {
		t.Fatalf("corrupted correction evidence expanded: %v %v", got, err)
	}
}

func TestIssue58RecapSourcesRejectChangedOriginalDigest(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	for i, id := range []string{"digest-a", "digest-b", "digest-c"} {
		text := "uncovered"
		if i < 2 {
			text = strings.Repeat(id, 1500)
		}
		issue58CompletedText(t, f, "digest-source", id, text)
	}
	f.reply = func(int) *schema.Message { return schema.AssistantMessage("exact folded prefix", nil) }
	if _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	head, err := f.owner.ReadRecap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := decodeRecapReceipt(head.Sources.Digest)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Range.After = head.CoveredThrough
	receipt.Range.Through = head.CoveredThrough + 1
	receipt.Range.ExactOriginalsDigest = strings.Repeat("0", 64)
	encoded, err := encodeRecapReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	bad, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: head.Revision, OperationID: "changed-range", Cause: "summary", Text: "must not substitute", Sources: cognitivecontract.SourceRange{After: receipt.Range.After, Through: receipt.Range.Through, Digest: encoded}})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.svc.ReadRecapSources(ctx, "main-reader", "", "", bad.Revision); err == nil || got != nil {
		t.Fatalf("bad digest expanded: %v %v", got, err)
	}
}
