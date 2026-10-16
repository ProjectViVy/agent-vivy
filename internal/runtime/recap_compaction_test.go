package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ProjectViVy/laputa/laputa/actmem"
	"reflect"
	"strings"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
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
	prior, err := f.b.ListRunsBySession(ctx, source.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	a.Run.CreatedAt = int64(len(prior) + 1)
	a.Message.CreatedAt = a.Run.CreatedAt
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

func TestIssue58ReconcileCertificateReset(t *testing.T) {
	for _, cause := range []string{"edit", "restore", "clear"} {
		t.Run(cause, func(t *testing.T) {
			f := newIssue58Wire(t)
			ctx := context.Background()
			row := f.completed(t, "reset-source", "reset-a")
			seed := issue58SeedSummary(t, f, row, "")
			if err := f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: row.SessionID, CutoffMessageID: "user_reset-a", TailMessageID: "user_reset-a", Reason: storage.TruncationEdit}); err != nil {
				t.Fatal(err)
			}
			f.reply = func(int) *schema.Message {
				return schema.AssistantMessage("Historical occurrence withdrawn in reset-source.", nil)
			}
			if _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
				t.Fatal(err)
			}
			head, _ := f.owner.ReadRecap(ctx)
			request := cognitivecontract.RecapCommit{BaseRevision: head.Revision, OperationID: "human-" + cause, Cause: cause}
			if cause == "edit" {
				request.Text = "Human text resets certification"
			}
			if cause == "restore" {
				request.RestoredFrom = seed.Revision
			}
			mutated, err := f.owner.CommitRecap(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
				t.Fatal(err)
			}
			after, _ := f.owner.ReadRecap(ctx)
			if cause == "clear" {
				if after != mutated || len(f.capture()) != 1 {
					t.Fatal("clear replayed covered originals")
				}
			} else if after.Cause != "reconcile" || after.CoveredThrough != mutated.CoveredThrough || len(f.capture()) != 2 {
				t.Fatalf("%s did not reset certificate %+v", cause, after)
			}
		})
	}
}

func TestIssue58DispositionCaptureRace(t *testing.T) {
	for _, repeat := range []bool{false, true} {
		t.Run(map[bool]string{false: "one-fresh-retry", true: "second-change-pauses"}[repeat], func(t *testing.T) {
			f := newIssue58Wire(t)
			ctx := context.Background()
			row := f.completed(t, "race-source", "race-a")
			issue58SeedSummary(t, f, row, "")
			mark := func(at int64) {
				if err := f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: row.SessionID, CutoffMessageID: "user_race-a", TailMessageID: "user_race-a", Reason: storage.TruncationRewind, CreatedAt: at}); err != nil {
					t.Error(err)
				}
			}
			mark(1)
			f.reply = func(i int) *schema.Message {
				if i == 0 || repeat {
					mark(int64(i + 2))
				}
				return schema.AssistantMessage("corrected historical occurrence", nil)
			}
			_, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{})
			if repeat {
				if !errors.Is(err, errRecapCaptureChanged) {
					t.Fatalf("second mutation not paused: %v", err)
				}
				head, _ := f.owner.ReadRecap(ctx)
				if head.Revision != 1 {
					t.Fatal("stale candidate committed")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(f.capture()) != 2 {
				t.Fatalf("unbounded or missing retry %d", len(f.capture()))
			}
		})
	}
}

func TestIssue58SummaryBudgetBound(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	row := f.completed(t, "budget-source", "budget-a")
	issue58SeedSummary(t, f, row, "")
	if err := f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: row.SessionID, CutoffMessageID: "user_budget-a", TailMessageID: "user_budget-a", Reason: storage.TruncationRewind}); err != nil {
		t.Fatal(err)
	}
	f.svc.deps.Budget.MaxModelCalls = 1
	run := f.run(t, "foreground must share summary call budget")
	if run.Status != domain.RunFailed || len(f.capture()) != 1 {
		t.Fatalf("summary bypassed call budget %+v requests=%d", run, len(f.capture()))
	}
	var requests int
	for _, event := range replayAll(t, f.b, run.ID) {
		if event.Type == domain.EventModelRequest {
			var p struct {
				Source string `json:"source"`
			}
			json.Unmarshal(event.Payload, &p)
			if p.Source != "summary" {
				t.Fatalf("wrong charged route %s", p.Source)
			}
			requests++
		}
	}
	if requests != 1 {
		t.Fatalf("summary request not journaled %d", requests)
	}
}

