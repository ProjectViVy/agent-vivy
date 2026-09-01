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
