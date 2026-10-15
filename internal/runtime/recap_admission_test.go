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
}

func (t *issue58CallbackTool) Spec() domain.ToolSpec {
	name := t.name
	if name == "" {
		name = "issue58_callback"
	}
	return domain.ToolSpec{Name: name, Readonly: true, Exposure: domain.ToolExposureDirect, Schema: json.RawMessage(`{"type":"object","properties":{"number":{"type":"number"}},"additionalProperties":false}`)}
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
				return
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
