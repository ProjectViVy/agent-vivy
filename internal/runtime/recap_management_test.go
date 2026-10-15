package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"github.com/cloudwego/eino/schema"
)

type issue58GovernedTool struct {
	service **Service
	calls   int
}

func (t *issue58GovernedTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "actmem_read", Readonly: true, Exposure: domain.ToolExposureDeferred, Description: "Read the full resident Recap", Schema: json.RawMessage(`{"type":"object","additionalProperties":false}`)}
}
func (t *issue58GovernedTool) AuthorizeTool(ctx context.Context) error {
	if t.service == nil || *t.service == nil {
		return errors.New("unarmed")
	}
	_, err := (*t.service).AuthorizeRecap(ctx, contextSessionID(ctx), contextRunID(ctx), "actmem_read")
	return err
}
func (t *issue58GovernedTool) InvokableRun(ctx context.Context, _ json.RawMessage) (string, error) {
	if err := t.AuthorizeTool(ctx); err != nil {
		return "", err
	}
	t.calls++
	return "actual recap", nil
}

func issue58RecapToolReply(name, args string) *schema.Message {
	message := issue58ToolReply(name, args)
	message.ToolCalls[0].ID = "recap-" + name
	return message
}

func TestIssue58RecapDeferredTools(t *testing.T) {
	var svc *Service
	tool := &issue58GovernedTool{service: &svc}
	f := newIssue58Wire(t, tool)
	svc = f.svc
	f.reply = func(i int) *schema.Message {
		switch i {
		case 0:
			return issue58RecapToolReply("tool_search", `{"query":"select:actmem_read"}`)
		case 1:
			return issue58RecapToolReply("actmem_read", `{}`)
		default:
			return schema.AssistantMessage("done", nil)
		}
	}
	run := f.run(t, "discover and read")
	if run.Status != domain.RunCompleted || tool.calls != 1 {
		t.Fatalf("run=%+v calls=%d events=%+v", run, tool.calls, replayAll(t, f.b, run.ID))
	}
	requests := f.capture()
	if len(requests) != 3 {
		t.Fatalf("requests=%d", len(requests))
	}
	if strings.Contains(requests[0], `"name":"actmem_read"`) {
		t.Fatal("full deferred schema leaked initially")
	}
	if !strings.Contains(requests[1], `"name":"actmem_read"`) {
		t.Fatal("selected schema not activated")
	}
	if _, err := svc.AuthorizeRecap(context.Background(), "main-reader", run.ID, "actmem_read"); err == nil {
		t.Fatal("closed run authorized")
	}
	for _, sid := range []domain.SessionID{"ordinary", "private", "worker", "unknown"} {
		if _, err := svc.AuthorizeRecap(context.Background(), sid, "", " "); err == nil {
			t.Fatalf("%s authorized", sid)
		}
	}
	if _, err := svc.AuthorizeRecap(context.Background(), "main-reader", "guessed", "actmem_read"); err == nil {
		t.Fatal("unknown run authorized")
	}

	f.restart(t)
	svc = f.svc
	f.reply = func(int) *schema.Message { return schema.AssistantMessage("after restart", nil) }
	next := f.run(t, "read retained authorized activation")
	if next.Status != domain.RunCompleted {
		t.Fatalf("restart=%+v %s", next, issue58FailurePayload(t, f, next))
	}
	rehydrated := f.capture()
	if len(rehydrated) != 4 || !strings.Contains(rehydrated[3], `"name":"actmem_read"`) {
		t.Fatal("authorized activation did not survive restart")
	}
}
func TestIssue58RecapUnauthorizedDiscovery(t *testing.T) {
	var svc *Service
	tool := &issue58GovernedTool{service: &svc}
	f := newIssue58Wire(t, tool)
	svc = f.svc
	mustCreateSession(t, f.b, "ordinary")
	f.reply = func(i int) *schema.Message {
		if i == 0 {
			return issue58RecapToolReply("tool_search", `{"query":"select:actmem_read"}`)
		}
		return schema.AssistantMessage("done", nil)
	}
	id, err := svc.Run(context.Background(), "ordinary", "find resident recap")
	if err != nil {
		t.Fatal(err)
	}
	svc.WaitIdle(context.Background())
	for _, ev := range replayAll(t, f.b, id) {
		if ev.Type == domain.EventToolFinished {
			var p payloadToolFinished
			json.Unmarshal(ev.Payload, &p)
			if p.ToolName == "tool_search" && strings.Contains(p.Result, "actmem_read") {
				t.Fatalf("ordinary discovery leaked: %s", p.Result)
			}
		}
	}
	if tool.calls != 0 {
		t.Fatal("ordinary mutated/read owner")
	}
}

func TestIssue58RecapStatusIsNotLegacyPressure(t *testing.T) {
	f := newIssue58Wire(t)
	f.completed(t, "contributing-source", "status-history")
	status, err := f.svc.ContextStatus(context.Background(), "main-reader")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(status)
	var wire struct {
		Available *bool  `json:"context_pressure_available"`
		Reason    string `json:"context_reason"`
	}
	json.Unmarshal(raw, &wire)
	if wire.Available == nil || *wire.Available || wire.Reason != "recap_context_measurement_unavailable" {
		t.Fatalf("legacy pressure presented as unified context: %s", raw)
	}
}

func TestIssue58RecapSearchFailureRetainsActualOutcome(t *testing.T) {
	var svc *Service
	tool := &issue58GovernedTool{service: &svc}
	f := newIssue58Wire(t, tool)
	svc = f.svc
	f.reply = func(int) *schema.Message { return issue58RecapToolReply("tool_search", `{"query":true}`) }
	run := f.run(t, "invalid search arguments")
	if run.Status != domain.RunFailed {
		t.Fatalf("invalid native search unexpectedly succeeded %+v", run)
	}
	op, err := f.b.GetToolOperation(context.Background(), run.ID, "recap-tool_search")
	if err != nil || op.State != domain.ToolOperationCompleted || op.Failure == "" {
		t.Fatalf("actual failed outcome not retained %+v %v", op, err)
	}
	if _, _, err = f.b.ReadSubjectRunEvidence(context.Background(), conformanceBinding(), run.ID); err != nil {
		t.Fatalf("search failure lost retained evidence: %v", err)
	}
	if tool.calls != 0 {
		t.Fatal("failed discovery executed governed tool")
	}
}
