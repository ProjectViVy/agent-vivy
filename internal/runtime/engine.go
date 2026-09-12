package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/adk"
	einotoolsearch "github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch"
	einoskill "github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/skillhost"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/skillsource"
)

const officialToolSearchName = "tool_search"

// SummaryModel is the opaque engine seam for an alternative compaction
// summarizer (CMP-2). The app layer builds it in internal/provider (D9
// override model) and passes it through without touching eino directly
// (D-007 quarantine).
type SummaryModel = model.BaseModel[*schema.Message]

// EngineConfig carries the tunables the app layer reads from config.
type EngineConfig struct {
	// ContextHost is the composed, bounded source surface. It is optional so
	// direct runtime tests and builds without an explicit bridge stay inert.
	ContextHost *contexthost.Host
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
	// and load SKILL.md content without a keyword match, and — when the
	// backend implements AlwaysSkills — injects `always: true` skill bodies
	// into every model call within the always budget (SKILL-MKT-2). Nil
	// leaves the existing catalog-only tools (skills_list / skill_view /
	// skill_manage) as the only Skill surface. Mutation stays on
	// skill_manage.
	SkillBackend einoskill.Backend
	// SkillSources are additional read-only Sources composed by SkillHost
	// alongside the mutable first-party backend. They receive the live
	// session/workspace identity from the Engine run context.
	SkillSources []skillsource.Provider
	// Optional host-owned Skill activation and authorization policy. Source
	// metadata never supplies these decisions.
	SkillActiveIDs        []string
	SkillAuthorize        skillhost.AuthorizeFunc
	SkillMaxBytes         int
	SkillAlwaysSkillBytes int
	SkillAlwaysTotalBytes int
	// Compaction enables the Eino-native context compression middlewares
	// (reduction + summarization). Nil keeps the legacy byte-truncation-only
	// feed behavior.
	Compaction *CompactionPolicy
	// SummaryModel optionally overrides the model that generates
	// compaction summaries (CMP-2). Nil keeps the main chat model. When
	// set, a summary failure falls back to the main model once.
	SummaryModel model.BaseModel[*schema.Message]
	// SummaryModelID is the provider-native id paired with SummaryModel. It
	// lets usage events distinguish the primary summary route from the main
	// model used by failover without inspecting provider implementation types.
	SummaryModelID string
	// AgentsMDBackend supplies AGENTS.md content for the run preamble (D6).
	// Nil disables injection. Eino's agentsmd middleware loads it per run
	// and injects it transiently before the first user message, so the
	// content never enters the persisted transcript and compaction needs
	// no carve-out.
	AgentsMDBackend AgentsMDBackend
	// AgentsMDFiles is the ordered list of AGENTS.md paths relative to
	// AgentsMDBackend. Empty keeps the historical single-file default.
	AgentsMDFiles []string
	// HiddenTools are registered-but-not-active tools. They join the
	// executable universe so a skill_view mount can use them mid-run, but
	// the mount projection never advertises them before they are mounted.
	HiddenTools []tools.Tool
	// OffloadBackend receives cleared tool results from the reduction
	// middleware (CMP-1). Content lands in the run workspace under
	// compaction/clear/<call-id> and the placeholder tells the model to
	// recover it with read_file. Nil keeps placeholder-only clears.
	OffloadBackend *EinoFilesystemBackend
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

// EngineFactory pins the production LoopDriver to the Eino v0.9.13 APIs used
// by NewEngine: adk.NewChatModelAgent, adk.NewRunner, Runner.Run/Query, and
// Runner.ResumeWithParams. The model value stays inside this quarantined
// package; internal/modules/loop sees only Vivy runtime types.
type EngineFactory struct {
	model model.ToolCallingChatModel
}

func NewEngineFactory(m model.ToolCallingChatModel) *EngineFactory {
	return &EngineFactory{model: m}
}

func (factory *EngineFactory) Build(ctx context.Context, ts []tools.Tool, cfg EngineConfig) (*Engine, error) {
	if factory == nil {
		return nil, errors.New("runtime: nil engine factory")
	}
	return NewEngine(ctx, factory.model, ts, cfg)
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
	// The application keeps the mutable EinoSkillBackend for protected
	// skill-management operations. At the model boundary, immediately replace
	// that mixed read/write backend with the read-only SkillHost adapter so
	// Eino List/Get and always-skill injection cannot bypass SkillHost.
	if local, ok := cfg.SkillBackend.(*EinoSkillBackend); ok {
		hosted, err := newHostedSkillBackend(local, skillhost.Config{
			ActiveIDs: cfg.SkillActiveIDs, Authorize: cfg.SkillAuthorize,
			MaxSkillBytes:       cfg.SkillMaxBytes,
			MaxAlwaysSkillBytes: cfg.SkillAlwaysSkillBytes,
			MaxAlwaysTotalBytes: cfg.SkillAlwaysTotalBytes,
		}, cfg.SkillSources...)
		if err != nil {
			return nil, fmt.Errorf("runtime: host skill backend: %w", err)
		}
		cfg.SkillBackend = hosted
	} else if len(cfg.SkillSources) > 0 {
		// A Generation may compile read-only Skill Sources even when the
		// mutable local Skill store is disabled. Keep those providers behind
		// the same SkillHost/Eino adapter instead of silently dropping the
		// typed Assembly contribution.
		hosted, err := newHostedSkillBackend(nil, skillhost.Config{
			ActiveIDs: cfg.SkillActiveIDs, Authorize: cfg.SkillAuthorize,
			MaxSkillBytes:       cfg.SkillMaxBytes,
			MaxAlwaysSkillBytes: cfg.SkillAlwaysSkillBytes,
			MaxAlwaysTotalBytes: cfg.SkillAlwaysTotalBytes,
		}, cfg.SkillSources...)
		if err != nil {
			return nil, fmt.Errorf("runtime: host generated skill sources: %w", err)
		}
		cfg.SkillBackend = hosted
	}
	// The static node contains only the fixed-visible core plus hidden tools.
	// Every other active tool is attached to Eino's official dynamic search
	// middleware below; placing it in both lists would create duplicate names
	// in the model/tool dispatch surface.
	staticTools := make([]einotool.BaseTool, 0, len(ts)+len(cfg.HiddenTools))
	dynamicTools := make([]einotool.BaseTool, 0, len(ts))
	specs := make([]domain.ToolSpec, 0, len(ts))
	byName := make(map[string]tools.Tool, len(ts)+len(cfg.HiddenTools))
	hiddenInfos := make(map[string]*schema.ToolInfo, len(cfg.HiddenTools))
	hiddenOrder := make([]string, 0, len(cfg.HiddenTools))
	for _, t := range ts {
		if t == nil {
			return nil, errors.New("runtime: nil active tool")
		}
		spec := t.Spec()
		if spec.Name == officialToolSearchName {
			return nil, fmt.Errorf("runtime: tool name %q is reserved by Eino dynamic tool search", spec.Name)
		}
		if _, exists := byName[spec.Name]; exists {
			return nil, fmt.Errorf("runtime: duplicate tool name %q", spec.Name)
		}
		adapter := newEnhancedToolAdapter(newToolAdapter(t, cfg.MaxToolResultBytes, cfg.Policy, cfg.ToolHooks, cfg.AutoApproveTools))
		specs = append(specs, spec)
		byName[spec.Name] = t
		if isFixedVisibleTool(spec.Name) {
			staticTools = append(staticTools, adapter)
		} else {
			dynamicTools = append(dynamicTools, adapter)
		}
	}
	for _, t := range cfg.HiddenTools {
		if t == nil {
			return nil, errors.New("runtime: nil hidden tool")
		}
		spec := t.Spec()
		if spec.Name == officialToolSearchName {
			return nil, fmt.Errorf("runtime: hidden tool name %q is reserved by Eino dynamic tool search", spec.Name)
		}
		if _, exists := byName[spec.Name]; exists {
			return nil, fmt.Errorf("runtime: duplicate tool name %q", spec.Name)
		}
		adapter := newEnhancedToolAdapter(newToolAdapter(t, cfg.MaxToolResultBytes, cfg.Policy, cfg.ToolHooks, cfg.AutoApproveTools))
		info, err := adapter.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("runtime: hidden tool %q info: %w", spec.Name, err)
		}
		staticTools = append(staticTools, adapter)
		hiddenInfos[spec.Name] = info
		hiddenOrder = append(hiddenOrder, spec.Name)
		byName[spec.Name] = t
	}
	// With no deferred tools every active tool is fixed-visible, so the
	// staticTools slice already contains the complete active surface.
	handlers := make([]adk.ChatModelAgentMiddleware, 0, 1+len(cfg.HiddenTools))
	var searchHandler adk.ChatModelAgentMiddleware
	if len(dynamicTools) > 0 {
		var err error
		searchHandler, err = einotoolsearch.New(ctx, &einotoolsearch.Config{
			DynamicTools:       dynamicTools,
			UseModelToolSearch: false,
		})
		if err != nil {
			return nil, fmt.Errorf("runtime: dynamic tool search: %w", err)
		}
	}
	if cfg.SkillBackend != nil {
		// Inline load only; fork frontmatter is left to Eino's native error.
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
		compHandlers, err := buildCompactionHandlers(ctx, m, cfg.SummaryModel, *cfg.Compaction, cfg.MaxContextBytes, cfg.OffloadBackend)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, compHandlers...)
	}
	if alwaysHandler := buildAlwaysSkillsHandler(cfg.SkillBackend); alwaysHandler != nil {
		// Registered after the compaction handlers for the same reason as
		// AGENTS.md (transient injections must not be summarized away) and
		// before the agentsmd handler so workspace instructions stay
		// closest to the first user turn.
		handlers = append(handlers, alwaysHandler)
	}
	if mdHandler, err := buildAgentsMDHandler(ctx, cfg.AgentsMDBackend, cfg.AgentsMDFiles); err != nil {
		return nil, err
	} else if mdHandler != nil {
		// Registered after the compaction handlers: the injected AGENTS.md
		// message is transient (never persisted), so summarization runs
		// first and cannot compact it away — the middleware's recommended
		// ordering.
		handlers = append(handlers, mdHandler)
	}
	// Official tool search is installed after Vivy's instruction and
	// compaction handlers. It owns only deferred active tools. The mount
	// projection runs last so a newly mounted hidden tool can be rehydrated
	// after Eino's persisted ToolInfo rewrite.
	if searchHandler != nil {
		handlers = append(handlers, searchHandler)
	}
	if len(hiddenInfos) > 0 {
		handlers = append(handlers, newMountedToolVisibilityMiddleware(hiddenInfos, hiddenOrder))
	}
	agentCfg := &adk.ChatModelAgentConfig{
		Name:        "vivy",
		Description: "Vivy, a precise personal assistant.",
		Instruction: composeStaticInstruction(),
		Model:       observeModelStreams(m),
		Handlers:    handlers,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: staticTools},
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

// SelectTools returns the complete active business-tool allowlist, including
// both fixed-visible and deferred active tools, in registry/config order. It
// describes executable Vivy tools, not the progressive model-visible surface:
// the framework-owned tool_search meta-tool is intentionally absent. The
// selection is enforced by the adapter through the run context.
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
