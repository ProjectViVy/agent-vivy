package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage"
	"github.com/cloudwego/eino/schema"
)

func TestIssue58CompactionWholeInteractionSegments(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	first := "segment-one-start" + strings.Repeat("A", 110000) + "segment-one-end"
	second := "segment-two-start" + strings.Repeat("B", 110000) + "segment-two-end"
	issue58CompletedText(t, f, "segment-source", "segment-a", first)
	issue58CompletedText(t, f, "segment-source", "segment-b", second)
	f.completed(t, "segment-tail", "segment-c")
	for i := 1; i <= 2; i++ {
		if _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
			t.Fatal(err)
		}
		head, err := f.owner.ReadRecap(ctx)
		if err != nil || head.CoveredThrough != uint64(i) {
			t.Fatalf("segment cursor %+v %v", head, err)
		}
	}
	requests := f.capture()
	if len(requests) != 2 || !strings.Contains(requests[0], first) || strings.Contains(requests[0], second) || !strings.Contains(requests[1], second) || strings.Contains(requests[1], first) {
		t.Fatal("over-200KiB prefix was truncated or repeated instead of split at whole interactions")
	}
	if _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(f.capture()) != 2 {
		t.Fatal("unchanged complete suffix retired")
	}
}

func TestIssue58PostCommitReassemblyFailure(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	first := issue58CompletedText(t, f, "postcommit-source", "postcommit-a", strings.Repeat("A", 10000))
	tail := f.completed(t, "postcommit-tail", "postcommit-b")
	fault := &issue58FaultRecap{RecapPort: f.owner, after: func(_ cognitivecontract.RecapCommit, head cognitivecontract.RecapHead, err error) (cognitivecontract.RecapHead, error) {
		f.svc.deps.Sessions = issue58FailSources{f.b, f.b}
		return head, err
	}}
	f.svc.deps.Recap = fault
	run := f.run(t, "must retain current after committed fold")
	if run.Status != domain.RunFailed || len(f.capture()) != 1 {
		t.Fatal("postcommit reassembly failure sent stale or partial foreground input")
	}
	head, err := f.owner.ReadRecap(ctx)
	if err != nil || head.CoveredThrough != first.Sequence || head.Revision != 1 {
		t.Fatalf("postcommit failure was misreported as rollback: %+v %v", head, err)
	}
	for _, row := range []string{first.ID, tail.ID} {
		originals, err := f.b.ReadSubjectInteraction(ctx, conformanceBinding(), row)
		if err != nil || len(originals) == 0 {
			t.Fatal("postcommit read failure lost retained originals")
		}
	}
}

func TestIssue58SummaryConcurrentCAS(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	row := issue58CompletedText(t, f, "concurrent-source", "concurrent-a", strings.Repeat("A", 10000))
	f.completed(t, "concurrent-tail", "concurrent-b")
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	f.reply = func(int) *schema.Message {
		entered <- struct{}{}
		<-release
		return schema.AssistantMessage("concurrent summary", nil)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); results <- err }()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(10 * time.Second):
			close(release)
			t.Fatal("inference serialized under owner CAS")
		}
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	head, err := f.owner.ReadRecap(ctx)
	if err != nil || head.Revision != 1 || head.CoveredThrough != row.Sequence || len(f.capture()) != 2 {
		t.Fatalf("concurrent summaries repeated durable range: %+v %v calls=%d", head, err, len(f.capture()))
	}
}

func TestIssue58CorrectionFailureCuts(t *testing.T) {
	for _, kind := range []string{"denied-before", "denied-during-generation", "missing-anchor", "malformed-envelope", "correction-capacity", "summary-failure"} {
		t.Run(kind, func(t *testing.T) {
			f := newIssue58Wire(t)
			ctx := context.Background()
			row := f.completed(t, "failure-source", "failure-a")
			override := ""
			if kind == "malformed-envelope" {
				override = "unsupported receipt"
			}
			seed := issue58SeedSummary(t, f, row, override)
			marker := storage.SessionTruncation{SessionID: row.SessionID, CutoffMessageID: "user_failure-a", TailMessageID: "user_failure-a", Reason: storage.TruncationRewind}
			if kind == "missing-anchor" {
				marker.CutoffMessageID = "missing-anchor"
				marker.TailMessageID = "missing-anchor"
			}
			if err := f.b.RecordSessionTruncation(ctx, marker); err != nil {
				t.Fatal(err)
			}
			if kind == "denied-before" {
				delete(f.sources, row.SessionID)
			}
			if kind == "denied-during-generation" {
				f.reply = func(int) *schema.Message {
					delete(f.sources, row.SessionID)
					return schema.AssistantMessage("never commit revoked source", nil)
				}
			}
			if kind == "summary-failure" {
				f.reply = func(int) *schema.Message { return nil }
			}
			if kind == "correction-capacity" {
				f.svc.SetCatalog(provider.NewCatalog(provider.Vendor{Name: "fixture", Endpoints: []provider.Endpoint{{Adapter: provider.AdapterOpenAICompletions, Models: []provider.Model{{ID: "fixture", ContextWindow: 4000}}}}}))
			}
			port := &issue58FaultRecap{RecapPort: f.owner}
			f.svc.deps.Recap = port
			if _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err == nil {
				t.Fatal("required correction failure hidden")
			}
			expected := 0
			if kind == "denied-during-generation" || kind == "summary-failure" {
				expected = 1
			}
			if len(f.capture()) != expected || len(port.attempts) != 0 {
				t.Fatalf("correction failure called fallback/commit: provider=%d owner=%d", len(f.capture()), len(port.attempts))
			}
			head, err := f.owner.ReadRecap(ctx)
			if err != nil || head != seed {
				t.Fatalf("failed correction changed authority %+v %v", head, err)
			}
			originals, err := f.b.ReadSubjectInteraction(ctx, conformanceBinding(), row.ID)
			if err != nil || len(originals) == 0 {
				t.Fatal("failed correction lost originals")
			}
		})
	}
}