func TestIssue58CompactionCrashCutsCallback(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	issue58CompletedText(t, f, "callback-source", "callback-a", strings.Repeat("A", 10000))
	f.completed(t, "callback-tail", "callback-b")
	callbackErr := errors.New("fixture post-Finalize Callback failure")
	called := false
	ctx = withGovernanceEventSink(ctx, func(context.Context, GovernanceEvent) error { called = true; return callbackErr })
	binding, err := f.svc.AuthorizeSubjectHistory(ctx, "main-reader")
	if err != nil {
		t.Fatal(err)
	}
	head, err := f.svc.compactSubjectRecap(ctx, binding, "")
	if err != nil || head.Revision != 1 || !called {
		t.Fatalf("Callback commit receipt lost: %+v %v called=%v", head, err, called)
	}
	// The bound C1 remains authoritative; no host restart is needed.
	resolved, found, err := f.owner.LookupRecapOperation(ctx, head.OperationID)
	if err != nil || !found || resolved != head {
		t.Fatal("post-Finalize failure was not resolved by exact operation")
	}
}

type issue58FaultRecap struct {
	cognitivecontract.RecapPort
	before   func(cognitivecontract.RecapCommit)
	after    func(cognitivecontract.RecapCommit, cognitivecontract.RecapHead, error) (cognitivecontract.RecapHead, error)
	unknown  bool
	attempts []cognitivecontract.RecapCommit
}

func (p *issue58FaultRecap) CommitRecap(ctx context.Context, r cognitivecontract.RecapCommit) (cognitivecontract.RecapHead, error) {
	if p.unknown {
		return cognitivecontract.RecapHead{}, &actmem.Error{Code: "recap_commit_unknown"}
	}
	p.attempts = append(p.attempts, r)
	if p.before != nil {
		p.before(r)
	}
	h, err := p.RecapPort.CommitRecap(ctx, r)
	if p.after != nil {
		return p.after(r, h, err)
	}
	return h, err
}
func (p *issue58FaultRecap) ReadRecap(ctx context.Context) (cognitivecontract.RecapHead, error) {
	if p.unknown {
		return cognitivecontract.RecapHead{}, &actmem.Error{Code: "recap_commit_unknown"}
	}
	return p.RecapPort.ReadRecap(ctx)
}
func (p *issue58FaultRecap) LookupRecapOperation(ctx context.Context, id string) (cognitivecontract.RecapHead, bool, error) {
	if p.unknown {
		return cognitivecontract.RecapHead{}, false, &actmem.Error{Code: "recap_commit_unknown"}
	}
	return p.RecapPort.LookupRecapOperation(ctx, id)
}

