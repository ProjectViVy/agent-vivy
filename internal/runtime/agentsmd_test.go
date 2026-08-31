package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

const agentsMDTestMarker = "VIVY-AGENTSMD-MARKER: always run just ci before claiming done"

// newAgentsMDTestEngine builds an engine whose AGENTS.md seam is the real
// EinoFilesystemBackend over a per-run workspace manager, plus a recording
// model wrapper so tests can assert what the model actually saw.
func newAgentsMDTestEngine(t *testing.T, script ...*schema.Message) (*Engine, *recordingModel, *WorkspaceManager) {
	t.Helper()
	ctx := context.Background()
	ts, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	root := t.TempDir()
	manager, err := NewWorkspaceManager(filepath.Join(root, "workspaces"))
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	sandbox, err := NewSandboxManager(domain.SandboxModeWorkspaceWrite, root, nil, nil)
	if err != nil {
		t.Fatalf("sandbox manager: %v", err)
	}
	backend := NewEinoFilesystemBackend(manager, sandbox)
	rec := &recordingModel{inner: NewScriptedModel(script...)}
	eng, err := NewEngine(ctx, rec, ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		AgentsMDBackend:      backend,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return eng, rec, manager
}

// agentsMDScript is a two-turn script: one echo tool call, then the close.
func agentsMDScript() []*schema.Message {
	return []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "call-agentsmd-1",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"probe"}`},
		}}),
		schema.AssistantMessage("AGENTSMD-TEST-DONE", nil),
	}
}

// countInjected returns how many messages carry the marker, the index of
// the first one, and the index of the first user message without it.
func countInjected(input []*schema.Message) (int, int, int) {
	count, firstInjected, firstRealUser := 0, -1, -1
	for i, msg := range input {
		if msg == nil || msg.Role != schema.User {
			continue
		}
		if strings.Contains(msg.Content, agentsMDTestMarker) {
			if firstInjected < 0 {
				firstInjected = i
			}
			count++
			continue
		}
		if firstRealUser < 0 {
			firstRealUser = i
		}
	}
	return count, firstInjected, firstRealUser
}

func TestEngineAgentsMDInjection(t *testing.T) {
	ctx := context.Background()
	eng, rec, manager := newAgentsMDTestEngine(t, agentsMDScript()...)
	runID := domain.RunID("run-agentsmd-1")
	ws, err := manager.Ensure(ctx, runID)
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws.Path, AgentsMDFileName), []byte(agentsMDTestMarker), 0o600); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	final := drainFinalText(t, eng.RunHistory(withRunID(ctx, runID), []*schema.Message{
		schema.UserMessage("echo something"),
	}))
	if final != "AGENTSMD-TEST-DONE" {
		t.Fatalf("final answer = %q", final)
	}
	inputs := rec.snapshot()
	if len(inputs) != 2 {
		t.Fatalf("model calls = %d, want 2", len(inputs))
	}
	for i, input := range inputs {
		count, injectedAt, userAt := countInjected(input)
		if count != 1 {
			t.Fatalf("model call %d: injected messages = %d, want 1 (idempotent across turns)", i, count)
		}
		// The instruction system message precedes the injection; the
		// injected message must sit immediately before the first real
		// user message.
		if injectedAt != userAt-1 {
			t.Fatalf("model call %d: injection at %d, first real user message at %d", i, injectedAt, userAt)
		}
	}
}

func TestEngineAgentsMDMissingFileInjectsNothing(t *testing.T) {
	ctx := context.Background()
	eng, rec, _ := newAgentsMDTestEngine(t, agentsMDScript()...)
	runID := domain.RunID("run-agentsmd-empty")
	final := drainFinalText(t, eng.RunHistory(withRunID(ctx, runID), []*schema.Message{
		schema.UserMessage("echo something"),
	}))
	if final != "AGENTSMD-TEST-DONE" {
		t.Fatalf("final answer = %q", final)
	}
	for i, input := range rec.snapshot() {
		if count, _, _ := countInjected(input); count != 0 {
			t.Fatalf("model call %d: injected messages = %d, want 0 without AGENTS.md", i, count)
		}
	}
}

func TestEngineAgentsMDRunWorkspaceScoping(t *testing.T) {
	ctx := context.Background()
	eng, rec, manager := newAgentsMDTestEngine(t, agentsMDScript()...)
	// Only run A's workspace carries the file; run B shares the workspace
	// root but must not inherit the instructions.
	wsA, err := manager.Ensure(ctx, domain.RunID("run-agentsmd-a"))
	if err != nil {
		t.Fatalf("ensure workspace a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wsA.Path, AgentsMDFileName), []byte(agentsMDTestMarker), 0o600); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	final := drainFinalText(t, eng.RunHistory(withRunID(ctx, domain.RunID("run-agentsmd-b")), []*schema.Message{
		schema.UserMessage("echo something"),
	}))
	if final != "AGENTSMD-TEST-DONE" {
		t.Fatalf("final answer = %q", final)
	}
	for i, input := range rec.snapshot() {
		if count, _, _ := countInjected(input); count != 0 {
			t.Fatalf("model call %d: run B inherited run A's AGENTS.md", i)
		}
	}
}

// seedingWorkspaces wraps a WorkspaceManager and drops an AGENTS.md into
// every freshly ensured run workspace, mirroring a user-prepared project.
type seedingWorkspaces struct {
	inner   *WorkspaceManager
	content string
}

func (s *seedingWorkspaces) Ensure(ctx context.Context, runID domain.RunID) (Workspace, error) {
	ws, err := s.inner.Ensure(ctx, runID)
	if err != nil {
		return Workspace{}, err
	}
	if s.content != "" {
		if err := os.WriteFile(filepath.Join(ws.Path, AgentsMDFileName), []byte(s.content), 0o600); err != nil {
			return Workspace{}, err
		}
	}
	return ws, nil
}

// newAgentsMDService wires the service stack with a recording model, the
// real filesystem backend as the AGENTS.md seam, and a workspace allocator
// that seeds every run workspace with the marker file.
func newAgentsMDService(t *testing.T, toolNames []string, script ...*schema.Message) (*Service, *sqlite.Backend, *recordingModel) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "agentsmd.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	ts, err := tools.Builtin(backend).Resolve(toolNames)
	if err != nil {
		t.Fatalf("resolve tools: %v", err)
	}
	root := t.TempDir()
	manager, err := NewWorkspaceManager(filepath.Join(root, "workspaces"))
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	sandbox, err := NewSandboxManager(domain.SandboxModeWorkspaceWrite, root, nil, nil)
	if err != nil {
		t.Fatalf("sandbox manager: %v", err)
	}
	fileBackend := NewEinoFilesystemBackend(manager, sandbox)
	checkpoints, err := NewVersionedCheckpointStore(backend.Blobs(), "test-engine")
	if err != nil {
		t.Fatalf("checkpoint store: %v", err)
	}
	rec := &recordingModel{inner: NewScriptedModel(script...)}
	eng, err := NewEngine(ctx, rec, ts, EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		Checkpoints:          checkpoints,
		AgentsMDBackend:      fileBackend,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sink := newTestSink()
	svc := NewService(eng, "scripted", "scripted-v0", ServiceDeps{
		Journal: backend, Runs: backend, Messages: backend, Notes: backend, Approvals: backend,
		Questions:          backend,
		ApprovalExpiration: 5 * time.Minute,
		Workspaces:         &seedingWorkspaces{inner: manager, content: agentsMDTestMarker},
		Sink:               sink,
	})
	return svc, backend, rec
}

// TestServiceAgentsMDInjectionIsTransient drives the full service stack
// (engine -> adapters -> journal) with an AGENTS.md in the run workspace:
// the model must see the instructions while the journal and the message
// store never persist them (D6: transient injection rides above storage).
func TestServiceAgentsMDInjectionIsTransient(t *testing.T) {
	svc, backend, rec := newAgentsMDService(t,
		[]string{tools.EchoInfoName}, agentsMDScript()...)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-agentsmd", "echo something")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	// The model really saw the injected instructions.
	inputs := rec.snapshot()
	if len(inputs) != 2 {
		t.Fatalf("model calls = %d, want 2", len(inputs))
	}
	for i, input := range inputs {
		if count, injectedAt, userAt := countInjected(input); count != 1 || injectedAt != userAt-1 {
			t.Fatalf("model call %d: injection count=%d injectedAt=%d userAt=%d, want 1 and adjacent", i, count, injectedAt, userAt)
		}
	}

	// ...but nothing downstream stored it: neither the journal replay nor
	// the message transcript may carry the marker.
	for _, ev := range replayAll(t, backend, runID) {
		if strings.Contains(string(ev.Payload), agentsMDTestMarker) {
			t.Fatalf("journal event seq %d (%s) persisted the injected AGENTS.md content", ev.Seq, ev.Type)
		}
	}
	stored, err := backend.ListMessages(ctx, "sess-agentsmd")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	for _, msg := range stored {
		if strings.Contains(msg.Content, agentsMDTestMarker) {
			t.Fatalf("message store persisted the injected AGENTS.md content (role %s)", msg.Role)
		}
	}
}

// TestServiceAgentsMDInjectionSurvivesApprovalResume covers the one flow
// that round-trips the injected message through a checkpoint: an approval
// suspension. After resume the middleware's idempotency tag must have
// survived serialization — the model must still see the instructions
// exactly once, and the journal must still be free of them.
func TestServiceAgentsMDInjectionSurvivesApprovalResume(t *testing.T) {
	// The write_note approval script: turn one requests the effectful call
	// (ApprovalFlowCallID), the resumed turn closes with the final reply.
	approvalScript := []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       ApprovalFlowCallID,
			Function: schema.FunctionCall{Name: tools.WriteNoteName, Arguments: `{"content":"buy milk"}`},
		}}),
		schema.AssistantMessage("Done: the note has been handled.", nil),
	}
	svc, backend, rec := newAgentsMDService(t,
		[]string{tools.WriteNoteName}, approvalScript...)
	ctx := context.Background()

	runID, err := svc.Run(ctx, "sess-agentsmd-resume", "note that I need milk")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	if err := svc.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatalf("decide approval: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	inputs := rec.snapshot()
	if len(inputs) != 2 {
		t.Fatalf("model calls = %d, want 2 (pre- and post-resume)", len(inputs))
	}
	for i, input := range inputs {
		if count, injectedAt, userAt := countInjected(input); count != 1 || injectedAt != userAt-1 {
			t.Fatalf("model call %d: injection count=%d injectedAt=%d userAt=%d, want exactly one before the user turn", i, count, injectedAt, userAt)
		}
	}
	for _, ev := range replayAll(t, backend, runID) {
		if strings.Contains(string(ev.Payload), agentsMDTestMarker) {
			t.Fatalf("journal event seq %d (%s) persisted the injected AGENTS.md content", ev.Seq, ev.Type)
		}
	}
}
