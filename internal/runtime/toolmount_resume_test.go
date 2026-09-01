package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// mountSkillOps is a minimal SkillOperations stub whose ViewSkill always
// resolves a skill named "writer" that declares echo_info. Only ViewSkill is
// exercised; the remaining methods exist to satisfy the interface.
type mountSkillOps struct{}

func (mountSkillOps) ListSkills(context.Context, domain.RunID) ([]tools.SkillSummary, error) {
	return []tools.SkillSummary{{
		Name: "writer", Description: "declares echo_info", Tools: []string{tools.EchoInfoName},
		Enabled: true, Hash: "test-hash",
	}}, nil
}

func (mountSkillOps) ViewSkill(_ context.Context, _ domain.RunID, name, _ string) (tools.SkillView, error) {
	return tools.SkillView{
		SkillSummary: tools.SkillSummary{
			Name: name, Description: "declares echo_info", Tools: []string{tools.EchoInfoName},
			Enabled: true, Hash: "test-hash",
		},
		Content:      "skill body",
		RelativePath: "SKILL.md",
	}, nil
}

func (mountSkillOps) ManageSkill(context.Context, domain.RunID, tools.SkillManageRequest) (tools.SkillManageResult, error) {
	return tools.SkillManageResult{}, errors.New("not exercised by this test")
}

func (mountSkillOps) PrepareSkillProposal(context.Context, domain.RunID, tools.SkillManageRequest) (domain.ToolProposal, error) {
	return domain.ToolProposal{}, errors.New("not exercised by this test")
}

func (mountSkillOps) ApplySkillRevision(context.Context, domain.RunID, string) (tools.SkillManageResult, error) {
	return tools.SkillManageResult{}, errors.New("not exercised by this test")
}

func (mountSkillOps) SetSkillEnabled(context.Context, string, bool, string) (tools.SkillSummary, error) {
	return tools.SkillSummary{}, errors.New("not exercised by this test")
}

// TestServiceResumeRestoresSkillMountedTools pins TT-2: a tool mounted by
// skill_view before an ask_user interrupt stays callable after the answer
// resumes the run. echo_info is a hidden tool, so it is outside the selected
// surface and only the mount path can admit its call.
func TestServiceResumeRestoresSkillMountedTools(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "mounts.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.BuiltinWithCapabilities(backend, nil, mountSkillOps{}).
		Resolve([]string{tools.SkillViewName, tools.AskUserName})
	if err != nil {
		t.Fatalf("resolve toolset: %v", err)
	}

	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-skill-view-1",
			Function: schema.FunctionCall{Name: tools.SkillViewName, Arguments: `{"name":"writer"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       QuestionFlowCallID,
			Function: schema.FunctionCall{Name: tools.AskUserName, Arguments: `{"question":"Which color should I use?"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-echo-resumed-1",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"post-resume echo"}`},
		}}),
		schema.AssistantMessage("Mounted tool ran after resume.", nil),
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

	runID, err := svc.Run(ctx, "sess-mount-resume", "view the writer skill, then ask me for a color")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	question := waitForPendingQuestion(t, backend, runID)
	if err := svc.AnswerQuestion(ctx, question.ID, "blue"); err != nil {
		t.Fatalf("answer: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	answerIdx := indexOfType(events, domain.EventUserQuestionAnswered)
	if answerIdx < 0 {
		t.Fatalf("question answer event missing: %v", events)
	}
	echoDone := false
	for _, ev := range events[answerIdx+1:] {
		if ev.Type != domain.EventToolFinished {
			continue
		}
		var p payloadToolFinished
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatalf("decode tool.finished: %v", err)
		}
		// Tool results come back wrapped in the untrusted-output envelope,
		// so match the echoed text instead of comparing the whole string.
		if p.ToolName == tools.EchoInfoName && p.Error == "" && strings.Contains(p.Result, "post-resume echo") {
			echoDone = true
		}
	}
	if !echoDone {
		t.Fatalf("echo_info did not complete successfully after resume; events = %v", events)
	}
}

