package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/schema"
)

type issue58Wire struct {
	svc      *Service
	b        *sqlite.Backend
	owner    *issue58BoundOwner
	mu       sync.Mutex
	requests []string
	sources  map[domain.SessionID]storage.SubjectSourceBinding
	reply    func(int) *schema.Message
}

func newIssue58Wire(t *testing.T, availableTools ...tools.Tool) *issue58Wire {
	t.Helper()
	f := &issue58Wire{sources: map[domain.SessionID]storage.SubjectSourceBinding{}, reply: func(int) *schema.Message { return schema.AssistantMessage("complete", nil) }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		index := len(f.requests)
		f.requests = append(f.requests, string(raw))
		f.mu.Unlock()
		reply := f.reply(index)
		delta := map[string]any{"role": "assistant", "content": reply.Content}
		reason := "stop"
		if len(reply.ToolCalls) > 0 {
			delta["tool_calls"] = reply.ToolCalls
			reason = "tool_calls"
		}
		payload, _ := json.Marshal(map[string]any{"id": "fixture", "object": "chat.completion.chunk", "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", payload)
	}))
	t.Cleanup(server.Close)
	ref, err := provider.NewCatalog().RefForSelection("fixture", provider.Endpoint{Adapter: provider.AdapterOpenAICompletions, BaseURL: server.URL, DefaultModel: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	model, err := ref.Model(context.Background(), provider.ModelSpec{ID: "fixture", APIKey: "fixture", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	f.svc, f.b, _ = newTestService(t, testsupport.NewEchoModel())
	eng, err := NewEngine(context.Background(), model, availableTools, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxToolResultBytes: 128, Compaction: &CompactionPolicy{Enabled: true, MaxTokens: 128000, TriggerPercent: 1, KeepRecent: 1}})
	if err != nil {
		t.Fatal(err)
	}
	f.svc.engine = eng
	f.svc.SetCatalog(provider.NewCatalog(provider.Vendor{Name: "test", Endpoints: []provider.Endpoint{{Adapter: provider.AdapterOpenAICompletions, DefaultModel: "test-model", Models: []provider.Model{{ID: "test-model", ContextWindow: 128000}}}}}))
	conformance.Issue58Identity(t, f.b)
	mustCreateSession(t, f.b, "main-reader")
	issue58Sources(f.svc, f.sources)
	f.owner = issue58Owner(t)
	f.svc.deps.Recap = f.owner
	t.Cleanup(func() { f.svc.CancelAll(); f.svc.WaitIdle(context.Background()) })
	return f
}
func (f *issue58Wire) run(t *testing.T, text string) domain.Run {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id, err := f.svc.Run(ctx, "main-reader", text)
	if err != nil {
		t.Fatal(err)
	}
	if !f.svc.WaitIdle(ctx) {
		t.Fatal("run did not settle")
	}
	run, err := f.b.GetRun(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range f.capture() {
		t.Logf("HTTP_CAPTURE %s", raw)
	}
	return run
}
func (f *issue58Wire) capture() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}
func (f *issue58Wire) completed(t *testing.T, sid, id string) storage.SubjectInteraction {
	t.Helper()
	source := conformance.Issue58Source(domain.SessionID(sid), false)
	f.sources[source.SessionID] = source
	mustCreateSession(t, f.b, source.SessionID)
	row := conformance.Issue58Completed(t, f.b, source, id)
	sealed, err := f.b.SealSubjectInteraction(context.Background(), source.SubjectHistoryBinding, row)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func TestIssue58SameRunCompleteToolOutputActualRequest(t *testing.T) {
	exact := "full-start-" + strings.Repeat("exact authorized bytes ", 150) + "-full-end"
	tool := &issue58ExactTool{result: exact}
	f := newIssue58Wire(t, tool)
	f.reply = func(i int) *schema.Message {
		if i == 0 {
			return schema.AssistantMessage("", []schema.ToolCall{{ID: "exact", Type: "function", Function: schema.FunctionCall{Name: tool.Spec().Name, Arguments: `{"number":1}`}}})
		}
		return schema.AssistantMessage("complete", nil)
	}
	run := f.run(t, "use the complete tool output")
	requests := f.capture()
	if run.Status != domain.RunCompleted || len(requests) != 2 {
		t.Fatalf("run=%+v requests=%d events=%+v", run, len(requests), replayAll(t, f.b, run.ID))
	}
	if !strings.Contains(requests[1], exact) {
		t.Fatalf("same-run tool output compacted before actual provider: %s", requests[1])
	}
}

func TestIssue58CoveredSupersessionAdmission(t *testing.T) {
	f := newIssue58Wire(t)
	row := f.completed(t, "archived-A", "covered-A")
	binding, err := f.svc.AuthorizeSubjectHistory(context.Background(), "main-reader")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.svc.CaptureSubjectContext(context.Background(), "main-reader")
	if err != nil {
		t.Fatal(err)
	}
	digest, err := encodeRecapReceipt(recapReceipt{Version: 1, ProjectionPolicy: 1, Binding: binding, Range: recapReceiptRange{Through: row.Sequence, ExactOriginalsDigest: row.SourceDigest}, Dispositions: recapReceiptDispositions{Digest: snapshot.DispositionDigest}, CorrectionRefs: []storage.SubjectDisposition{}, CorrectionEvidenceDigest: sha256Hex(nil)})
	if err != nil {
		t.Fatal(err)
	}
	head, err := f.owner.CommitRecap(context.Background(), cognitivecontract.RecapCommit{OperationID: "initial", Cause: "summary", Text: "covered fact", Sources: cognitivecontract.SourceRange{Through: row.Sequence, Digest: digest}})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.b.ArchiveRetainedSession(context.Background(), row.SessionID); err != nil {
		t.Fatal(err)
	}
	if err = f.b.RecordSessionTruncation(context.Background(), storage.SessionTruncation{SessionID: row.SessionID, CutoffMessageID: "user_covered-A", TailMessageID: "user_covered-A", Reason: storage.TruncationRewind}); err != nil {
		t.Fatal(err)
	}
	run := f.run(t, "new current B")
	if run.Status != domain.RunFailed || len(f.capture()) != 0 {
		t.Fatalf("mandatory reconciliation reached HTTP %+v", run)
	}
	events := replayAll(t, f.b, run.ID)
	var payloads []json.RawMessage
	for _, event := range events {
		payloads = append(payloads, event.Payload)
	}
	raw, _ := json.Marshal(payloads)
	if !strings.Contains(string(raw), "recap_reconciliation_required") {
		t.Fatalf("missing explicit refusal: %s", raw)
	}
	after, err := f.owner.ReadRecap(context.Background())
	if err != nil || after != head {
		t.Fatal("Task1 changed owner/coverage")
	}
}
