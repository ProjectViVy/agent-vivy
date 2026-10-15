package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/schema"
)

type issue58CallbackTool struct {
	run    func()
	result string
	name   string
	schema string
}

func (t *issue58CallbackTool) Spec() domain.ToolSpec {
	name := t.name
	if name == "" {
		name = "issue58_callback"
	}
	body := t.schema
	if body == "" {
		body = `{"type":"object","properties":{"number":{"type":"number"}},"additionalProperties":false}`
	}
	return domain.ToolSpec{Name: name, Readonly: true, Exposure: domain.ToolExposureDirect, Schema: json.RawMessage(body)}
}
func (t *issue58CallbackTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	if t.run != nil {
		t.run()
	}
	return t.result, nil
}
func issue58ToolReply(name, args string) *schema.Message {
	return schema.AssistantMessage("", []schema.ToolCall{{ID: "fixture-call", Type: "function", Function: schema.FunctionCall{Name: name, Arguments: args}}})
}
func issue58FailurePayload(t *testing.T, f *issue58Wire, run domain.Run) string {
	t.Helper()
	for _, e := range replayAll(t, f.b, run.ID) {
		if e.Type == domain.EventRunFailed {
			return string(e.Payload)
		}
	}
	return ""
}

func TestIssue58CapacityPauses(t *testing.T) {
	for _, kind := range []string{"oversized-current", "unknown-window", "unknown-image-charge", "large-full-schema"} {
		t.Run(kind, func(t *testing.T) {
			f := newIssue58Wire(t)
			if kind == "large-full-schema" {
				large := &issue58CallbackTool{schema: `{"type":"object","properties":{"payload":{"type":"string","description":"` + strings.Repeat("schema", 1000) + `"}}}`}
				f = newIssue58Wire(t, large)
			}
			text := "current retained"
			opts := RunOptions{}
			switch kind {
			case "oversized-current":
				f.svc.engine.cfg.MaxContextBytes = 1500
				text = strings.Repeat("中文", 1000)
			case "unknown-window":
				f.svc.SetCatalog(nil)
			case "unknown-image-charge":
				opts.Attachments = []domain.Attachment{{Name: "fixture", MimeType: "image/png", Data: []byte("fixture")}}
			case "large-full-schema":
				// The name/description are tiny; only the complete parameter schema causes
				// pressure. The old name-only/bytes4 estimate cannot detect this request.
				f.svc.engine.cfg.MaxContextBytes = 2400
				info := &schema.ToolInfo{Name: "large", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{"payload": {Type: schema.String, Desc: strings.Repeat("schema", 1000)}})}
				usage, err := f.svc.measureRecapRequest(context.Background(), []*schema.Message{schema.UserMessage("tiny")}, []*schema.ToolInfo{info})
				if !errors.Is(err, ErrContextCapacityRequired) || usage.TextualTokenUpperBound < 5000 {
					t.Fatalf("schema not budgeted: %+v %v", usage, err)
				}
			}
			id, err := f.svc.RunWithOptions(context.Background(), "main-reader", text, opts)
			if err != nil {
				t.Fatal(err)
			}
			if !f.svc.WaitIdle(context.Background()) {
				t.Fatal("not idle")
			}
			run, err := f.b.GetRun(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			if run.Status != domain.RunFailed || len(f.capture()) != 0 || !strings.Contains(issue58FailurePayload(t, f, run), "context_capacity_required") {
				t.Fatalf("capacity did not pause before HTTP %+v %s", run, issue58FailurePayload(t, f, run))
			}
			head, err := f.owner.ReadRecap(context.Background())
			if err != nil || head.Revision != 0 || head.CoveredThrough != 0 {
				t.Fatal("capacity retired evidence")
			}
		})
	}
}

type issue58FailMessages struct{ storage.MessageStore }

func (issue58FailMessages) ListMessages(context.Context, domain.SessionID) ([]domain.Message, error) {
	return nil, errors.New("fixture exact read failure")
}

type issue58FailSources struct {
	storage.SessionStore
	storage.SubjectSourceStore
}

