package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// TestToolSearchActivatesDeferredTool is the E3 acceptance path end to
// end: the model calls tool_search over the deferred catalog, the match
// journals as a session activation (tools.exposure semantics), and the
// next model leg sees the activated tool in its catalog and calls it.
func TestToolSearchActivatesDeferredTool(t *testing.T) {
	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-search",
			Function: schema.FunctionCall{Name: officialToolSearchName, Arguments: `{"query":"echo"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-echo",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"parity"}`},
		}}),
		schema.AssistantMessage("searched and echoed.", nil),
	)
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "search.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	ts, err := tools.Builtin(backend).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	eng, err := NewEngine(ctx, model, ts, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "test", "test-model", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Sessions: backend, Sink: newTestSink(), Truncations: backend,
	})
	t.Cleanup(func() {
		svc.CancelAll()
		svc.WaitIdle(context.Background())
	})
	mustCreateSession(t, backend, "sess-search")

	runID, err := svc.Run(ctx, "sess-search", "find the echo tool")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	var requests []payloadModelRequestV3
	sawSearchFinish, sawEchoFinish := false, false
	for _, ev := range replayAll(t, backend, runID) {
		switch ev.Type {
		case domain.EventModelRequest:
			var req payloadModelRequestV3
			mustUnmarshal(t, ev.Payload, &req)
			requests = append(requests, req)
		case domain.EventToolFinished:
			var finished payloadToolFinished
			mustUnmarshal(t, ev.Payload, &finished)
			if finished.ToolName == officialToolSearchName {
				sawSearchFinish = true
				var result struct {
					Matches []string `json:"matches"`
				}
				if err := json.Unmarshal([]byte(finished.Result), &result); err != nil {
					t.Fatalf("tool_search result not parseable: %q", finished.Result)
				}
				if len(result.Matches) != 1 || result.Matches[0] != tools.EchoInfoName {
					t.Fatalf("tool_search matches = %v, want [echo_info]", result.Matches)
				}
			}
			if finished.ToolName == tools.EchoInfoName {
				sawEchoFinish = true
				if !strings.HasSuffix(finished.Result, "parity") {
					t.Fatalf("echo result = %q", finished.Result)
				}
			}
		}
	}
	if !sawSearchFinish || !sawEchoFinish {
		t.Fatal("run missing tool_search or echo_info completion")
	}
	if len(requests) < 3 {
		t.Fatalf("model.request count = %d, want >= 3", len(requests))
	}
	// Legs after the search must list the activated tool in the model
	// catalog — the session activation + forward-selection projection.
	activated := false
	for _, req := range requests[1:] {
		for _, name := range req.SelectedTools {
			if name == tools.EchoInfoName {
				activated = true
			}
		}
	}
	if !activated {
		t.Fatalf("no post-search model.request exposed echo_info: %+v", requests)
	}
	// The fold carries the match into the session activation set.
	if got := svc.ToolActivation(ctx, "sess-search"); len(got) != 1 || got[0] != tools.EchoInfoName {
		t.Fatalf("session activation = %v, want [echo_info]", got)
	}
}
