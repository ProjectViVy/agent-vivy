package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// TestServiceSessionPinRestoresSkillMountedTools pins TT-1: a tool mounted
// by skill_view in one run stays callable in a later run of the same
// session without viewing the skill again. echo_info is a hidden tool, so
// only the session-pinned mount can admit its call in run 2.
func TestServiceSessionPinRestoresSkillMountedTools(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "mounts-pin.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.BuiltinWithCapabilities(backend, nil, mountSkillOps{}).
		Resolve([]string{tools.SkillViewName})
	if err != nil {
		t.Fatalf("resolve toolset: %v", err)
	}
	// One scripted queue serves both runs: view skill + close (run 1),
	// then echo directly + close (run 2, no skill_view in between).
	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-skill-view-pin",
			Function: schema.FunctionCall{Name: tools.SkillViewName, Arguments: `{"name":"writer"}`},
		}}),
		schema.AssistantMessage("Skill reviewed.", nil),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-echo-pinned",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"echo pin"}`},
		}}),
		schema.AssistantMessage("Pinned tool ran in a later run.", nil),
	)
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, model, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
		HiddenTools: []tools.Tool{tools.NewEchoInfo()},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Questions: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})

	run1, err := svc.Run(ctx, "sess-pin", "view the writer skill")
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	waitForRunStatus(t, backend, run1, domain.RunCompleted)

	run2, err := svc.Run(ctx, "sess-pin", "echo directly without viewing the skill")
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	waitForRunStatus(t, backend, run2, domain.RunCompleted)

	echoDone := false
	for _, ev := range replayAll(t, backend, run2) {
		if ev.Type != domain.EventToolFinished {
			continue
		}
		var p payloadToolFinished
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatalf("decode tool.finished: %v", err)
		}
		if p.ToolName != tools.EchoInfoName {
			t.Fatalf("run 2 called %q, want only the pinned echo_info", p.ToolName)
		}
		// Tool results come back wrapped in the untrusted-output envelope,
		// so match the echoed text instead of comparing the whole string.
		if p.Error == "" && strings.Contains(p.Result, "echo pin") {
			echoDone = true
		}
	}
	if !echoDone {
		t.Fatalf("echo_info did not complete successfully in run 2 of the same session")
	}
}

// TestServiceSessionPinDoesNotLeakAcrossSessions pins the negative: the
// session pin restores mounts only within their session. In a different
// session the hidden echo_info call is rejected by the adapter gate, even
// though the same storage holds the mount.
func TestServiceSessionPinDoesNotLeakAcrossSessions(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "mounts-leak.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.BuiltinWithCapabilities(backend, nil, mountSkillOps{}).
		Resolve([]string{tools.SkillViewName})
	if err != nil {
		t.Fatalf("resolve toolset: %v", err)
	}
	mountModel := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-skill-view-leak",
			Function: schema.FunctionCall{Name: tools.SkillViewName, Arguments: `{"name":"writer"}`},
		}}),
		schema.AssistantMessage("Skill reviewed.", nil),
	)
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng, err := NewEngine(ctx, mountModel, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
		HiddenTools: []tools.Tool{tools.NewEchoInfo()},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Questions: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})

	run1, err := svc.Run(ctx, "sess-pin-src", "view the writer skill")
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	waitForRunStatus(t, backend, run1, domain.RunCompleted)

	// A second engine + service over the same durable state, its own script
	// calling echo_info directly in a DIFFERENT session.
	leakModel := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-echo-leak",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"echo leak"}`},
		}}),
		schema.AssistantMessage("Unrelated final reply.", nil),
	)
	eng2, err := NewEngine(ctx, leakModel, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints,
		HiddenTools: []tools.Tool{tools.NewEchoInfo()},
	})
	if err != nil {
		t.Fatalf("new second engine: %v", err)
	}
	svc2 := NewService(eng2, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Questions: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})

	run2, err := svc2.Run(ctx, "sess-other", "echo without any skill in this session")
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	// The adapter gate rejects the call as an engine node error, so the run
	// fails closed instead of feeding the rejection back to the model.
	waitForRunStatus(t, backend, run2, domain.RunFailed)

	for _, ev := range replayAll(t, backend, run2) {
		if ev.Type != domain.EventToolFinished {
			continue
		}
		var p payloadToolFinished
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatalf("decode tool.finished: %v", err)
		}
		if p.ToolName == tools.EchoInfoName && p.Error == "" && strings.Contains(p.Result, "echo leak") {
			t.Fatalf("echo_info succeeded in a session without the mount; result = %s", p.Result)
		}
	}
}
