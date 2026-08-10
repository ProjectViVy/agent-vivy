package runtime

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// EngineConfig carries the tunables the app layer reads from config.
type EngineConfig struct {
	// StreamBuffer sizes the downstream event fan-out channel (C4).
	StreamBuffer int
	// MaxEventPayloadBytes caps a single event payload (C4).
	MaxEventPayloadBytes int
	// MaxToolTurns caps the model's generation cycles per run (MA-4);
	// exceeding it fails the run with a classified terminal. Zero keeps
	// eino's own default.
	MaxToolTurns int
	// MaxContextBytes bounds the transient UTF-8 context sent to one run.
	// Zero leaves the direct runtime test harness unbounded.
	MaxContextBytes int
	// MaxHistoryMessages bounds retained user/assistant transcript rows.
	// Zero leaves the direct runtime test harness unbounded.
	MaxHistoryMessages int
	// Checkpoints wires the two-layer checkpoint bridge (C6). Nil leaves
	// the runner without persistence, which is how the model-only tests
	// run.
	Checkpoints *VersionedCheckpointStore
}

// Engine owns the Eino ChatModelAgent + Runner behind the Vivy runtime.
// Together with internal/provider it is the only code allowed to touch
// Eino types (D-007).
type Engine struct {
	runner *adk.Runner
	cfg    EngineConfig
	// toolSpecs mirrors the resolved tool set for the per-run prompt
	// composer (MA-2); the engine never needs the callables here.
	toolSpecs []domain.ToolSpec
}

// NewEngine builds the ChatModelAgent and Runner over an Eino
// tool-calling chat model and the resolved tool set. Domain models cross
// the boundary via WrapModel at wiring time; native eino-ext components
// are passed in directly. Streaming is always enabled; resume reuses the
// mode persisted in the checkpoint (docs/eino-capability-verify.md 2.2).
// When cfg.Checkpoints is wired, the runner persists interrupt points
// through the versioned blob bridge; a nil store keeps interrupts
// non-resumable (eino skips persistence without a CheckPointStore).
func NewEngine(ctx context.Context, m model.ToolCallingChatModel, ts []tools.Tool, cfg EngineConfig) (*Engine, error) {
	if m == nil {
		return nil, errors.New("runtime: nil model")
	}
	wrapped := make([]einotool.BaseTool, 0, len(ts))
	specs := make([]domain.ToolSpec, 0, len(ts))
	for _, t := range ts {
		wrapped = append(wrapped, newToolAdapter(t))
		specs = append(specs, t.Spec())
	}
	agentCfg := &adk.ChatModelAgentConfig{
		Name:        "vivy",
		Description: "Vivy, a precise personal assistant.",
		Instruction: composeStaticInstruction(specs),
		Model:       m,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: wrapped},
		},
	}
	if cfg.MaxToolTurns > 0 {
		// Loop guardrail (MA-4): eino counts one iteration per model
		// generation cycle and surfaces ErrExceedMaxIterations past the
		// cap, which the service classifies into a run.failed terminal.
		agentCfg.MaxIterations = cfg.MaxToolTurns
	}
	agent, err := adk.NewChatModelAgent(ctx, agentCfg)
	if err != nil {
		return nil, err
	}
	runnerCfg := adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	}
	if cfg.Checkpoints != nil {
		runnerCfg.CheckPointStore = NewEinoCheckpointAdapter(cfg.Checkpoints)
	}
	runner := adk.NewRunner(ctx, runnerCfg)
	return &Engine{runner: runner, cfg: cfg, toolSpecs: specs}, nil
}

// Query starts one user turn and returns the raw engine event iterator.
// Options such as adk.WithCheckPointID are supplied by the service (C4/C6).
func (e *Engine) Query(ctx context.Context, text string, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	return e.runner.Query(ctx, text, opts...)
}

// RunHistory starts a run over an explicit message list — the session
// transcript the service rebuilds from the message store (MA-1, ADR-009).
// eino's Runner.Query is exactly this call with a single fresh user
// message, so the checkpoint/resume contract carries over unchanged
// (docs/v1-minimal-agent-proposal.md §1).
func (e *Engine) RunHistory(ctx context.Context, msgs []*schema.Message, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	return e.runner.Run(ctx, msgs, opts...)
}

// Resume restarts a suspended run from its checkpoint, feeding the resume
// payload (the approval decision) back to the interrupted tool via
// params.Targets. The caller must pass the same checkpoint id the run was
// started with (docs/eino-capability-verify.md §2.2).
func (e *Engine) Resume(ctx context.Context, checkpointID string, params *adk.ResumeParams, opts ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	return e.runner.ResumeWithParams(ctx, checkpointID, params, opts...)
}