func (issue58FailSources) ReadSubjectInteraction(context.Context, storage.SubjectHistoryBinding, string) ([]domain.Message, error) {
	return nil, errors.New("fixture exact expansion failure")
}
func TestIssue58NoSilentHistoryFallback(t *testing.T) {
	for _, kind := range []string{"current-read", "source-expansion"} {
		t.Run(kind, func(t *testing.T) {
			f := newIssue58Wire(t)
			f.completed(t, "source-A", "read-A")
			if kind == "current-read" {
				f.svc.deps.Messages = issue58FailMessages{f.b}
			} else {
				f.svc.deps.Sessions = issue58FailSources{f.b, f.b}
			}
			run := f.run(t, "must pause")
			if run.Status != domain.RunFailed || len(f.capture()) != 0 {
				t.Fatalf("history failure became provider request %+v", run)
			}
		})
	}
}

func TestIssue58CoveredSourceGrantBoundary(t *testing.T) {
	f := newIssue58Wire(t)
	row := f.completed(t, "archived-A", "grant-A")
	issue58SeedSummary(t, f, row, "")
	if err := f.b.ArchiveRetainedSession(context.Background(), row.SessionID); err != nil {
		t.Fatal(err)
	}
	source := f.sources[row.SessionID]
	delete(f.sources, row.SessionID)
	run := f.run(t, "deny changed source")
	if run.Status != domain.RunFailed || len(f.capture()) != 0 {
		t.Fatal("revoked archived source reached HTTP")
	}
	for _, mutate := range []func(*storage.SubjectSourceBinding){func(s *storage.SubjectSourceBinding) { s.Grant.Version++ }, func(s *storage.SubjectSourceBinding) { s.Grant.Snapshot = json.RawMessage(`{"decision":"changed"}`) }} {
		changed := source
		mutate(&changed)
		f.sources[row.SessionID] = changed
		run = f.run(t, "deny changed decision")
		if run.Status != domain.RunFailed || len(f.capture()) != 0 {
			t.Fatal("changed grant reached HTTP")
		}
	}
	f.sources[row.SessionID] = source
	run = f.run(t, "exact restoration")
	if run.Status != domain.RunCompleted || len(f.capture()) != 1 {
		t.Fatalf("restoration failed %+v", run)
	}
}

func TestIssue58ModelLegGrantAndClear(t *testing.T) {
	for _, kind := range []string{"source-revoked", "clear"} {
		t.Run(kind, func(t *testing.T) {
			tool := &issue58CallbackTool{result: "tool exact"}
			f := newIssue58Wire(t, tool)
			row := f.completed(t, "source-A", "leg-A")
			source := f.sources[row.SessionID]
			tool.run = func() {
				if kind == "source-revoked" {
					delete(f.sources, row.SessionID)
				} else {
					_, err := f.owner.CommitRecap(context.Background(), cognitivecontract.RecapCommit{OperationID: "clear-between-legs", Cause: "clear"})
					if err != nil {
						t.Error(err)
					}
				}
			}
			f.reply = func(i int) *schema.Message {
				if i == 0 {
					return issue58ToolReply(tool.Spec().Name, `{"number":1}`)
				}
				return schema.AssistantMessage("fresh complete", nil)
			}
			run := f.run(t, "first admitted cut")
			if run.Status != domain.RunFailed || len(f.capture()) != 1 {
				t.Fatalf("stale leg reached HTTP %+v requests=%d", run, len(f.capture()))
			}
			tool.run = nil
			f.sources[row.SessionID] = source
			run = f.run(t, "fresh admission after restoration")
			if run.Status != domain.RunCompleted || len(f.capture()) != 2 {
				t.Fatalf("fresh recovery failed %+v events=%+v", run, replayAll(t, f.b, run.ID))
			}
		})
	}
}