func TestIssue58SummaryCASRecompute(t *testing.T) {
	for _, cause := range []string{"edit", "clear", "restore", "second-conflict"} {
		t.Run(cause, func(t *testing.T) {
			f := newIssue58Wire(t)
			ctx := context.Background()
			row := issue58CompletedText(t, f, "cas-source", "cas-a", strings.Repeat("A", 10000))
			f.completed(t, "cas-tail", "cas-b")
			// A real initial human revision allows a restore race without seeding coverage.
			seed, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{OperationID: "human-base", Cause: "edit", Text: "human original"})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			fault := &issue58FaultRecap{RecapPort: f.owner}
			fault.before = func(r cognitivecontract.RecapCommit) {
				if calls > 0 && cause != "second-conflict" {
					return
				}
				calls++
				c := cause
				if c == "second-conflict" {
					c = "edit"
				}
				request := cognitivecontract.RecapCommit{BaseRevision: r.BaseRevision, OperationID: fmt.Sprintf("race-%d", calls), Cause: c, Text: "human wins"}
				if c == "clear" {
					request.Text = ""
				}
				if c == "restore" {
					request.Text = ""
					request.RestoredFrom = seed.Revision
				}
				if _, err := f.owner.CommitRecap(ctx, request); err != nil {
					t.Error(err)
				}
			}
			f.svc.deps.Recap = fault
			_, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{})
			if cause == "second-conflict" {
				if !errors.Is(err, errRecapCaptureChanged) {
					t.Fatalf("second CAS conflict not paused %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			head, _ := f.owner.ReadRecap(ctx)
			if cause == "second-conflict" {
				if head.Cause != "edit" || head.CoveredThrough != 0 {
					t.Fatal("second conflict overrode human")
				}
			} else if head.Cause != "summary" || head.Sources.After != 0 || head.CoveredThrough != row.Sequence {
				t.Fatalf("CAS did not regenerate from coverage %+v", head)
			}
			if len(f.capture()) != 2 {
				t.Fatalf("CAS regenerated %d times", len(f.capture()))
			}
		})
	}
}

func TestIssue58CompactionCrashCuts(t *testing.T) {
	for _, zero := range []bool{false, true} {
		for _, cut := range []string{"before-commit", "lost-response", "cancel-after-commit", "live-unknown"} {
			t.Run(fmt.Sprintf("zero=%v/%s", zero, cut), func(t *testing.T) {
				f := newIssue58Wire(t)
				ctx := context.Background()
				var row storage.SubjectInteraction
				if zero {
					row = f.completed(t, "crash-source", "crash-a")
					issue58SeedSummary(t, f, row, "")
					if err := f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: row.SessionID, CutoffMessageID: "user_crash-a", TailMessageID: "user_crash-a", Reason: storage.TruncationRewind}); err != nil {
						t.Fatal(err)
					}
				} else {
					row = issue58CompletedText(t, f, "crash-source", "crash-a", strings.Repeat("A", 10000))
					f.completed(t, "crash-tail", "crash-b")
				}
				fault := &issue58FaultRecap{RecapPort: f.owner}
				f.svc.deps.Recap = fault
				runCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				switch cut {
				case "before-commit":
					fault.before = func(cognitivecontract.RecapCommit) { cancel() }
				case "lost-response":
					fault.after = func(_ cognitivecontract.RecapCommit, _ cognitivecontract.RecapHead, err error) (cognitivecontract.RecapHead, error) {
						if err != nil {
							t.Error(err)
						}
						return cognitivecontract.RecapHead{}, errors.New("fixture lost positive response")
					}
				case "cancel-after-commit":
					fault.after = func(_ cognitivecontract.RecapCommit, _ cognitivecontract.RecapHead, err error) (cognitivecontract.RecapHead, error) {
						cancel()
						return cognitivecontract.RecapHead{}, context.Canceled
					}
				case "live-unknown":
					fault.after = func(_ cognitivecontract.RecapCommit, _ cognitivecontract.RecapHead, err error) (cognitivecontract.RecapHead, error) {
						fault.unknown = true
						return cognitivecontract.RecapHead{}, &actmem.Error{Code: "recap_commit_unknown"}
					}
				}
				_, err := f.svc.CompactSession(runCtx, "main-reader", CompactOptions{})
				if cut == "before-commit" || cut == "live-unknown" {
					if !errors.Is(err, ErrRecapCommitPending) {
						t.Fatalf("uncertain attempt did not pause %v", err)
					}
				} else if err != nil {
					t.Fatalf("positive lookup lost committed outcome %v", err)
				}
				keys, keyErr := f.b.Blobs().(storage.BlobPrefixLister).ListPrefix(ctx, recapPendingPrefix(conformanceBinding()))
				if keyErr != nil {
					t.Fatal(keyErr)
				}
				if cut == "live-unknown" || cut == "before-commit" {
					if len(keys) != 1 || len(fault.attempts) != 1 {
						t.Fatalf("pending request lost %v attempts=%d", keys, len(fault.attempts))
					}
					raw, ok, e := f.b.Blobs().Get(ctx, keys[0])
					var intent recapPending
					if e != nil || !ok || json.Unmarshal(raw, &intent) != nil || intent.Request != fault.attempts[0] || intent.Fold.Receipt != intent.Request.Sources.Digest || intent.Fold.InputDigest == "" {
						t.Fatal("exact durable request/input proof lost")
					}
					if zero && len(intent.Fold.CorrectionRefs) != 1 {
						t.Fatal("pending correction evidence lost")
					}
				}
				originals, e := f.b.ReadSubjectInteraction(ctx, conformanceBinding(), row.ID)
				if e != nil || len(originals) == 0 {
					t.Fatal("crash retired originals")
				}
				// Existing lifecycle closes the old backend and owner before either is renewed.
				f.reopen(t, false)
				if _, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
					t.Fatalf("renewed bound port could not resolve exact pending request %v", err)
				}
				head, e := f.owner.ReadRecap(ctx)
				if e != nil {
					t.Fatal(e)
				}
				if head.CoveredThrough != row.Sequence || head.Cause != map[bool]string{true: "reconcile", false: "summary"}[zero] {
					t.Fatalf("duplicate/lost fold %+v", head)
				}
				if len(f.capture()) != 1 {
					t.Fatal("restart regenerated committed or exact-retry summary")
				}
			})
		}
	}
}