// TestServiceRecoverRestoresSkillMountedTools pins the restart half of the
// mount story: the live mount registry is memory-only, but tool.mounted
// events are durable, so restart recovery must rebuild the mounts from the
// journal before the answered question resumes the run. echo_info stays a
// hidden tool in the restarted engine too, so only a restored mount can
// admit its call — without recovery the resume fails.
func TestServiceRecoverRestoresSkillMountedTools(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "mounts-recover.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.BuiltinWithCapabilities(backend, nil, mountSkillOps{}).
		Resolve([]string{tools.SkillViewName, tools.AskUserName})
	if err != nil {
		t.Fatalf("resolve toolset: %v", err)
	}
	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-skill-view-3",
			Function: schema.FunctionCall{Name: tools.SkillViewName, Arguments: `{"name":"writer"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       QuestionFlowCallID,
			Function: schema.FunctionCall{Name: tools.AskUserName, Arguments: `{"question":"Which color should I use?"}`},
		}}),
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
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Questions: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})

	runID, err := svc.Run(ctx, "sess-mount-recover", "view the writer skill, then ask me for a color")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	question := waitForPendingQuestion(t, backend, runID)

	// Restart while suspended: fresh engine + service over the same
	// durable state, echo_info still hidden so only the recovered mount
	// can admit its call.
	resumedModel := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-echo-after-restart",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"post-restart echo"}`},
		}}),
		schema.AssistantMessage("Mounted tool ran after restart.", nil),
	)
	ts2, err := tools.BuiltinWithCapabilities(backend, nil, mountSkillOps{}).
		Resolve([]string{tools.SkillViewName, tools.AskUserName})
	if err != nil {
		t.Fatalf("resolve restarted toolset: %v", err)
	}
	checkpoints2, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	eng2, err := NewEngine(ctx, resumedModel, ts2, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, Checkpoints: checkpoints2,
		HiddenTools: []tools.Tool{tools.NewEchoInfo()},
	})
	if err != nil {
		t.Fatalf("new restarted engine: %v", err)
	}
	restarted := NewService(eng2, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Questions: backend,
		ApprovalExpiration: 5 * time.Minute, Sink: newTestSink(),
	})
	if err := restarted.Recover(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}

	// Answering on the restarted service resumes the rebuilt run; the
	// mount restored from the journal admits the hidden echo_info call.
	if err := restarted.AnswerQuestion(ctx, question.ID, "blue"); err != nil {
		t.Fatalf("answer after recovery: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	events := replayAll(t, backend, runID)
	answerIdx := indexOfType(events, domain.EventUserQuestionAnswered)
	if answerIdx < 0 {
		t.Fatalf("question answer event missing: %v", events)
	}
	echoDone := false
	for _, ev := range events[answerIdx+1:] {
		if ev.Type != domain.EventToolFinished {
			continue
		}
		var p payloadToolFinished
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatalf("decode tool.finished: %v", err)
		}
		if p.ToolName == tools.EchoInfoName && p.Error == "" && strings.Contains(p.Result, "post-restart echo") {
			echoDone = true
		}
	}
	if !echoDone {
		t.Fatalf("echo_info did not complete after restart recovery; events = %v", events)
	}
}

// TestServiceJournalRecordsSkillToolMounts pins TT-3: mounting hidden tools
// mid-run journals its own tool.mounted audit event (model.request only
// records the active baseline surface).
func TestServiceJournalRecordsSkillToolMounts(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "mounts-journal.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.BuiltinWithCapabilities(backend, nil, mountSkillOps{}).Resolve([]string{tools.SkillViewName})
	if err != nil {
		t.Fatalf("resolve toolset: %v", err)
	}
	model := NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-skill-view-2",
			Function: schema.FunctionCall{Name: tools.SkillViewName, Arguments: `{"name":"writer"}`},
		}}),
		schema.AssistantMessage("Skill reviewed.", nil),
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

	runID, err := svc.Run(ctx, "sess-mount-journal", "view the writer skill")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	var mounted *payloadToolMounted
	for _, ev := range replayAll(t, backend, runID) {
		if ev.Type != domain.EventToolMounted {
			continue
		}
		var p payloadToolMounted
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatalf("decode tool.mounted: %v", err)
		}
		mounted = &p
	}
	if mounted == nil {
		t.Fatalf("no tool.mounted event journaled")
	}
	if mounted.ToolName != tools.SkillViewName || len(mounted.Tools) != 1 || mounted.Tools[0] != tools.EchoInfoName {
		t.Fatalf("tool.mounted payload = %+v, want tool_name=%s tools=[%s]", mounted, tools.SkillViewName, tools.EchoInfoName)
	}
}