func TestIssue58NotebookAutomaticExclusionActualRequest(t *testing.T) {
	tool := &issue58CallbackTool{name: tools.ReadNoteName, result: "private-heading private-body"}
	f := newIssue58Wire(t, tool)
	f.reply = func(i int) *schema.Message {
		switch i {
		case 0:
			return issue58ToolReply(tool.Spec().Name, `{"number":1}`)
		case 1:
			return schema.AssistantMessage("private-paraphrase", nil)
		default:
			return schema.AssistantMessage("safe fresh", nil)
		}
	}
	first := f.run(t, "read notebook")
	if first.Status != domain.RunCompleted {
		t.Fatalf("first=%+v", first)
	}
	second := f.run(t, "fresh automatic context")
	requests := f.capture()
	if second.Status != domain.RunCompleted || len(requests) != 3 {
		t.Fatalf("second=%+v requests=%d events=%+v", second, len(requests), replayAll(t, f.b, second.ID))
	}
	if !strings.Contains(requests[1], "private-body") {
		t.Fatal("authorized same-run Notebook unavailable")
	}
	for _, bad := range []string{"private-heading", "private-body", "private-paraphrase", `\\\"number\\\":1`} {
		if strings.Contains(requests[2], bad) {
			t.Fatalf("automatic request leaked %q: %s", bad, requests[2])
		}
	}
	if !strings.Contains(requests[2], "retained source excluded") {
		t.Fatal("excluded rows lack metadata-only notices")
	}
	rows, _, err := f.b.ReadSubjectRunEvidence(context.Background(), conformanceBinding(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.Content == tool.result {
			found = true
		}
	}
	if !found {
		t.Fatal("excluded original disappeared")
	}
}
func conformanceBinding() storage.SubjectHistoryBinding {
	return storage.SubjectHistoryBinding{SubjectID: "11111111-1111-4111-8111-111111111111", DomainID: "subject-personal-v1"}
}

func issue58SeedSummary(t *testing.T, f *issue58Wire, row storage.SubjectInteraction, digestOverride string) cognitivecontract.RecapHead {
	t.Helper()
	ctx := context.Background()
	binding, err := f.svc.AuthorizeSubjectHistory(ctx, "main-reader")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.svc.CaptureSubjectContext(ctx, "main-reader")
	if err != nil {
		t.Fatal(err)
	}
	digest, err := encodeRecapReceipt(recapReceipt{Version: 1, ProjectionPolicy: 1, Binding: binding, Range: recapReceiptRange{Through: row.Sequence, ExactOriginalsDigest: row.SourceDigest}, Dispositions: recapReceiptDispositions{Digest: snapshot.DispositionDigest}, CorrectionRefs: []storage.SubjectDisposition{}, CorrectionEvidenceDigest: sha256Hex(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if digestOverride != "" {
		digest = digestOverride
	}
	head, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{OperationID: "seed-summary", Cause: "summary", Text: "source-certified fact", Sources: cognitivecontract.SourceRange{Through: row.Sequence, Digest: digest}})
	if err != nil {
		t.Fatal(err)
	}
	return head
}

func TestIssue58ProjectionPolicyCertificateActualRequest(t *testing.T) {
	for _, kind := range []string{"supported", "legacy", "unknown-policy", "missing-policy", "malformed", "manual-edit", "manual-restore", "clear"} {
		t.Run(kind, func(t *testing.T) {
			f := newIssue58Wire(t)
			row := f.completed(t, "source-A", "certificate-A")
			head := issue58SeedSummary(t, f, row, "")
			ctx := context.Background()
			allowed := true
			switch kind {
			case "legacy", "unknown-policy", "missing-policy", "malformed":
				digest := head.Sources.Digest
				switch kind {
				case "legacy":
					digest = "historical-fingerprint-only"
				case "unknown-policy":
					digest = strings.Replace(digest, `"projection_policy":1`, `"projection_policy":2`, 1)
				case "missing-policy":
					digest = strings.Replace(digest, `"projection_policy":1,`, "", 1)
				case "malformed":
					digest = strings.Replace(digest, `"projection_policy":1`, `"projection_policy":null`, 1)
				}
				_, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: head.Revision, OperationID: "unsupported-reconcile", Cause: "reconcile", Text: head.Text, Sources: cognitivecontract.SourceRange{After: row.Sequence, Through: row.Sequence, Digest: digest}})
				if err != nil {
					t.Fatal(err)
				}
				allowed = false
			case "manual-edit":
				_, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: head.Revision, OperationID: "manual", Cause: "edit", Text: "explicit human text"})
				if err != nil {
					t.Fatal(err)
				}
			case "manual-restore":
				cleared, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: head.Revision, OperationID: "clear", Cause: "clear"})
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: cleared.Revision, OperationID: "restore", Cause: "restore", RestoredFrom: head.Revision})
				if err != nil {
					t.Fatal(err)
				}
			case "clear":
				_, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: head.Revision, OperationID: "clear", Cause: "clear"})
				if err != nil {
					t.Fatal(err)
				}
			}
			before, err := f.owner.ReadRecap(ctx)
			if err != nil {
				t.Fatal(err)
			}
			run := f.run(t, "certificate current")
			after, err := f.owner.ReadRecap(ctx)
			if err != nil || after != before {
				t.Fatal("admission rewrote/recertified head")
			}
			requests := f.capture()
			if allowed {
				if run.Status != domain.RunCompleted || len(requests) != 1 {
					t.Fatalf("valid/manual/clear admission failed %+v", run)
				}
				if strings.Contains(requests[0], "exact source certificate-A") {
					t.Fatal("covered original replayed")
				}
			} else {
				if run.Status != domain.RunFailed || len(requests) != 0 || !strings.Contains(issue58FailurePayload(t, f, run), "recap_unavailable") {
					t.Fatalf("unsupported certificate reached HTTP %+v", run)
				}
			}
		})
	}
}