func TestIssue58SummaryRouteCapacityAndFallback(t *testing.T) {
	for _, kind := range []string{"unknown-summary-window", "smaller-summary-window", "summary-fallback"} {
		t.Run(kind, func(t *testing.T) {
			f := newIssue58Wire(t)
			issue58CompletedText(t, f, "route-source", "route-a", strings.Repeat("A", 10000))
			f.completed(t, "route-tail", "route-b")
			ref, err := provider.NewCatalog().RefForSelection("fixture", provider.Endpoint{Adapter: provider.AdapterOpenAICompletions, BaseURL: f.url, DefaultModel: "summary-fixture"})
			if err != nil {
				t.Fatal(err)
			}
			summary, err := ref.Model(context.Background(), provider.ModelSpec{ID: "summary-fixture", APIKey: "fixture", BaseURL: f.url})
			if err != nil {
				t.Fatal(err)
			}
			f.svc.engine.cfg.SummaryModel = summary
			f.svc.engine.cfg.SummaryModelID = "summary-fixture"
			window := 128000
			if kind == "unknown-summary-window" {
				window = 0
			}
			if kind == "smaller-summary-window" {
				window = 4000
			}
			f.svc.SetCatalog(provider.NewCatalog(provider.Vendor{Name: "fixture", Endpoints: []provider.Endpoint{{Adapter: provider.AdapterOpenAICompletions, DefaultModel: "fixture", Models: []provider.Model{{ID: "fixture", ContextWindow: 128000}, {ID: "summary-fixture", ContextWindow: window}}}}}))
			if kind == "summary-fallback" {
				f.reply = func(i int) *schema.Message {
					if i == 0 {
						return nil
					}
					return schema.AssistantMessage("native fallback summary", nil)
				}
			}
			_, err = f.svc.CompactSession(context.Background(), "main-reader", CompactOptions{})
			if kind != "summary-fallback" {
				if !errors.Is(err, ErrContextCapacityRequired) || len(f.capture()) != 0 {
					t.Fatalf("summary used foreground window: %v requests=%d", err, len(f.capture()))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			requests := f.capture()
			if len(requests) != 2 || !strings.Contains(requests[0], `"model":"summary-fixture"`) || !strings.Contains(requests[1], `"model":"fixture"`) {
				t.Fatalf("wrong actual summary/fallback route: %+v", requests)
			}
			for _, wire := range requests {
				if !strings.Contains(wire, `"max_tokens":3072`) {
					t.Fatalf("summary output bound mismatch: %s", wire)
				}
			}
		})
	}
}

type issue58SummaryJournalFault struct {
	storage.Journal
	fail domain.EventType
}

func (j issue58SummaryJournalFault) Append(ctx context.Context, c storage.Commit) (domain.EventSeq, error) {
	for _, e := range c.Events {
		if strings.HasPrefix(string(c.RunID), "sessctl_recap_") && e.Type == j.fail {
			return 0, errors.New("fixture synthetic summary append failed")
		}
	}
	return j.Journal.Append(ctx, c)
}

func TestIssue58ExplicitCompactionControlLane(t *testing.T) {
	for _, kind := range []string{"success", "request-failure", "finish-failure", "missing-port"} {
		t.Run(kind, func(t *testing.T) {
			f := newIssue58Wire(t)
			ctx := context.Background()
			row := issue58CompletedText(t, f, "control-source", "control-a", strings.Repeat("A", 10000))
			f.completed(t, "control-tail", "control-b")
			before := replayAll(t, f.b, row.RunID)
			messagesBefore, err := f.b.ListMessages(ctx, "main-reader")
			if err != nil {
				t.Fatal(err)
			}
			throughBefore, err := f.b.CaptureSubjectHistory(ctx, conformanceBinding())
			if err != nil {
				t.Fatal(err)
			}
			if kind == "request-failure" {
				f.svc.deps.Journal = issue58SummaryJournalFault{f.b, domain.EventModelRequest}
			}
			if kind == "finish-failure" {
				f.svc.deps.Journal = issue58SummaryJournalFault{f.b, domain.EventModelCallFinished}
			}
			if kind == "missing-port" {
				f.svc.deps.Recap = nil
			}
			_, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{})
			if kind == "success" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("control failure hidden")
			}
			expectedHTTP := 1
			if kind == "request-failure" || kind == "missing-port" {
				expectedHTTP = 0
			}
			if len(f.capture()) != expectedHTTP {
				t.Fatalf("control failure made provider work: %d", len(f.capture()))
			}
			if !reflect.DeepEqual(replayAll(t, f.b, row.RunID), before) {
				t.Fatal("explicit compact changed terminal source run")
			}
			runs, err := f.b.ListRunsBySession(ctx, "main-reader")
			if err != nil || len(runs) != 0 {
				t.Fatal("synthetic control fabricated real run")
			}
			messagesAfter, err := f.b.ListMessages(ctx, "main-reader")
			if err != nil || !reflect.DeepEqual(messagesBefore, messagesAfter) {
				t.Fatal("synthetic control fabricated user/tool message")
			}
			throughAfter, err := f.b.CaptureSubjectHistory(ctx, conformanceBinding())
			if err != nil || throughAfter != throughBefore {
				t.Fatal("synthetic control fabricated sealed interaction")
			}
			var control domain.RunID
			for _, event := range f.svc.deps.Sink.(*testSink).snapshot() {
				if strings.HasPrefix(string(event.RunID), "sessctl_recap_") {
					control = event.RunID
					break
				}
			}
			if kind == "success" || kind == "finish-failure" {
				if control == "" || !strings.Contains(string(control), sha256Hex([]byte("main-reader"))[:16]) {
					t.Fatal("missing scoped control identity")
				}
				if _, err = f.b.GetRun(ctx, control); !errors.Is(err, storage.ErrNotFound) {
					t.Fatal("rowless control gained RunStore row")
				}
				events := replayAll(t, f.b, control)
				var requests, finishes int
				for _, event := range events {
					if _, terminal := event.Type.RunStatus(); terminal {
						t.Fatal("synthetic control acquired terminal")
					}
					if event.Type == domain.EventModelRequest || event.Type == domain.EventModelCallFinished || event.Type == domain.EventModelUsage {
						var payload map[string]json.RawMessage
						if err := json.Unmarshal(event.Payload, &payload); err != nil || payload["call_id"] == nil {
							t.Fatalf("synthetic model event lost object payload: %s", event.Payload)
						}
					}
					if event.Type == domain.EventModelRequest {
						requests++
						if event.PayloadVersion != 4 {
							t.Fatal("lost request provenance version")
						}
					}
					if event.Type == domain.EventModelCallFinished {
						finishes++
					}
				}
				if requests != 1 || (kind == "success" && finishes != 1) {
					t.Fatalf("control lifecycle requests=%d finishes=%d", requests, finishes)
				}
			}
		})
	}
}

