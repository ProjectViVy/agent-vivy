package runtime

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	"agent-vivy/internal/tools"
)

// EngineConfig carries the tunables the app layer reads from config.
type EngineConfig struct {
	// StreamBuffer sizes the downstream event fan-out channel (C4).
	StreamBuffer int
	// MaxEventPayloadBytes caps a single event payload (C4).
	MaxEventPayloadBytes int
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
	for _, t := range ts {
		wrapped = append(wrapped, newToolAdapter(t))
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "vivy",
		Description: "Vivy, a precise personal assistant.",
		Instruction: "You are Vivy, a precise personal assistant.",
		Model:       m,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: wrapped},
		},
	})
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
	return &Engine{runner: runner, cfg: cfg}, nil
}

// Query starts one user turn and returns the raw engine event iterator.
// Options such as adk.WithCheckPointID are supplied by the service (C4/C6).
func (e *Engine) Query(ctx context.Context, text string, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	return e.runner.Query(ctx, text, opts...)
}

// Resume restarts a suspended run from its checkpoint, feeding the resume
// payload (the approval decision) back to the interrupted tool via
// params.Targets. The caller must pass the same checkpoint id the run was
// started with (docs/eino-capability-verify.md §2.2).
func (e *Engine) Resume(ctx context.Context, checkpointID string, params *adk.ResumeParams, opts ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	return e.runner.ResumeWithParams(ctx, checkpointID, params, opts...)
}
