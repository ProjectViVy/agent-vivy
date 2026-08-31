package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/adk"
	einoskill "github.com/cloudwego/eino/adk/middlewares/skill"
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
	// MaxToolResultBytes bounds a tool result before it is returned to Eino
	// and therefore before it can consume the model's next context window.
	MaxToolResultBytes int
	// Checkpoints wires the two-layer checkpoint bridge (C6). Nil leaves
	// the runner without persistence, which is how the model-only tests
	// run.
	Checkpoints *VersionedCheckpointStore
	// Policy is the immutable governance engine shared by all tool adapters.
	// Nil uses the built-in default profiles.
	Policy *PolicyEngine
	// ToolHooks is the in-process pre/post execution chain.
	ToolHooks *ToolHookChain
	// AutoApproveTools lists effectful tools that skip HITL under the
	// session approval policy "auto".
	AutoApproveTools []string
	// SkillBackend wires Eino's skill middleware so the model can discover
	// and load SKILL.md content without a keyword match. Nil leaves the
	// existing catalog-only tools (skills_list / skill_view / skill_manage)
	// as the only Skill surface. Mutation stays on skill_manage.
	SkillBackend einoskill.Backend
	// Compaction enables the Eino-native context compression middlewares
	// (reduction + summarization). Nil keeps the legacy byte-truncation-only
	// feed behavior.
	Compaction *CompactionPolicy
	// AgentsMDBackend supplies workspace AGENTS.md content for the run
	// preamble (D6). Nil disables injection. Eino's agentsmd middleware
	// loads it per run and injects it transiently before the first user
	// message, so the content never enters the persisted transcript and
	// compaction needs no carve-out.
	AgentsMDBackend AgentsMDBackend
	// HiddenTools are registered-but-not-active tools. They join the
	// executable universe so a skill_view mount can use them mid-run, but
	// the surface middleware never advertises them before they are mounted.
	HiddenTools []tools.Tool
}

// Engine owns the Eino ChatModelAgent + Runner behind the Vivy runtime.
// Together with internal/provider it is the only code allowed to touch
// Eino types (D-007).
type Engine struct {
	runner *adk.Runner
	cfg    EngineConfig
	// chatModel is the wrapped provider model; the service reuses it for
	// session-level summary generation (context/compact).
	chatModel model.ToolCallingChatModel
	// toolSpecs mirrors the resolved tool set for the per-run prompt
	// composer (MA-2); the engine never needs the callables here.
	toolSpecs []domain.ToolSpec
	// activeTools is the config-resolved surface bound on every request,
	// in registry order.
	activeTools []tools.Tool
	toolByName  map[string]tools.Tool
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
	if cfg.Policy == nil {
		var err error
		cfg.Policy, err = NewPolicyEngine(nil)
		if err != nil {
			return nil, err
		}
	}
	wrapped := make([]einotool.BaseTool, 0, len(ts)+len(cfg.HiddenTools))
	specs := make([]domain.ToolSpec, 0, len(ts))
	byName := make(map[string]tools.Tool, len(ts)+len(cfg.HiddenTools))
	for _, t := range ts {
		wrapped = append(wrapped, newEnhancedToolAdapter(newToolAdapter(t, cfg.MaxToolResultBytes, cfg.Policy, cfg.ToolHooks, cfg.AutoApproveTools)))
		specs = append(specs, t.Spec())
		byName[t.Spec().Name] = t
	}
	// Hidden tools execute only after a skill_view mounts them; they never
	// reach the model's view before that (toolSurfaceMiddleware).
	universeNames := make([]string, 0, len(specs)+len(cfg.HiddenTools))
	for _, spec := range specs {
		universeNames = append(universeNames, spec.Name)
	}
	for _, t := range cfg.HiddenTools {
		wrapped = append(wrapped, newEnhancedToolAdapter(newToolAdapter(t, cfg.MaxToolResultBytes, cfg.Policy, cfg.ToolHooks, cfg.AutoApproveTools)))
		universeNames = append(universeNames, t.Spec().Name)
		byName[t.Spec().Name] = t
	}
	activeNames := make([]string, 0, len(specs))
	activeNames = append(activeNames, universeNames[:len(specs)]...)
	handlers := []adk.ChatModelAgentMiddleware{newToolSurfaceMiddleware(activeNames, universeNames)}
	if cfg.SkillBackend != nil {
		// Registered after the surface middleware so its injected skill
		// tool is a foreign name the view filter never hides. Inline load
		// only; fork frontmatter is left to Eino's native error.
		skillHandler, err := einoskill.NewMiddleware(ctx, &einoskill.Config{Backend: cfg.SkillBackend})
		if err != nil {
			return nil, fmt.Errorf("runtime: skill middleware: %w", err)
		}
		handlers = append(handlers, skillHandler)
	}
	if cfg.Compaction != nil && cfg.Compaction.Enabled {
		// Eino-native compression (research AGENT-LOOP-PORT-COMPARISON
		// §4.3-E2): reduction clears old tool turns deterministically, then
		// summarization LLM-compacts what remains when the feed is still
		// over the trigger. m is a model.ToolCallingChatModel, so it also
		// satisfies the summarization middleware's BaseModel requirement.
		compHandlers, err := buildCompactionHandlers(ctx, m, *cfg.Compaction, cfg.MaxContextBytes)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, compHandlers...)
	}
	if mdHandler, err := buildAgentsMDHandler(ctx, cfg.AgentsMDBackend); err != nil {
		return nil, err
	} else if mdHandler != nil {
		// Registered after the compaction handlers: the injected AGENTS.md
		// message is transient (never persisted), so summarization runs
		// first and cannot compact it away — the middleware's recommended
		// ordering.
		handlers = append(handlers, mdHandler)
	}
	agentCfg := &adk.ChatModelAgentConfig{
		Name:        "vivy",
		Description: "Vivy, a precise personal assistant.",
		Instruction: composeStaticInstruction(),
		Model:       m,
		Handlers:    handlers,
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
	return &Engine{runner: runner, cfg: cfg, chatModel: m, toolSpecs: specs, activeTools: append([]tools.Tool(nil), ts...), toolByName: byName}, nil
}

// PrepareProposal asks an effectful tool for a bounded review plan before the
// runner is suspended. Tools that do not implement ProposalProvider retain the
// legacy empty proposal shape.
func (e *Engine) PrepareProposal(ctx context.Context, name string, args json.RawMessage) (domain.ToolProposal, error) {
	t, ok := e.toolByName[name]
	if !ok {
		return domain.ToolProposal{}, nil
	}
	provider, ok := t.(tools.ProposalProvider)
	if !ok {
		return domain.ToolProposal{}, nil
	}
	return provider.PrepareProposal(tools.WithRunID(ctx, contextRunID(ctx)), args)
}

// SelectTools returns the full active surface: every tool the config
// resolved, in registry order. Every request binds this complete set —
// the former keyword selector that narrowed (and routinely emptied) the
// surface per request is retired; tools.enabled stays the only admission
// gate. The selection is enforced by the adapter through the run context.
func (e *Engine) SelectTools() tools.Selection {
	return tools.Selection{
		Tools: append([]tools.Tool(nil), e.activeTools...),
		Specs: append([]domain.ToolSpec(nil), e.toolSpecs...),
	}
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