func TestIssue58CommittedCorrectionProvenance(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	row := f.completed(t, "provenance-source", "provenance-a")
	issue58SeedSummary(t, f, row, "")
	source := f.sources[row.SessionID]
	later := conformance.Issue58Admission(source, "provenance-b")
	if _, err := f.b.CommitRunAdmission(ctx, later); err != nil {
		t.Fatal(err)
	}
	if err := f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: row.SessionID, CutoffMessageID: "user_provenance-a", TailMessageID: later.Message.ID, Reason: storage.TruncationRewind, CreatedAt: 7}); err != nil {
		t.Fatal(err)
	}
	fault := &issue58FaultRecap{RecapPort: f.owner, after: func(_ cognitivecontract.RecapCommit, _ cognitivecontract.RecapHead, err error) (cognitivecontract.RecapHead, error) {
		if err != nil {
			t.Error(err)
		}
		return cognitivecontract.RecapHead{}, errors.New("fixture lost response")
	}}
	f.svc.deps.Recap = fault
	if _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	committed, err := f.owner.ReadRecap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	original, err := decodeRecapReceipt(committed.Sources.Digest)
	if err != nil || len(original.CorrectionRefs) != 1 || len(original.CorrectionRefs[0].Targets) != 1 {
		t.Fatalf("wrong used target subset %+v %v", original, err)
	}
	keys, err := f.b.Blobs().(storage.BlobPrefixLister).ListPrefix(ctx, recapPendingPrefix(conformanceBinding()))
	if err != nil || len(keys) != 0 {
		t.Fatal("resolved pending intent not cleared")
	}
	seq, err := f.b.Append(ctx, storage.Commit{RunID: later.Run.ID, Events: []domain.RunEvent{{RunID: later.Run.ID, Type: domain.EventRunCompleted, PayloadVersion: 1, Payload: []byte(`{}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.b.SetRunStatus(ctx, later.Run.ID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
	digest, err := storage.SubjectEvidenceDigest([]domain.Message{later.Message})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.b.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, storage.SubjectInteraction{ID: storage.SubjectInteractionID(later.Run.ID, seq), SessionID: source.SessionID, RunID: later.Run.ID, TerminalSeq: seq, SourceDigest: digest}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.svc.CaptureSubjectContext(ctx, "main-reader")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Dispositions) != 1 || len(snapshot.Dispositions[0].Targets) != 2 {
		t.Fatal("late seal did not expand same marker")
	}
	historical, err := f.owner.ReadRecapRevision(ctx, committed.Revision)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := decodeRecapReceipt(historical.Sources.Digest)
	if err != nil || !reflect.DeepEqual(recovered.CorrectionRefs, original.CorrectionRefs) || recovered.CorrectionEvidenceDigest != original.CorrectionEvidenceDigest || recovered.Dispositions.Digest == snapshot.DispositionDigest {
		t.Fatal("historical provenance inferred larger live capture")
	}
	if err = validateRecapCorrectionReferences(recovered.CorrectionRefs, snapshot); err != nil {
		t.Fatal("actual-used subset cannot be verified against later snapshot")
	}
}

func TestIssue58AdmissionCapacityUsesSharedFold(t *testing.T) {
	f := newIssue58Wire(t)
	old := issue58CompletedText(t, f, "capacity-source", "capacity-a", strings.Repeat("A", 5000))
	f.completed(t, "capacity-tail", "capacity-b")
	f.svc.SetCatalog(provider.NewCatalog(provider.Vendor{Name: "fixture", Endpoints: []provider.Endpoint{{Adapter: provider.AdapterOpenAICompletions, Models: []provider.Model{{ID: "fixture", ContextWindow: 10000}}}}}))
	run := f.run(t, "capacity current")
	if run.Status != domain.RunCompleted || len(f.capture()) != 2 {
		t.Fatalf("hard capacity did not invoke shared native fold: %+v %s calls=%d", run, issue58FailurePayload(t, f, run), len(f.capture()))
	}
	head, err := f.owner.ReadRecap(context.Background())
	if err != nil || head.CoveredThrough != old.Sequence {
		t.Fatalf("capacity fold not committed %+v %v", head, err)
	}
}

func TestIssue58SummaryHardOutputBound(t *testing.T) {
	for _, kind := range []string{"hard-max"} {
		t.Run(kind, func(t *testing.T) {
			f := newIssue58Wire(t)
			issue58CompletedText(t, f, "output-source", "output-a", strings.Repeat("A", 10000))
			f.completed(t, "output-tail", "output-b")
			text := strings.Repeat("x", 3073)
			f.reply = func(int) *schema.Message { return schema.AssistantMessage(text, nil) }
			_, err := f.svc.CompactSession(context.Background(), "main-reader", CompactOptions{})
			if !errors.Is(err, ErrContextCapacityRequired) {
				t.Fatalf("out-of-bound summary accepted %v", err)
			}
			head, e := f.owner.ReadRecap(context.Background())
			if e != nil || head.Revision != 0 {
				t.Fatal("invalid summary retired exact originals")
			}
		})
	}
}

func TestIssue58PendingIntentIntegrity(t *testing.T) {
	for _, kind := range []string{"text", "oversized-text", "base", "binding", "cause", "captured-source", "unknown-member", "partial"} {
		t.Run(kind, func(t *testing.T) {
			f := newIssue58Wire(t)
			ctx := context.Background()
			issue58CompletedText(t, f, "integrity-source", "integrity-a", strings.Repeat("A", 10000))
			f.completed(t, "integrity-tail", "integrity-b")
			cancelled, cancel := context.WithCancel(ctx)
			port := &issue58FaultRecap{RecapPort: f.owner, before: func(cognitivecontract.RecapCommit) { cancel() }}
			f.svc.deps.Recap = port
			if _, err := f.svc.CompactSession(cancelled, "main-reader", CompactOptions{}); !errors.Is(err, ErrRecapCommitPending) {
				t.Fatalf("fixture did not retain pending %v", err)
			}
			keys, err := f.b.Blobs().(storage.BlobPrefixLister).ListPrefix(ctx, recapPendingPrefix(conformanceBinding()))
			if err != nil || len(keys) != 1 {
				t.Fatal("missing fixture pending")
			}
			raw, _, err := f.b.Blobs().Get(ctx, keys[0])
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if json.Unmarshal(raw, &value) != nil {
				t.Fatal("invalid fixture pending")
			}
			request := value["Request"].(map[string]any)
			fold := value["Fold"].(map[string]any)
			switch kind {
			case "text":
				request["Text"] = "tampered valid text"
			case "oversized-text":
				request["Text"] = strings.Repeat("X", 4000)
			case "base":
				request["BaseRevision"] = float64(42)
			case "binding":
				request["Binding"] = map[string]any{"SubjectID": "other", "DomainID": "other"}
			case "cause":
				request["Cause"] = "edit"
			case "captured-source":
				fold["Reader"] = "other-reader"
			case "unknown-member":
				value["unknown"] = true
			case "partial":
				delete(fold, "InputDigest")
			}
			corrupted, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err = f.b.Blobs().Put(ctx, keys[0], corrupted); err != nil {
				t.Fatal(err)
			}
			f.reopen(t, false)
			renewed := &issue58FaultRecap{RecapPort: f.owner}
			f.svc.deps.Recap = renewed
			if _, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err == nil {
				t.Fatal("corrupted pending accepted")
			}
			if len(renewed.attempts) != 0 || len(f.capture()) != 1 {
				t.Fatal("corrupted request reached owner commit/provider")
			}
			head, e := f.owner.ReadRecap(ctx)
			if e != nil || head.Revision != 0 {
				t.Fatalf("corrupted request changed authority %+v %v", head, e)
			}
		})
	}
}

func TestIssue58ReconcileCopiedViewScope(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	row := f.completed(t, "copy-parent", "copy-a")
	originals, err := f.b.ListMessages(ctx, row.SessionID)
	if err != nil || len(originals) != 1 {
		t.Fatal("missing copy original")
	}
	copyRow := originals[0]
	copyRow.CopySource = &domain.MessageCopySource{SessionID: row.SessionID, MessageID: copyRow.ID}
	copyRow.ID = "view-" + copyRow.ID
	copyRow.SessionID = "copy-child"
	if _, err = f.b.CommitSessionFork(ctx, domain.Session{ID: copyRow.SessionID}, []domain.Message{copyRow}, []storage.SessionTruncation{{SessionID: row.SessionID, Reason: storage.TruncationFork, ForkSessionID: string(copyRow.SessionID), CutoffMessageID: originals[0].ID, TailMessageID: originals[0].ID}, {SessionID: copyRow.SessionID, Reason: storage.TruncationForkedFrom, ForkSessionID: string(row.SessionID), CutoffMessageID: copyRow.ID, TailMessageID: copyRow.ID}}, nil); err != nil {
		t.Fatal(err)
	}
	child := conformance.Issue58Source(copyRow.SessionID, false)
	f.sources[child.SessionID] = child
	own := conformance.Issue58Completed(t, f.b, child, "copy-own")
	if _, err = f.b.SealSubjectInteraction(ctx, child.SubjectHistoryBinding, own); err != nil {
		t.Fatal(err)
	}
	issue58SeedSummary(t, f, row, "")
	if err = f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: copyRow.SessionID, CutoffMessageID: copyRow.ID, TailMessageID: copyRow.ID, Reason: storage.TruncationRewind}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	head, err := f.owner.ReadRecap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := decodeRecapReceipt(head.Sources.Digest)
	if err != nil || len(receipt.CorrectionRefs) != 1 {
		t.Fatal("missing copied-view reference")
	}
	d := receipt.CorrectionRefs[0]
	target := d.Targets[0]
	if d.SessionID != copyRow.SessionID || target.SourceSessionID != row.SessionID || !reflect.DeepEqual(target.MessageIDs, []string{originals[0].ID}) || !reflect.DeepEqual(target.ViewMessageIDs, []string{copyRow.ID}) {
		t.Fatalf("copy mapping lost %+v", d)
	}
	after, err := f.b.ReadSubjectInteraction(ctx, child.SubjectHistoryBinding, row.ID)
	digest, _ := storage.SubjectEvidenceDigest(after)
	if err != nil || digest != row.SourceDigest {
		t.Fatal("copied-view correction mutated parent source")
	}
	wire := f.capture()[0]
	if !strings.Contains(wire, string(copyRow.SessionID)) || !strings.Contains(wire, copyRow.ID) || !strings.Contains(wire, "does not withdraw a valid parent-view claim") {
		t.Fatal("model did not receive exact view-scoped correction")
	}
}

func TestIssue58ClearInvalidatesResume(t *testing.T) {
	tool := &issue58CallbackTool{result: "clear completed"}
	f := newIssue58Wire(t, tool)
	ctx := context.Background()
	row := f.completed(t, "clear-source", "clear-a")
	head := issue58SeedSummary(t, f, row, "")
	tool.run = func() {
		if _, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: head.Revision, OperationID: "clear-active-provider", Cause: "clear"}); err != nil {
			t.Error(err)
		}
	}
	f.reply = func(i int) *schema.Message {
		if i == 0 {
			return issue58ToolReply(tool.Spec().Name, `{"number":1}`)
		}
		return schema.AssistantMessage("new admission completed", nil)
	}
	run := f.run(t, "active bytes already sent")
	if run.Status != domain.RunFailed || len(f.capture()) != 1 || !strings.Contains(f.capture()[0], "source-certified fact") {
		t.Fatal("clear did not preserve first request and stop next leg")
	}
	next := f.run(t, "after clear new admission")
	requests := f.capture()
	if next.Status != domain.RunCompleted || len(requests) != 2 {
		t.Fatalf("new clear admission failed %+v", next)
	}
	if strings.Contains(requests[1], "source-certified fact") || strings.Contains(requests[1], "exact source clear-a") {
		t.Fatal("new admission replayed cleared covered sources")
	}
}
