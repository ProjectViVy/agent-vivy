package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/middlewares/agentsmd"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func TestProjectAgentsMDBackendReadAndEscape(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("hello agents"), 0o600); err != nil {
		t.Fatal(err)
	}
	backend, err := NewProjectAgentsMDBackend(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := backend.Read(context.Background(), &agentsmd.ReadRequest{FilePath: "AGENTS.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "hello agents" {
		t.Fatalf("content = %q", got.Content)
	}
	if _, err := backend.Read(context.Background(), &agentsmd.ReadRequest{FilePath: "missing.md"}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing = %v, want ErrNotExist", err)
	}
	if _, err := backend.Read(context.Background(), &agentsmd.ReadRequest{FilePath: "../AGENTS.md"}); err == nil {
		t.Fatal("expected escape to fail")
	}
}

func TestEngineProjectAgentsMDInjectsHostFileInSandbox(t *testing.T) {
	ctx := context.Background()
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, AgentsMDFileName), []byte(agentsMDTestMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	workspaces := t.TempDir()
	manager, err := NewWorkspaceManager(filepath.Join(workspaces, "runs"))
	if err != nil {
		t.Fatal(err)
	}
	projectMD, err := NewProjectAgentsMDBackend(project)
	if err != nil {
		t.Fatal(err)
	}
	rec := &recordingModel{inner: NewScriptedModel(agentsMDScript()...)}
	eng, err := NewEngine(ctx, rec, ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		AgentsMDBackend:      projectMD,
		AgentsMDFiles:        []string{AgentsMDFileName},
	})
	if err != nil {
		t.Fatal(err)
	}
	runID := domain.RunID("run-project-agentsmd")
	ws, err := manager.Ensure(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws.Path, AgentsMDFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sandbox AGENTS.md should be absent, err=%v", err)
	}
	final := drainFinalText(t, eng.RunHistory(withRunID(ctx, runID), []*schema.Message{
		schema.UserMessage("echo something"),
	}))
	if final != "AGENTSMD-TEST-DONE" {
		t.Fatalf("final answer = %q", final)
	}
	for i, input := range rec.snapshot() {
		count, _, _ := countInjected(input)
		if count != 1 {
			t.Fatalf("model call %d: injected = %d, want 1 from host AGENTS.md", i, count)
		}
		joined := ""
		for _, msg := range input {
			if msg != nil {
				joined += msg.Content
			}
		}
		if !strings.Contains(joined, agentsMDTestMarker) {
			t.Fatalf("model call %d missing host marker", i)
		}
	}
}

func TestEngineReadsInitCreatedProjectInstructionsOnNextTurn(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, AgentsMDFileName), []byte("original root rules"), 0o600); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "pkg")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	instructions, err := DiscoverProjectInstructions(child)
	if err != nil {
		t.Fatal(err)
	}
	backend, err := NewProjectAgentsMDBackend(instructions.Root)
	if err != nil {
		t.Fatal(err)
	}
	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	rec := &recordingModel{inner: NewScriptedModel(
		schema.AssistantMessage("FIRST", nil),
		schema.AssistantMessage("SECOND", nil),
	)}
	eng, err := NewEngine(ctx, rec, ts, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		AgentsMDBackend: backend, AgentsMDFiles: instructions.AgentsMDFiles,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := drainFinalText(t, eng.RunHistory(withRunID(ctx, "run-before-init"), []*schema.Message{schema.UserMessage("before")})); got != "FIRST" {
		t.Fatalf("before = %q", got)
	}
	if err := os.WriteFile(filepath.Join(child, AgentsMDFileName), []byte(agentsMDTestMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := drainFinalText(t, eng.RunHistory(withRunID(ctx, "run-after-init"), []*schema.Message{schema.UserMessage("after")})); got != "SECOND" {
		t.Fatalf("after = %q", got)
	}
	inputs := rec.snapshot()
	if len(inputs) != 2 {
		t.Fatalf("model calls = %d, want 2", len(inputs))
	}
	for i, input := range inputs {
		joined := ""
		for _, message := range input {
			if message != nil {
				joined += message.Content
			}
		}
		if !strings.Contains(joined, "original root rules") || strings.Contains(joined, agentsMDTestMarker) != (i == 1) {
			t.Fatalf("model call %d instruction snapshot = %q", i, joined)
		}
	}
}

func TestEngineSelectedWorkspaceAgentsMDReplacesLaunchProject(t *testing.T) {
	ctx := context.Background()
	launchProject := t.TempDir()
	selectedProject := t.TempDir()
	const launchMarker = "LAUNCH-PROJECT-INSTRUCTIONS"
	const selectedMarker = "SELECTED-WORKSPACE-INSTRUCTIONS"
	if err := os.WriteFile(filepath.Join(launchProject, AgentsMDFileName), []byte(launchMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(selectedProject, AgentsMDFileName), []byte(selectedMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	backend, err := NewSessionProjectAgentsMDBackend(launchProject, []string{AgentsMDFileName}, workspaceSessionLookup{
		"session-selected": {ID: "session-selected", WorkspacePath: selectedProject},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	rec := &recordingModel{inner: NewScriptedModel(agentsMDScript()...)}
	eng, err := NewEngine(ctx, rec, ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		AgentsMDBackend:      backend,
		AgentsMDFiles:        []string{AgentsMDFileName},
	})
	if err != nil {
		t.Fatal(err)
	}
	runCtx := withSessionID(withRunID(ctx, "run-selected-instructions"), "session-selected")
	if final := drainFinalText(t, eng.RunHistory(runCtx, []*schema.Message{schema.UserMessage("follow the project")})); final != "AGENTSMD-TEST-DONE" {
		t.Fatalf("final answer = %q", final)
	}
	for i, input := range rec.snapshot() {
		joined := ""
		for _, msg := range input {
			if msg != nil {
				joined += msg.Content
			}
		}
		if !strings.Contains(joined, selectedMarker) {
			t.Fatalf("model call %d missing selected workspace instructions", i)
		}
		if strings.Contains(joined, launchMarker) {
			t.Fatalf("model call %d leaked launch-project instructions", i)
		}
	}
}