func TestIssue58TruthfulPreexecutionRefusalActualRequest(t *testing.T) {
	tool := &classifierStubTool{class: tools.InvocationDenied, findings: []string{"denied synthetic command"}}
	f := newIssue58Wire(t, tool)
	f.reply = func(i int) *schema.Message {
		if i == 0 {
			return issue58ToolReply(tool.Spec().Name, `{"command":"synthetic"}`)
		}
		return schema.AssistantMessage("refusal acknowledged", nil)
	}
	run := f.run(t, "attempt governed tool")
	if run.Status != domain.RunCompleted || tool.calls != 0 || len(f.capture()) != 2 {
		t.Fatalf("refusal misreported %+v calls=%d", run, tool.calls)
	}
	events := replayAll(t, f.b, run.ID)
	metadata := false
	for _, event := range events {
		if event.Type != domain.EventToolFinished {
			continue
		}
		var outcome struct{ Outcome, Effects string }
		if json.Unmarshal(event.Payload, &outcome) == nil && outcome.Outcome == toolFailureStatusRefused && outcome.Effects == toolEffectsNotExecuted {
			metadata = true
		}
	}
	if !metadata {
		t.Fatal("refusal missing explicit refused/not_executed metadata")
	}
	wire := f.capture()[1]
	if !strings.Contains(wire, "did not run: denied synthetic command") {
		t.Fatalf("actual provider lacks truthful refusal: %s", wire)
	}
	f.restart(t)
	rows, _, err := f.b.ReadSubjectRunEvidence(context.Background(), conformanceBinding(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.Role == domain.RoleTool && strings.Contains(row.Content, "denied synthetic command") {
			found = true
		}
	}
	if !found {
		t.Fatal("retained refusal changed to execution")
	}
	run = f.run(t, "fresh after refusal")
	if run.Status != domain.RunCompleted || len(f.capture()) != 3 {
		t.Fatalf("fresh admitted recovery failed %+v", run)
	}
}

func TestIssue58AdmittedModelCapacityIgnoresMutableLabels(t *testing.T) {
	tool := &issue58CallbackTool{result: "exact tool"}
	f := newIssue58Wire(t, tool)
	tool.run = func() { f.svc.SetModel("changed-after-admission", "unknown-model") }
	f.reply = func(i int) *schema.Message {
		if i == 0 {
			return issue58ToolReply(tool.Spec().Name, `{"number":1}`)
		}
		return schema.AssistantMessage("done", nil)
	}
	run := f.run(t, "pinned admitted model")
	if run.Status != domain.RunCompleted || len(f.capture()) != 2 {
		t.Fatalf("capacity used mutable labels instead of admitted route %+v", run)
	}
	next := f.run(t, "new unknown model")
	if next.Status != domain.RunFailed || len(f.capture()) != 2 {
		t.Fatal("unknown current route reused earlier capacity")
	}
}

type issue58RunPinFailure struct {
	storage.SessionStore
	storage.SubjectSourceStore
}

func (issue58RunPinFailure) GetRunSubjectSource(context.Context, domain.RunID) (storage.SubjectSourceBinding, error) {
	return storage.SubjectSourceBinding{}, errors.New("fixture run pin read unavailable")
}
func TestIssue58RunPinFailureCannotSelectLegacyCompaction(t *testing.T) {
	f := newIssue58Wire(t)
	f.svc.deps.Sessions = issue58RunPinFailure{f.b, f.b}
	run := f.run(t, strings.Repeat("large unsealed current ", 500))
	if run.Status != domain.RunFailed || len(f.capture()) != 0 {
		t.Fatal("run pin lookup failure reached provider")
	}
	for _, event := range replayAll(t, f.b, run.ID) {
		if event.Type == domain.EventContextCompacted {
			t.Fatal("run pin read failure silently selected legacy compaction")
		}
	}
}
