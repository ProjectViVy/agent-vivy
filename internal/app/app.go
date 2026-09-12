// Package app is the composition root of the vivy process. It owns the
// startup order (storage -> providers -> runtime -> JSON-RPC control plane), restart
// recovery of non-terminal runs before the server listens (E2), and the
// reverse shutdown order with a bounded grace period.
//
// The config is fully validated before New is called; app never re-reads
// files for non-secret settings (config boundary, FR-10). Provider keys
// live in the user workspace settings.yaml or a frozen ENV session and
// are resolved per call. A missing key no longer aborts startup.
package app

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"agent-vivy/internal/actionhost"
	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/channelhost"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/eval"
	"agent-vivy/internal/events"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/generated/presentation"
	"agent-vivy/internal/i18n"
	"agent-vivy/internal/logging"
	"agent-vivy/internal/modelhost"
	checkpointmodule "agent-vivy/internal/modules/checkpoint"
	credentialmodule "agent-vivy/internal/modules/credential"
	loopmodule "agent-vivy/internal/modules/loop"
	modelmodule "agent-vivy/internal/modules/model"
	storagemodule "agent-vivy/internal/modules/storage"
	"agent-vivy/internal/provider"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/studio"
	"agent-vivy/internal/tools"
	"agent-vivy/internal/worker"
	"agent-vivy/sdk/generation"
	"agent-vivy/sdk/module"
	actionport "agent-vivy/sdk/port/controlaction"
	"agent-vivy/sdk/port/providerprofile"
	toolworldport "agent-vivy/sdk/port/toolworld"
	"agent-vivy/ui"
)

// shutdownGrace bounds the whole graceful shutdown window. It stays under
// the ~5s Windows CTRL_CLOSE window: a console close lets the signal
// handler run before the OS terminates the process, so the drain, the
// HTTP shutdown and the storage close must all fit inside (E4).
const shutdownGrace = 5 * time.Second

// App is the composed process.
type App struct {
	cfg    config.Config
	logger *slog.Logger

	service    *runtime.Service
	channels   *channelhost.Host
	actionHost *actionhost.Host
	backend    storage.Engine
	worker     *workerManager
	resolver   *ModelResolver
	modelHost  *modelhost.Host

	control    controlrpc.Handler
	httpServer *http.Server
	rpcToken   string
	mcpBackend *runtime.MCPBackend
	assembly   *genassembly.RuntimeAssembly
	closeOnce  sync.Once
	closeErr   error
}

// AppOption tweaks one composition of the process. The zero value is the
// default gateway assembly.
type AppOption func(*appOptions)

type appOptions struct {
	channels     bool
	gateway      bool
	sink         runtime.EventSink
	settingsPath string
	// projectRoot is deliberately opt-in. Runtime.WorkspaceRoot is the
	// tenant/sandbox workspace for ordinary Vivy processes, not necessarily
	// the code project root from which a face may resolve attachments.
	projectRoot string
	// instructionRoot is the launch directory scanned for AGENTS.md and
	// conventional skill packages. It is independent of projectRoot so the
	// web sandbox can inject project instructions without exposing @file.
	instructionRoot string
}

// WithoutEars composes the process with no channel Host: no partition, no
// run hook, no StartAll. The headless face is one terminal-bound turn and
// must never consume inbound channel traffic.
func WithoutEars() AppOption { return func(o *appOptions) { o.channels = false } }

// WithoutGateway composes the process with no HTTP gateway: no listener, no
// mux, no embedded UI, no browser origin policy (VIVY-FACE-PACK §7 — HTTP
// listening is faces/web's effect, not a kernel obligation). The control
// plane stays reachable in-process via DialControl, so a face can drive the
// same JSON-RPC methods as the web face. WithEventSink is the streaming
// path for such faces.
func WithoutGateway() AppOption { return func(o *appOptions) { o.gateway = false } }

// WithEventSink adds a second event sink next to the gateway bus. The
// headless face renders the run stream for stdout/stderr through it.
func WithEventSink(sink runtime.EventSink) AppOption {
	return func(o *appOptions) { o.sink = sink }
}

// WithSettingsPath points this process at a shared operator settings file
// while allowing its Journal and other runtime state to live elsewhere.
// This is used by independent code-face processes: provider/model/operator
// preferences are shared, but sessions, runs, approvals, and checkpoints are
// isolated in each process's own storage directory.
func WithSettingsPath(path string) AppOption {
	return func(o *appOptions) { o.settingsPath = path }
}

// WithCodeProjectRoot supplies the canonical project root owned by the
// vivy-code composition. It is kept separate from Runtime.WorkspaceRoot so
// ordinary Vivy/web compositions cannot accidentally expose a tenant
// workspace through the attachment resolver.
func WithCodeProjectRoot(path string) AppOption {
	return func(o *appOptions) { o.projectRoot = path }
}

// WithInstructionRoot supplies the launch directory whose AGENTS.md and
// .agents/.vivy skill packages are discovered and fed to the existing Eino
// agentsmd / skill middlewares. It does not change the file-tool world.
func WithInstructionRoot(path string) AppOption {
	return func(o *appOptions) { o.instructionRoot = path }
}

// developerPresentationLocale reads the development-only locale input from
// the launch root. A sealed Generation must depend solely on its embedded
// presentation settings and therefore never consult process or dotenv state.
func developerPresentationLocale(root string, sealed bool) (i18n.Locale, error) {
	if sealed {
		return "", nil
	}
	return i18n.DeveloperDefault(filepath.Join(root, ".env"))
}

// fanoutSink publishes one event to both the gateway bus and the extra
// face sink, preserving the synchronous persist order.
type fanoutSink struct {
	primary, extra runtime.EventSink
}

type assemblyHost string

func (host assemblyHost) ModuleID() string { return string(host) }

type assemblyHosts struct{}

func (assemblyHosts) ForModule(id string) module.Host { return assemblyHost(id) }

func (f fanoutSink) Publish(ev domain.RunEvent) {
	f.primary.Publish(ev)
	f.extra.Publish(ev)
}

// New builds the app from the default generated Assembly.
func New(ctx context.Context, cfg config.Config, opts ...AppOption) (*App, error) {
	return NewWithAssembly(ctx, cfg, genassembly.BuildDefault(), opts...)
}

// NewWithAssembly is the application composition boundary. General config
// syntax and policy errors are rejected by config.Load / config.Validate;
// generation-specific capability mismatches are rejected here before any
// runtime construction begins.
func NewWithAssembly(ctx context.Context, cfg config.Config, runtimeAssembly genassembly.RuntimeAssembly, opts ...AppOption) (*App, error) {
	logger := slog.Default()
	ao := appOptions{channels: true, gateway: true}
	for _, opt := range opts {
		opt(&ao)
	}
	if err := validateRuntimeAssemblyConfig(runtimeAssembly, cfg); err != nil {
		return nil, err
	}
	// Providers are not discovered, exposed, or constructed until every
	// generated Module has crossed Start and Ready. The deferred close above
	// rolls this back on every later composition failure.
	if err := runtimeAssembly.Start(ctx, assemblyHosts{}); err != nil {
		return nil, fmt.Errorf("app: start generated assembly: %w", err)
	}
	assemblyOwned := true
	defer func() {
		if assemblyOwned {
			shutdownCtx := context.WithoutCancel(ctx)
			_ = closeToolWorlds(shutdownCtx, runtimeAssembly.Worlds)
			_ = runtimeAssembly.Close(shutdownCtx)
		}
	}()
	if err := validateRuntimeAssembly(runtimeAssembly); err != nil {
		return nil, err
	}
	developerLocale, err := developerPresentationLocale(ao.instructionRoot, presentation.SealedGeneration)
	if err != nil {
		return nil, fmt.Errorf("app: resolve developer locale: %w", err)
	}
	liveSettingsPath := ao.settingsPath
	if liveSettingsPath == "" {
		liveSettingsPath = settings.Path(cfg.DataDirectory())
	}
	configProvider, configModel := providerConfigBaseline(cfg)
	// Operator-managed preferences (network search, execute ceiling, the
	// per-channel knobs) overlay the validated config. Provider keys are
	// NOT applied to the process environment; ModelResolver reads
	// settings.yaml / frozen ENV per call. The compiled plugin set is
	// registered first so the channels overlay can only name channels this
	// generation actually carries.
	cfg = applySettingsOverlayAt(ctx, logger, cfg, liveSettingsPath, compiledChannelNames(runtimeAssembly.Channels))
	if err := validateRuntimeAssemblyConfig(runtimeAssembly, cfg); err != nil {
		return nil, err
	}
	var originPolicy controlrpc.OriginPolicy
	if ao.gateway {
		policy, policyErr := controlrpc.NewOriginPolicy(cfg.Server.AllowedOrigins)
		if policyErr != nil {
			return nil, fmt.Errorf("app: configure browser origins: %w", policyErr)
		}
		originPolicy = policy
	}

	dataRoot := cfg.DataDirectory()
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		return nil, fmt.Errorf("app: create data dir: %w", err)
	}

	backend, err := storagemodule.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}

	bundlePath := func(name string) string {
		return filepath.Join(cfg.Providers.BundleDir, name+".yaml")
	}
	openaiBundle, err := provider.LoadBundle(bundlePath("openai"))
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: load openai bundle: %w", err)
	}
	anthropicBundle, err := provider.LoadBundle(bundlePath("anthropic"))
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: load anthropic bundle: %w", err)
	}
	catalog := provider.NewCatalog(openaiBundle, anthropicBundle)
	compiledProfiles := make([]providerprofile.Profile, 0, len(runtimeAssembly.ProviderProfiles))
	for _, profileProvider := range runtimeAssembly.ProviderProfiles {
		compiledProfiles = append(compiledProfiles, profileProvider.Definition())
	}
	credentialResolver, err := credentialmodule.Compose(credentialmodule.CompileScopes(compiledProfiles, cfg.Channels))
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: construct Credential Resolver: %w", err)
	}
	modelProvider, err := modelmodule.Compose(compiledProfiles, modelhost.Capabilities{
		provider.AdapterFamilyOpenAICompatible: modelhost.CapabilitySupported,
		provider.AdapterFamilyAnthropic:        modelhost.CapabilitySupported,
	})
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: construct ModelHost: %w", err)
	}
	modelHost := modelProvider.Host()
	resolver := newModelResolver(cfg, liveSettingsPath, catalog, modelHost, credentialResolver)
	cur := resolver.Current()
	providerName := cur.Provider
	if providerName == "" {
		providerName = cfg.Providers.Active
	}
	modelID := cur.Model
	if modelID == "" {
		modelID = defaultModelFor(cfg, providerName)
	}
	chatModel := provider.NewResolvingChatModel(modelHost, catalog, resolver)
	loopDriver, err := loopmodule.Compose(runtime.NewEngineFactory(chatModel))
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: construct LoopDriver: %w", err)
	}
	// CMP-2: optional cheaper compaction summary model, pinned to the
	// active provider's live spec (D9 single data source). Nil keeps the
	// main model as the summarizer. The value crosses into the engine as
	// the opaque runtime.SummaryModel seam (D-007: no direct eino import).
	var summaryModel runtime.SummaryModel
	if id := cfg.Runtime.Compaction.SummaryModel; id != "" {
		summaryModel = provider.NewOverrideModel(modelHost, catalog, resolver, id)
	}

	var workspaces runtime.WorkspaceAllocator
	var fileOps tools.FileOperations
	var fileBackend *runtime.EinoFilesystemBackend
	var fileRecorder tools.FileVersionRecorder
	var skillOps tools.SkillOperations
	var skillBackend *runtime.EinoSkillBackend
	var todoOps tools.TodoOperations
	var searchOps tools.SearchOperations
	var httpOps tools.HTTPOperations
	var fetchOps tools.WebFetchOperations
	var downloadOps tools.DownloadOperations
	var mcpOps tools.MCPOperations
	var sequentialOps tools.SequentialThinkingOperations
	var commandOps tools.CommandOperations
	var workspaceManager *runtime.WorkspaceManager
	var sandboxManager *runtime.SandboxManager
	if cfg.Runtime.WorkspaceRoot != "" {
		var manager *runtime.WorkspaceManager
		var err error
		if cfg.Runtime.World == "local" {
			manager, err = runtime.NewLocalWorkspaceManager(cfg.Runtime.WorkspaceRoot)
		} else {
			manager, err = runtime.NewWorkspaceManager(cfg.Runtime.WorkspaceRoot)
		}
		if err != nil {
			_ = backend.Close()
			return nil, fmt.Errorf("app: build workspace isolation: %w", err)
		}
		workspaces = manager
		workspaceManager = manager

		// Create SandboxManager with config (D-021)
		sandboxMode := domain.SandboxMode(cfg.Runtime.Sandbox.DefaultMode)
		if !sandboxMode.Valid() {
			sandboxMode = domain.SandboxModeWorkspaceWrite
		}
		sandboxRoot := cfg.Runtime.Sandbox.WorkspaceRoot
		if sandboxRoot == "" {
			sandboxRoot = cfg.Runtime.WorkspaceRoot
		}
		netPolicy := &domain.NetworkPolicy{
			AllowedDomains: cfg.Runtime.Sandbox.Network.AllowedDomains,
			DenyPrivateIPs: cfg.Runtime.Sandbox.Network.DenyPrivateIPs,
		}
		sandboxManager, err = runtime.NewSandboxManager(
			sandboxMode,
			sandboxRoot,
			cfg.Runtime.ExecuteAllowedCommands,
			netPolicy,
		)
		if err != nil {
			_ = backend.Close()
			return nil, fmt.Errorf("app: build sandbox manager: %w", err)
		}

		fileBackend = runtime.NewEinoFilesystemBackend(manager, sandboxManager)
		fileOps = fileBackend
		fileRecorder = runtime.NewFileVersionRecorder(backend, nil)
		fileBackend.SetFileVersionRecorder(fileRecorder)
	}
	if cfg.Runtime.SkillsRoot != "" {
		built, err := runtime.NewEinoSkillBackend(cfg.Runtime.SkillsRoot, backend)
		if err != nil {
			_ = backend.Close()
			return nil, fmt.Errorf("app: build skills backend: %w", err)
		}
		skillBackend = built
		skillOps = built
	}
	var marketplace tools.SkillsMarketplace
	if skillBackend != nil {
		built, err := runtime.NewMarketplaceService(skillBackend, cfg.Runtime.SkillsMarketplaceURL)
		if err != nil {
			_ = backend.Close()
			return nil, fmt.Errorf("app: build skills marketplace: %w", err)
		}
		marketplace = built
	}
	todoBackend := runtime.NewEinoTodoBackend(backend, filepath.Join(dataRoot, "todos"))
	todoOps = todoBackend
	searchService := runtime.NewNetworkSearchService(nil, nil)
	searchService.SetPreferredProvider(cfg.Tools.NetworkSearch.Provider)
	searchOps = searchService
	httpBackend := runtime.NewHTTPBackend(cfg.Runtime.HTTPAllowedHosts, cfg.Runtime.HTTPMaxResponseBytes, cfg.Runtime.HTTPTimeoutSeconds, sandboxManager)
	httpOps = httpBackend
	applyLiveHTTPSettings(httpBackend, liveSettingsPath, cfg)
	fetchOps = runtime.NewWebFetchBackend(cfg.Runtime.HTTPMaxResponseBytes, sandboxManager)
	if fileBackend != nil {
		downloadOps = runtime.NewDownloadBackend(fileBackend, sandboxManager)
	}
	var mcpBackend *runtime.MCPBackend
	if assemblyHasToolWorld(runtimeAssembly.Worlds, "mcp") {
		mcpBackend = runtime.NewMCPBackendWithOptions(mcpRuntimeConfigs(cfg.Runtime.MCPServers), nil, runtime.MCPBackendOptions{
			ProcessRoot: cfg.Runtime.WorkspaceRoot,
			Logger:      logger,
		})
	}
	mcpOwned := true
	defer func() {
		if mcpOwned && mcpBackend != nil {
			_ = mcpBackend.Close()
		}
	}()
	if mcpBackend != nil {
		mcpOps = mcpBackend
		// Replace the generated no-op MCP provider with the runtime bridge
		// before ToolWorld staging. The resulting provider is still discovered
		// by the sole ToolHost below; the compatibility MCP backend remains
		// control-plane-only.
		providers := append([]toolworldport.Provider(nil), runtimeAssembly.Worlds...)
		for index, provider := range providers {
			if provider != nil && provider.Definition().ID == "mcp" {
				providers[index] = mcpBackend.MCPToolWorldProvider()
			}
		}
		runtimeAssembly.Worlds = providers
	}
	sequentialOps = runtime.NewSequentialThinkingBackend()
	commandOps = runtime.NewCommandBackend(workspaceManager, sandboxManager, cfg.Runtime.ExecuteAllowedCommands, time.Duration(cfg.Runtime.ExecuteMaxTimeoutSeconds)*time.Second)
	var worldLookup worldWorkspaceLookup
	if workspaceManager != nil {
		worldLookup = func(ctx context.Context) (string, error) {
			workspace, err := workspaceManager.Ensure(ctx, tools.RunIDFromContext(ctx))
			return workspace.Path, err
		}
	}
	if fileBackend != nil && len(runtimeAssembly.DiagnosticObservers) > 0 {
		fileBackend.SetWriteDiagnostics(generatedWriteDiagnostics{
			observers: runtimeAssembly.DiagnosticObservers,
			worldIDs:  runtimeAssembly.DiagnosticObserverWorldIDs,
			grants:    runtimeAssembly.ToolWorldGrants,
			lookup:    worldLookup,
			recorder:  fileRecorder,
		})
	}
	// The builtin registry is built once and re-resolved per engine build:
	// Resolve filters by the active name list (settings tools_enabled
	// overlay when written, config default otherwise).
	// The agent tool's ops are armed after the worker manager exists
	// (the manager needs the registered tool set; the ref defers the bind).
	agentOps := &agentToolRef{}
	var builtinRegistry *tools.Registry
	var registryMu sync.RWMutex
	var liveApplyMu sync.Mutex
	rebuildToolRegistry := func(providers []toolworldport.Provider) error {
		staged, stageErr := bindToolWorlds(ctx, providers, runtimeAssembly.ToolWorldGrants, worldLookup, fileRecorder)
		if stageErr != nil {
			return stageErr
		}
		next := tools.BuiltinWithAgent(backend, fileOps, skillOps, todoOps, searchOps, httpOps, mcpOps, sequentialOps, commandOps, fetchOps, downloadOps, agentOps)
		next = next.WithAdditional(staged...)
		next, stageErr = bindGeneratedTools(runtimeAssembly.Tools, next)
		if stageErr != nil {
			return stageErr
		}
		registryMu.Lock()
		builtinRegistry = next
		registryMu.Unlock()
		return nil
	}
	if err := rebuildToolRegistry(runtimeAssembly.Worlds); err != nil {
		_ = backend.Close()
		return nil, err
	}
	appliedMCPConfigs := []runtime.MCPServerConfig(nil)
	if mcpBackend != nil {
		appliedMCPConfigs = mcpBackend.ConfiguredServers()
	}
	registryMu.Lock()
	appliedMCPConfigs = append([]runtime.MCPServerConfig(nil), appliedMCPConfigs...)
	registryMu.Unlock()
	// resolveActiveTools builds the live active surface plus its hidden
	// complement. It backs startup and every engine rebuild, so a
	// Settings-side active/hidden change lands without a process restart.
	// Generated ToolWorld tools stay appended to the active surface; only
	// builtins participate in the active/hidden split.
	resolveActiveTools := func() ([]tools.Tool, []tools.Tool, error) {
		registryMu.RLock()
		registry := builtinRegistry
		registryMu.RUnlock()
		if registry == nil {
			return nil, nil, errors.New("app: tool registry is unavailable")
		}
		enabled := cfg.Tools.Enabled
		if s, err := settings.Load(liveSettingsPath); err == nil && s.ToolsEnabled != nil {
			enabled = append([]string(nil), *s.ToolsEnabled...)
		}
		enabled = config.NormalizeLegacyToolSearch(enabled)
		// MCP capabilities are discovered from the live server, so a
		// settings/config selection may name a capability that disappears
		// when the server is replaced. Preserve the selected server intent,
		// drop retired projections, and include the replacement's current
		// projections before strict registry resolution. Static tool names
		// remain strict: an unknown non-MCP name is still an error.
		selectedMCPInstances := make(map[string]struct{})
		for _, name := range enabled {
			if instance, ok := mcpProjectedToolInstance(name); ok {
				selectedMCPInstances[instance] = struct{}{}
			}
		}
		filtered := make([]string, 0, len(enabled))
		for _, name := range enabled {
			if _, compiled := registry.Lookup(name); !compiled && (tools.IsAssemblyControlledTool(name) || name == "mcp_list_tools" || name == "mcp_call") {
				continue
			}
			if _, compiled := registry.Lookup(name); !compiled {
				if _, selected := mcpProjectedToolInstance(name); selected {
					continue
				}
			}
			filtered = append(filtered, name)
		}
		enabled = filtered
		if len(selectedMCPInstances) > 0 {
			selected := make(map[string]struct{}, len(enabled))
			for _, name := range enabled {
				selected[name] = struct{}{}
			}
			for _, spec := range registry.Specs() {
				instance, ok := mcpProjectedToolInstance(spec.Name)
				if !ok {
					continue
				}
				if _, wanted := selectedMCPInstances[instance]; !wanted {
					continue
				}
				if _, already := selected[spec.Name]; already {
					continue
				}
				enabled = append(enabled, spec.Name)
				selected[spec.Name] = struct{}{}
			}
		}
		resolved, err := registry.Resolve(enabled)
		if err != nil {
			return nil, nil, err
		}
		return resolved, registry.Except(enabled), nil
	}
	ts, hidden, err := resolveActiveTools()
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: resolve tools: %w", err)
	}
	// The checkpoint bridge fail-closes on its engine version, so an
	// unknown build version aborts startup rather than suspend runs on
	// unverifiable checkpoints (C6).
	engineVersion := runtime.EinoEngineVersion()
	if engineVersion == "" {
		_ = backend.Close()
		return nil, errors.New("app: eino engine version unavailable; checkpoint store cannot be anchored")
	}
	checkpointProvider, err := checkpointmodule.Compose(backend.Blobs(), engineVersion)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: build checkpoint store: %w", err)
	}
	checkpoints := checkpointProvider.Store()
	policy := policyEngine(cfg)
	userHooks := scriptHooksForConfig(cfg, func(format string, args ...any) {
		logger.Warn("hook registered but not approved", "detail", fmt.Sprintf(format, args...))
	})
	hooks := runtime.NewToolHookChain(cfg.Governance.HookTimeout, userHooks...)
	// Resolve the effective context-compression policy against the model's
	// context window (settings overlay already folded into cfg at startup).
	modelWindow := 0
	if info, infoErr := catalog.ResolveModelInfo(ctx, providerName, modelID); infoErr == nil {
		modelWindow = info.ContextWindow
	}
	cmp := compactionPolicyFor(cfg, nil, modelWindow)
	agentsMDBackend, agentsMDFiles, err := projectInstructionBackends(logger, ao.instructionRoot, skillBackend, fileBackend)
	if err != nil {
		_ = backend.Close()
		return nil, err
	}
	engineCfg := buildEngineConfig(cfg, skillBackend, agentsMDBackend, checkpoints, policy, hooks, &cmp, summaryModel, fileBackend)
	engineCfg.ContextHost, err = contextHostForAssembly(runtimeAssembly, mcpBackend)
	if err != nil {
		_ = backend.Close()
		return nil, err
	}
	engineCfg.SkillSources, err = generatedSkillSources(runtimeAssembly)
	if err != nil {
		_ = backend.Close()
		return nil, err
	}
	engineCfg.AgentsMDFiles = agentsMDFiles
	engineCfg.HiddenTools = hidden
	eng, err := loopDriver.Build(ctx, ts, engineCfg)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: build engine: %w", err)
	}

	bus := events.NewBus(cfg.Runtime.StreamBuffer)
	svcSink := runtime.EventSink(bus)
	if ao.sink != nil {
		svcSink = fanoutSink{primary: bus, extra: ao.sink}
	}
	// The ChannelHost is constructed before the runtime service so it can
	// join the initial hook list. It receives only a Run callback — never
	// *runtime.Service — so plugins cannot reach the runtime and the
	// channelhost layer stays free of internal/runtime imports. The
	// channels envelope may only name compiled-in channel plugins, and
	// every generated Channel Provider must carry the focused v1 Channel ABI (FR-10).
	// The headless face composes with no ears (WithoutEars).
	var svc *runtime.Service
	var channelHost *channelhost.Host
	if ao.channels {
		channelPlugins, err := bindChannels(runtimeAssembly.Channels, runtimeAssembly.ChannelGrants, cfg.Channels)
		if err != nil {
			_ = backend.Close()
			return nil, err
		}
		channelHost = channelhost.New(channelhost.Deps{
			Journal:  backend,
			Messages: backend,
			Sessions: backend,
			Run: func(ctx context.Context, sessionID domain.SessionID, text string, prov *domain.Provenance) (domain.RunID, error) {
				if svc == nil {
					return "", errors.New("app: runtime service is not wired")
				}
				return svc.RunWithOptions(ctx, sessionID, text, runtime.RunOptions{Provenance: prov})
			},
			Channels:    channelPlugins,
			Config:      cfg.Channels,
			Logger:      logger,
			Credentials: credentialResolver,
		})
	}
	runHooks := []runtime.RunHook{runtime.AuditHook{Sink: runtime.SlogAuditSink{Logger: logger}}}
	if channelHost != nil {
		runHooks = append(runHooks, channelHost)
	}
	svc = runtime.NewService(eng, providerName, modelID, runtime.ServiceDeps{
		Journal:            backend,
		Runs:               backend,
		Messages:           backend,
		Notes:              backend,
		Approvals:          backend,
		Questions:          backend,
		ApprovalExpiration: cfg.Tools.Approval.Expiration,
		ShellState:         backend.Blobs(),
		Budget: runtime.BudgetPolicy{
			MaxEvents: cfg.Runtime.MaxRunEvents, MaxModelCalls: cfg.Runtime.MaxModelCalls,
			MaxToolCalls: cfg.Runtime.MaxRunToolCalls, MaxRetries: cfg.Runtime.MaxRunRetries,
		},
		Workspaces:           workspaces,
		Sessions:             backend,
		PolicyDefaultProfile: domain.PolicyProfile(cfg.Governance.Profile),
		Hooks:                runHooks,
		Sink:                 svcSink,
		Compactions:          backend,
		Truncations:          backend,
		Crons:                backend,
		Channels:             channelHost,
		Titles:               provider.NewChainTitler(provider.TitleCandidates(modelHost, catalog, resolver, chatModel, cfg.Runtime.SmallModel)...),
		RebuildEngine: func(ctx context.Context, ec runtime.EngineConfig) (*runtime.Engine, error) {
			live, hidden, err := resolveActiveTools()
			if err != nil {
				return nil, fmt.Errorf("app: resolve live tools: %w", err)
			}
			ec.HiddenTools = hidden
			return loopDriver.Build(ctx, live, ec)
		},
	})
	svc.SetCatalog(catalog)
	// Worker children inherit the parent's effective log settings so their
	// per-worker file sink matches this process (LOGGING.md §3); an empty
	// handoff would leave child diagnostics invisible.
	effLog, err := logging.ResolveEffective(cfg.Logging.Level, cfg.Logging.Format)
	if err != nil {
		return nil, fmt.Errorf("app: resolve worker log settings: %w", err)
	}
	workerManager := newWorkerManager(svc, backend, backend, policy, hooks, ts, cfg.Runtime.MaxToolResultBytes, cfg.Tools.Approval.Expiration, chatModel, worker.WorkerLog{Dir: cfg.LogDirectory(), Level: effLog.Level, Format: effLog.Format})
	agentOps.arm(workerManager)
	svc.SetChildApprovalRouter(workerManager)
	svc.SetChildRunCanceller(workerManager)

	liveProfile := domain.PolicyProfile(cfg.Governance.Profile)
	if !liveProfile.Valid() {
		liveProfile = domain.PolicyProfileDefault
	}

	// The action host is enabled only when the compiler emitted action
	// ProviderSets and the process can prove the exact sealed Generation. An
	// empty or unverifiable inventory is a disabled capability, never an
	// implicit default-allow host.
	rpcToken := controlrpc.NewSessionToken()
	generationID := runtimeGenerationID(runtimeAssembly)
	var actionHost *actionhost.Host
	actionHostOwned := false
	if len(runtimeAssembly.ActionSets) > 0 && generationID != "" {
		allowedTools := make(map[string]struct{}, len(ts))
		for _, candidate := range ts {
			if candidate != nil {
				allowedTools[candidate.Spec().Name] = struct{}{}
			}
		}
		authenticateAction := func(ctx context.Context, caller actionhost.Caller) (actionhost.Identity, error) {
			if caller.Opaque() == "" || subtle.ConstantTimeCompare([]byte(caller.Opaque()), []byte(rpcToken)) != 1 {
				return actionhost.Identity{}, actionport.ErrUnauthenticated
			}
			// The HTTP/RPC authentication middleware must attach this identity;
			// a browser-supplied session or face field is never accepted here.
			identity, ok := actionhost.IdentityFromContext(ctx)
			if !ok {
				return actionhost.Identity{}, actionport.ErrUnauthenticated
			}
			return identity, nil
		}
		authorizeAction := func(_ context.Context, _ actionhost.Identity, definition actionport.Definition, input json.RawMessage) error {
			if policy == nil {
				return actionport.ErrAuthorizationUnavailable
			}
			spec := actionToolSpec(definition)
			evaluation, evalErr := policy.Evaluate(liveProfile, spec, input)
			if evalErr != nil {
				return actionport.ErrAuthorizationUnavailable
			}
			if evaluation.Decision != domain.PolicyAllow {
				if evaluation.Decision == domain.PolicyPrompt || definition.RequiresApproval || definition.ApprovalRequired {
					return actionport.ErrApprovalRequired
				}
				return runtime.ErrPolicyDenied
			}
			// This composition has no action-specific approval row/continuation
			// route. Requiring one is safer than treating a provider claim as an
			// approval; ordinary full-auto policy remains an explicit authority.
			if definition.RequiresApproval || definition.ApprovalRequired {
				return actionport.ErrApprovalRequired
			}
			return nil
		}
		authorizeBridge := func(ctx context.Context, identity actionhost.Identity, request actionhost.BridgeRequest) error {
			if identity.SessionID == "" {
				return actionport.ErrUnauthenticated
			}
			if request.Kind == actionhost.BridgeRun {
				if request.Run.SessionID != identity.SessionID {
					return actionport.ErrUnauthenticated
				}
			} else if request.Kind != actionhost.BridgeTool {
				return actionport.ErrAuthorizationUnavailable
			}
			return authorizeAction(ctx, identity, request.Action, request.Input)
		}
		actionHost, err = actionhost.New(actionhost.Deps{
			ProviderSets:        runtimeAssembly.ActionSets,
			GenerationAvailable: true,
			GenerationID:        generationID,
			Audit:               actionhost.JournalAuditSink{Journal: backend, Logger: logger},
			Authenticate:        authenticateAction,
			Authorize:           authorizeAction,
			AuthorizeBridge:     authorizeBridge,
			AllowedTools:        allowedTools,
			StartRun: func(ctx context.Context, request actionport.RunRequest) (actionport.RunResult, error) {
				if svc == nil {
					return actionport.RunResult{}, actionport.ErrRunDenied
				}
				identity, ok := actionhost.IdentityFromContext(ctx)
				if !ok || identity.SessionID == "" || strings.TrimSpace(request.SessionID) != identity.SessionID {
					return actionport.RunResult{}, actionport.ErrUnauthenticated
				}
				id, runErr := svc.Run(ctx, domain.SessionID(identity.SessionID), request.Text)
				if runErr != nil {
					return actionport.RunResult{}, actionport.ErrRunDenied
				}
				return actionport.RunResult{ID: string(id), Status: "accepted"}, nil
			},
			InvokeTool: func(ctx context.Context, id string, input json.RawMessage) (json.RawMessage, error) {
				if svc == nil {
					return nil, actionport.ErrToolDenied
				}
				identity, ok := actionhost.IdentityFromContext(ctx)
				if !ok || identity.SessionID == "" || identity.RunID == "" {
					return nil, actionport.ErrUnauthenticated
				}
				result, invokeErr := svc.InvokeActionTool(ctx, domain.SessionID(identity.SessionID), domain.RunID(identity.RunID), id, input)
				if invokeErr != nil {
					return nil, actionport.ErrToolDenied
				}
				return json.Marshal(result)
			},
		})
		if err != nil {
			_ = backend.Close()
			return nil, fmt.Errorf("app: build action host: %w", err)
		}
		actionHostOwned = true
	} else if len(runtimeAssembly.ActionSets) > 0 {
		logger.Warn("control actions disabled: sealed Generation identity unavailable")
	}
	defer func() {
		if actionHostOwned {
			_ = actionHost.CloseContext(context.Background())
		}
	}()

	liveSnap, err := policy.Snapshot(liveProfile)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: snapshot default policy: %w", err)
	}
	liveTools := make([]domain.ToolSpec, 0, len(ts))
	for _, tool := range ts {
		liveTools = append(liveTools, tool.Spec())
	}
	// appliedToolsEnabled tracks the active surface the running engine was
	// built with, so a settings save can detect an active/hidden change.
	appliedToolsEnabled := append([]string(nil), cfg.Tools.Enabled...)
	studioSvc := studio.NewService(backend)
	executable, exeErr := os.Executable()
	if exeErr != nil {
		executable = ""
	}
	bundleDir := cfg.Providers.BundleDir
	if abs, err := filepath.Abs(bundleDir); err == nil {
		bundleDir = abs
	}
	evalRunner := eval.NewRunner(eval.Runner{
		Studio:     studioSvc,
		Executable: executable,
		EvalRoot:   filepath.Join(dataRoot, "evals"),
		Isolation: eval.Isolation{
			ProductionSQLite:    cfg.Storage.SQLite.Path,
			ProductionWorkspace: cfg.Runtime.WorkspaceRoot,
			ProductionListen:    cfg.Server.Addr,
			BundleDir:           bundleDir,
		},
	})
	fileVersions, _ := backend.(storage.ModifiedFileStore)
	mcpCompiled := assemblyHasToolWorld(runtimeAssembly.Worlds, "mcp")
	contextCompiled := assemblyHasModule(runtimeAssembly.Manifest.Modules, "vivy/context-host")
	controlHandler, err := controlrpc.NewControlHandler(controlrpc.ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Reviews: backend, Todos: backend, Skills: skillOps, Bus: bus, Service: svc,
		ActionHost:     actionHost,
		Marketplace:    marketplace,
		SkillRevisions: backend,
		Compactions:    backend,
		Truncations:    backend,
		Crons:          backend, CronRunner: svc,
		Studio: studioSvc,
		Live: studio.LiveView{
			Provider:      providerName,
			PolicyProfile: liveProfile,
			PolicyHash:    liveSnap.Hash,
			Tools:         liveTools,
		},
		Eval:             evalRunner,
		Children:         workerManager,
		SettingsPath:     liveSettingsPath,
		GenerationLocale: presentation.DefaultLocale,
		DeveloperLocale:  developerLocale,
		SealedGeneration: presentation.SealedGeneration,
		ConfigProvider:   configProvider,
		ConfigModel:      configModel,
		ProviderBundles:  []provider.Bundle{openaiBundle, anthropicBundle},
		ProviderProfileStatuses: func() []modelhost.ProfileStatus {
			current := resolver.Current()
			return modelHost.Statuses(current.Provider, current.Ready)
		},
		RuntimeBaseURL:                 cur.BaseURL,
		ConfigNetworkSearchProvider:    cfg.Tools.NetworkSearch.Provider,
		ConfigExecuteMaxTimeoutSeconds: cfg.Runtime.ExecuteMaxTimeoutSeconds,
		DefaultPermissionPreset:        defaultPermissionPreset(cfg),
		SandboxWorkspaceRoot:           cfg.Runtime.WorkspaceRoot,
		ExecuteAllowedCommands:         append([]string(nil), cfg.Runtime.ExecuteAllowedCommands...),
		ConfigSandboxDenyPrivateIPs:    cfg.Runtime.Sandbox.Network.DenyPrivateIPs,
		ConfigSandboxAllowedDomains:    append([]string(nil), cfg.Runtime.Sandbox.Network.AllowedDomains...),
		ConfigHTTPAllowedHosts:         append([]string(nil), cfg.Runtime.HTTPAllowedHosts...),
		ConfigHTTPTimeoutSeconds:       cfg.Runtime.HTTPTimeoutSeconds,
		ConfigCompaction:               cmp,
		// Channel ears: the Host exposes the compiled-in set and the process
		// truth of the last StartAll; channel writes go through the settings
		// overlay and apply on the next restart (contract §11).
		Channels:       channelHost,
		ConfigChannels: cfg.Channels,
		// Settings→Tools surface: the full builtin catalog (active and
		// hidden) plus the effective config default active set.
		ToolCatalog:        builtinRegistry.Specs(),
		ConfigToolsEnabled: append([]string(nil), cfg.Tools.Enabled...),
		// Write-time env apply: a settings/providers save updates the
		// running process environment (base_url → VIVY_API_BASE, resolved
		// api_key → active bundle env_key) immediately; the startup overlay
		// replays the same document on the next launch.
		ApplySettingsEnv: func(s settings.Settings) { applySettingsEnv(logger, cfg, s) },
		TokenUsage:       backend,
		FileVersions:     fileVersions,
		// Model metadata rides the same provider catalog the runtime and
		// compaction use (D9: no separate data source). Resolve failures
		// mean unpriced/unknown, which the cost math reports as such.
		ModelMeta: func(ctx context.Context, providerName, modelID string) domain.ModelInfo {
			info, err := catalog.ResolveModelInfo(ctx, providerName, modelID)
			if err != nil {
				return domain.ModelInfo{}
			}
			return info
		},
		MCP:             mcpBackend,
		MCPCompiled:     &mcpCompiled,
		ContextCompiled: &contextCompiled,
		LanguageServers: buildLanguageServerStatusSource(runtimeAssembly.LanguageServerStatuses, backend, workspaceManager),
		WorkspaceFiles: func() controlrpc.WorkspaceFiles {
			if workspaceManager == nil {
				return nil
			}
			return runtime.NewWorkspaceFiles(workspaceManager, 0)
		}(),
		// Only vivy-code supplies this explicit seam. Runtime.WorkspaceRoot is
		// a tenant/sandbox setting for ordinary Vivy compositions and must not
		// implicitly become an attachment project root.
		ProjectRoot: ao.projectRoot,
		Frozen:      resolver.Frozen(),
		ToolCatalogLive: func() []domain.ToolSpec {
			registryMu.RLock()
			registry := builtinRegistry
			registryMu.RUnlock()
			if registry == nil {
				return nil
			}
			return registry.Specs()
		},
		OnSettingsChanged: func() {
			liveApplyMu.Lock()
			defer liveApplyMu.Unlock()
			resolver.Invalidate()
			live := resolver.Current()
			name := live.Provider
			if name == "" {
				name = cfg.Providers.Active
			}
			id := live.Model
			if id == "" {
				id = defaultModelFor(cfg, name)
			}
			svc.SetModel(name, id)
			applyLiveSandboxSettings(sandboxManager, liveSettingsPath, cfg)
			applyLiveHTTPSettings(httpBackend, liveSettingsPath, cfg)
			s, err := settings.Load(liveSettingsPath)
			if err != nil {
				logger.Warn("mcp overlay reload skipped", "err", err)
				return
			}
			mcpChanged := false
			if mcpBackend != nil {
				nextMCPConfigs := liveMCPConfigs(cfg, s)
				registryMu.RLock()
				previousMCPConfigs := append([]runtime.MCPServerConfig(nil), appliedMCPConfigs...)
				registryMu.RUnlock()
				mcpChanged = !sameMCPRuntimeConfigs(nextMCPConfigs, previousMCPConfigs)
				if mcpChanged {
					mcpBackend.ReplaceServers(nextMCPConfigs)
					currentMCPConfigs := mcpBackend.ConfiguredServers()
					registryMu.Lock()
					appliedMCPConfigs = append([]runtime.MCPServerConfig(nil), currentMCPConfigs...)
					registryMu.Unlock()
					if rebuildErr := rebuildToolRegistry(runtimeAssembly.Worlds); rebuildErr != nil {
						// A removed or replaced MCP instance must not remain in
						// the model catalog when discovery of its replacement
						// fails. Keep other generated worlds live, but fail closed
						// for MCP until the next successful settings rebuild.
						logger.Warn("MCP tool registry rebuild failed", "err", rebuildErr)
						withoutMCP := make([]toolworldport.Provider, 0, len(runtimeAssembly.Worlds))
						for _, provider := range runtimeAssembly.Worlds {
							if provider != nil && provider.Definition().ID == "mcp" {
								continue
							}
							withoutMCP = append(withoutMCP, provider)
						}
						if fallbackErr := rebuildToolRegistry(withoutMCP); fallbackErr != nil {
							logger.Warn("MCP tool registry fail-closed rebuild failed", "err", fallbackErr)
						}
					}
				}
			}
			// Tools live-apply: an active/hidden change rebuilds the engine
			// so the next run binds the new surface (immediately when idle,
			// otherwise at the next idle run start).
			mergedTools := mergedToolsEnabled(cfg, s)
			registryMu.RLock()
			previousToolsEnabled := append([]string(nil), appliedToolsEnabled...)
			registryMu.RUnlock()
			toolsChanged := !sameStrings(mergedTools, previousToolsEnabled)
			if toolsChanged {
				registryMu.Lock()
				appliedToolsEnabled = mergedTools
				registryMu.Unlock()
			}
			// Compaction live-apply: rebuild the engine (reduction +
			// summarization middleware) only when the effective policy
			// changed. The rebuild lands immediately when idle, otherwise
			// at the next idle run start.
			window := svc.GetModelInfo(context.Background()).ContextWindow
			cmp := compactionPolicyFor(cfg, s.Compaction, window)
			compactionChanged := !sameCompactionPolicy(svc.CompactionPolicy(), &cmp)
			if toolsChanged || mcpChanged || compactionChanged {
				reloadCfg := buildEngineConfig(cfg, skillBackend, agentsMDBackend, checkpoints, policy, hooks, &cmp, summaryModel, fileBackend)
				var reloadErr error
				reloadCfg.ContextHost, reloadErr = contextHostForAssembly(runtimeAssembly, mcpBackend)
				if reloadErr != nil {
					logger.Warn("MCP context bridge reload skipped", "err", reloadErr)
					reloadCfg.ContextHost = nil
				}
				reloadCfg.SkillSources, reloadErr = generatedSkillSources(runtimeAssembly)
				if reloadErr != nil {
					logger.Warn("generated SkillSource reload skipped", "err", reloadErr)
					reloadCfg.SkillSources = nil
				}
				reloadCfg.AgentsMDFiles = agentsMDFiles
				if err := svc.ScheduleEngineReload(reloadCfg); err != nil {
					logger.Warn("engine reload failed", "err", err)
				}
			}
		},
	})
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: build rpc control plane: %w", err)
	}
	// Restart recovery before the server listens (E2, FR-8): every
	// non-terminal run either re-registers on its pending approval or
	// closes with a definitive run.failed. A listing failure means the
	// storage truth is unreachable; startup aborts.
	if err := svc.Recover(ctx); err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: restart recovery: %w", err)
	}
	// Start the channel ears before the server listens (C3). Unconfigured
	// and disabled channels are skipped; empty allow_from refuses Start
	// for that channel. A wiring failure here aborts startup. The headless
	// face has no ears.
	if channelHost != nil {
		if err := channelHost.StartAll(ctx); err != nil {
			_ = backend.Close()
			return nil, fmt.Errorf("app: start channels: %w", err)
		}
	}

	app := &App{
		cfg:        cfg,
		logger:     logger,
		service:    svc,
		channels:   channelHost,
		actionHost: actionHost,
		backend:    backend,
		worker:     workerManager,
		resolver:   resolver,
		modelHost:  modelHost,
		control:    controlHandler,
		rpcToken:   rpcToken,
		mcpBackend: mcpBackend,
		assembly:   &runtimeAssembly,
	}
	mcpOwned = false
	actionHostOwned = false
	assemblyOwned = false
	// The gateway is faces/web's effect: the mux, the embedded UI shell and
	// the loopback listener exist only in the gateway assembly (face-pack
	// §3). A gateway-less generation reaches the identical control plane
	// through DialControl instead.
	if !ao.gateway {
		return app, nil
	}

	mux := http.NewServeMux()
	mux.Handle("/rpc", controlrpc.WebSocketServer{Handler: controlHandler, Token: rpcToken, Origins: originPolicy})
	mux.HandleFunc("/rpc/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !originPolicy.Allows(r) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		originPolicy.ApplyCORS(w, r)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"protocol_version":%q,"websocket_path":"/rpc","token":%q}`, controlrpc.ProtocolVersion, rpcToken)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","stage":"e2-recovery"}`))
	})
	// The default build serves the embedded UI; the vivy_headless build
	// supplies a 404 handler while retaining the same control plane.
	mux.Handle("/", ui.Handler())

	app.httpServer = &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           controlrpc.AccessLogMiddleware(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return app, nil
}

// Close shuts down a gateway-less composition and is safe to call more than
// once. Resident gateway processes use Run, whose shutdown additionally owns
// the HTTP listener ordering.
func (a *App) Close() error {
	if a == nil {
		return nil
	}
	a.closeOnce.Do(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if a.actionHost != nil {
			a.closeErr = errors.Join(a.closeErr, a.actionHost.Close())
		}
		if a.channels != nil {
			a.channels.StopAll(shutdownCtx)
		}
		if a.service != nil {
			a.service.StopInteractionSweeper()
			a.service.StopCronScheduler()
			a.service.CancelAll()
		}
		if a.worker != nil {
			a.closeErr = errors.Join(a.closeErr, a.worker.Close(shutdownCtx))
		}
		if a.service != nil && !a.service.WaitIdle(shutdownCtx) {
			a.closeErr = errors.Join(a.closeErr, shutdownCtx.Err())
		}
		if a.mcpBackend != nil {
			a.closeErr = errors.Join(a.closeErr, a.mcpBackend.Close())
		}
		if a.assembly != nil {
			a.closeErr = errors.Join(a.closeErr, closeToolWorlds(shutdownCtx, a.assembly.Worlds), a.assembly.Close(shutdownCtx))
		}
		if a.backend != nil {
			a.closeErr = errors.Join(a.closeErr, a.backend.Close())
		}
	})
	return a.closeErr
}

// ActionHost returns the process-owned typed Control Action host. Callers
// still need an authenticated Caller; this method does not expose provider
// registration or any arbitrary RPC route.
func (a *App) ActionHost() *actionhost.Host {
	if a == nil {
		return nil
	}
	return a.actionHost
}

func policyEngine(cfg config.Config) *runtime.PolicyEngine {
	definitions := make(map[domain.PolicyProfile]runtime.PolicyDefinition, len(cfg.Governance.Profiles))
	for name, profile := range cfg.Governance.Profiles {
		rules := make([]runtime.PolicyRule, 0, len(profile.Rules))
		for _, rule := range profile.Rules {
			rules = append(rules, runtime.PolicyRule{
				Tool: rule.Tool, Field: rule.Field, Equals: rule.Equals, Prefix: rule.Prefix,
				Decision: domain.PolicyDecision(rule.Decision), Reason: rule.Reason,
			})
		}
		definitions[domain.PolicyProfile(name)] = runtime.PolicyDefinition{
			Default: domain.PolicyDecision(profile.Default), Rules: rules,
		}
	}
	engine, err := runtime.NewPolicyEngine(definitions)
	if err != nil {
		// Config.Validate already rejects invalid policy definitions. Keep
		// composition fail-safe if a direct test bypasses that boundary.
		panic(fmt.Sprintf("app: invalid governance policy: %v", err))
	}
	return engine
}

func actionToolSpec(definition actionport.Definition) domain.ToolSpec {
	return domain.ToolSpec{
		Name:        definition.ID,
		Description: definition.Description,
		Readonly:    definition.Effect == actionport.EffectRead,
	}
}

// runtimeGenerationID accepts an explicitly injected identity in tests and
// generated compositions, then falls back to the linker-embedded sealed
// manifest used by packed binaries. It intentionally never invents an ID
// from mutable runtime state.
func runtimeGenerationID(runtimeAssembly genassembly.RuntimeAssembly) string {
	if id := strings.TrimSpace(runtimeAssembly.GenerationID); id != "" {
		return id
	}
	raw, err := generation.EmbeddedManifest()
	if err != nil {
		return ""
	}
	id, _, err := generation.InspectManifestProvenance(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(id)
}

// applySettingsEnv applies the non-secret base URL and the active bundle
// api_key overlays to the process environment. It is shared between the
// startup overlay (next-launch semantics) and the write-time path, so a
// settings save updates the environment immediately AND the next start
// replays the same document. Secret values are never logged.
func applySettingsEnv(logger *slog.Logger, cfg config.Config, s settings.Settings) {
	if s.BaseURL != "" {
		if err := os.Setenv(provider.APIBaseEnvVar, s.BaseURL); err != nil {
			logger.Warn("settings base_url not applied", "err", err)
		}
	}
	keyEnv := ""
	switch s.Provider {
	case settings.ProviderOpenAI:
		keyEnv = cfg.Providers.OpenAI.EnvKey
	case settings.ProviderAnthropic:
		keyEnv = cfg.Providers.Anthropic.EnvKey
	}
	if keyEnv != "" {
		if key := settings.ActiveKey(s, s.Provider, s.BaseURL); key != "" {
			if err := os.Setenv(keyEnv, key); err != nil {
				logger.Warn("settings api_key not applied", "err", err)
			}
		}
	}
}

// applySettingsOverlay reads the operator-managed settings document and
// overlays non-secret preferences onto cfg. Provider keys stay in the
// settings document (or a frozen ENV session) and are resolved per call.
// compiledChannels holds the channel plugin names of this generation: the
// per-channel overlay may only name those, and stale overlay entries for
// channels that are no longer compiled-in are dropped (with a warning)
// instead of failing startup.
func applySettingsOverlay(ctx context.Context, logger *slog.Logger, cfg config.Config, compiledChannels []string) config.Config {
	return applySettingsOverlayAt(ctx, logger, cfg, settings.Path(cfg.DataDirectory()), compiledChannels)
}

func applySettingsOverlayAt(ctx context.Context, logger *slog.Logger, cfg config.Config, path string, compiledChannels []string) config.Config {
	cfg.Tools.Enabled = config.NormalizeLegacyToolSearch(cfg.Tools.Enabled)
	s, err := settings.Load(path)
	if err != nil {
		logger.Warn("settings overlay skipped", "path", path, "err", err)
		return cfg
	}
	if s.IsZero() {
		return cfg
	}
	if s.Provider != "" {
		cfg.Providers.Active = s.Provider
		switch s.Provider {
		case settings.ProviderOpenAI:
			if s.DefaultModel != "" {
				cfg.Providers.OpenAI.DefaultModel = s.DefaultModel
			}
		case settings.ProviderAnthropic:
			if s.DefaultModel != "" {
				cfg.Providers.Anthropic.DefaultModel = s.DefaultModel
			}
		}
	}
	if s.NetworkSearch.Provider != "" {
		cfg.Tools.NetworkSearch.Provider = s.NetworkSearch.Provider
	}
	if s.ExecuteMaxTimeoutSeconds > 0 {
		cfg.Runtime.ExecuteMaxTimeoutSeconds = s.ExecuteMaxTimeoutSeconds
	}
	if overlay := enabledMCPFromSettings(s); overlay != nil {
		cfg.Runtime.MCPServers = overlay
	}
	if s.ToolsEnabled != nil {
		cfg.Tools.Enabled = config.NormalizeLegacyToolSearch(*s.ToolsEnabled)
	}
	if s.Sandbox.DefaultPreset.ValidSwitch() {
		mode, policy, ok := s.Sandbox.DefaultPreset.Bundle()
		if ok {
			cfg.Runtime.Sandbox.DefaultMode = string(mode)
			cfg.Runtime.Sandbox.Approval.DefaultPolicy = string(policy)
		}
	}
	if s.Sandbox.Network.DenyPrivateIPs != nil {
		cfg.Runtime.Sandbox.Network.DenyPrivateIPs = *s.Sandbox.Network.DenyPrivateIPs
	}
	if s.Sandbox.Network.AllowedDomains != nil {
		cfg.Runtime.Sandbox.Network.AllowedDomains = append([]string(nil), s.Sandbox.Network.AllowedDomains...)
	}
	cfg.Runtime.Compaction = mergedCompactionConfig(cfg.Runtime.Compaction, s.Compaction)
	if merged := mergedChannels(logger, cfg.Channels, s.Channels, compiledChannels); merged != nil {
		cfg.Channels = merged
	}
	logger.Info("settings overlay applied", "provider", cfg.Providers.Active, "model", s.DefaultModel, "network_search_provider", cfg.Tools.NetworkSearch.Provider, "execute_max_timeout_seconds", cfg.Runtime.ExecuteMaxTimeoutSeconds, "sandbox_preset", s.Sandbox.DefaultPreset, "mcp_servers", len(cfg.Runtime.MCPServers), "compaction_enabled", cfg.Runtime.Compaction.Enabled, "channels_overlayed", len(s.Channels))
	return cfg
}

// mergedChannels overlays the settings.yaml per-channel entries onto the
// config.yaml channels envelopes. Each entry replaces only the fields it
// carries (pointer semantics); the config envelope's opaque per-plugin
// Settings yaml.Node always survives. An entry for a compiled-in channel
// that config.yaml has no envelope for starts a fresh envelope — that is
// how the UI configures a channel this generation without touching
// config.yaml. Entries naming channels that are not compiled-in are
// dropped with a warning so a stale overlay can never fail startup. A nil
// result (no overlay entries) keeps the config envelopes untouched.
func mergedChannels(logger *slog.Logger, base config.Channels, overlays []settings.ChannelOverlay, compiled []string) config.Channels {
	if len(overlays) == 0 {
		return nil
	}
	compiledSet := make(map[string]bool, len(compiled))
	for _, name := range compiled {
		compiledSet[name] = true
	}
	out := make(config.Channels, len(base)+len(overlays))
	for name, envelope := range base {
		out[name] = envelope
	}
	for _, overlay := range overlays {
		if !compiledSet[overlay.Name] {
			logger.Warn("settings overlay names a channel that is not compiled in; entry dropped", "channel", overlay.Name)
			continue
		}
		envelope := out[overlay.Name]
		if overlay.Enabled != nil {
			envelope.Enabled = *overlay.Enabled
		}
		if overlay.AllowFrom != nil {
			envelope.AllowFrom = append([]string(nil), *overlay.AllowFrom...)
		}
		if overlay.TokenEnv != nil {
			envelope.TokenEnv = *overlay.TokenEnv
		}
		out[overlay.Name] = envelope
	}
	return out
}

// mergedToolsEnabled returns the effective active tool names: the settings
// tools_enabled overlay when written, else the (already startup-overlaid)
// config default. The result is a copy; order is preserved because the
// surface order is a product contract.
func mergedToolsEnabled(cfg config.Config, s settings.Settings) []string {
	if s.ToolsEnabled != nil {
		return config.NormalizeLegacyToolSearch(*s.ToolsEnabled)
	}
	return config.NormalizeLegacyToolSearch(cfg.Tools.Enabled)
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameMCPRuntimeConfigs(left, right []runtime.MCPServerConfig) bool {
	if len(left) != len(right) {
		return false
	}
	leftByName := make(map[string]runtime.MCPServerConfig, len(left))
	rightByName := make(map[string]runtime.MCPServerConfig, len(right))
	for _, config := range left {
		leftByName[config.Name] = config
	}
	for _, config := range right {
		rightByName[config.Name] = config
	}
	return reflect.DeepEqual(leftByName, rightByName)
}

func mcpProjectedToolInstance(name string) (string, bool) {
	parts := strings.SplitN(name, ".", 3)
	if len(parts) != 3 || parts[0] != "mcp" || parts[1] == "" || parts[2] == "" {
		return "", false
	}
	return parts[1], true
}

// enabledMCPFromSettings returns the MCP servers from the overlay, preserving
// disabled entries as inactive runtime instances. A nil overlay means the
// config-file catalog remains authoritative.
func enabledMCPFromSettings(s settings.Settings) []config.MCPServer {
	if s.MCPServers == nil {
		return nil
	}
	out := make([]config.MCPServer, 0, len(*s.MCPServers))
	for _, server := range *s.MCPServers {
		out = append(out, config.MCPServer{
			Name: server.Name, Endpoint: server.Endpoint, Command: server.Command,
			Args: append([]string(nil), server.Args...), EnvFrom: cloneMCPEnvFrom(server.EnvFrom),
			Cwd: server.Cwd, AuthEnv: server.AuthEnv, ResourceBridge: server.ResourceBridge,
			DeferredReason: server.DeferredReason, Enabled: cloneBoolPtr(server.Enabled),
		})
	}
	return out
}

func mcpRuntimeConfigs(servers []config.MCPServer) []runtime.MCPServerConfig {
	out := make([]runtime.MCPServerConfig, 0, len(servers))
	for _, server := range servers {
		out = append(out, runtime.MCPServerConfig{
			Name: server.Name, Endpoint: server.Endpoint, Command: server.Command,
			Args: append([]string(nil), server.Args...), EnvFrom: cloneMCPEnvFrom(server.EnvFrom),
			Cwd: server.Cwd, AuthEnv: server.AuthEnv, ResourceBridge: server.ResourceBridge,
			DeferredReason: server.DeferredReason, Enabled: cloneBoolPtr(server.Enabled),
		})
	}
	return out
}

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneMCPEnvFrom(value map[string]string) map[string]string {
	if value == nil {
		return nil
	}
	out := make(map[string]string, len(value))
	for child, host := range value {
		out[child] = host
	}
	return out
}

func liveMCPConfigs(cfg config.Config, s settings.Settings) []runtime.MCPServerConfig {
	if overlay := enabledMCPFromSettings(s); overlay != nil {
		return mcpRuntimeConfigs(overlay)
	}
	return mcpRuntimeConfigs(cfg.Runtime.MCPServers)
}

func defaultPermissionPreset(cfg config.Config) domain.PermissionPreset {
	mode := domain.SandboxMode(cfg.Runtime.Sandbox.DefaultMode)
	if !mode.Valid() {
		mode = domain.SandboxModeWorkspaceWrite
	}
	policy := domain.ApprovalPolicy(cfg.Runtime.Sandbox.Approval.DefaultPolicy)
	if !policy.Valid() {
		policy = domain.ApprovalPolicyAsk
	}
	return domain.PermissionPresetOf(mode, policy)
}

func applyLiveSandboxSettings(manager *runtime.SandboxManager, path string, cfg config.Config) {
	if manager == nil {
		return
	}
	s, err := settings.Load(path)
	if err != nil {
		return
	}
	if s.Sandbox.DefaultPreset.ValidSwitch() {
		mode, _, ok := s.Sandbox.DefaultPreset.Bundle()
		if ok {
			manager.SetDefaultMode(mode)
		}
	}
	denyPrivate := cfg.Runtime.Sandbox.Network.DenyPrivateIPs
	allowed := append([]string(nil), cfg.Runtime.Sandbox.Network.AllowedDomains...)
	if s.Sandbox.Network.DenyPrivateIPs != nil {
		denyPrivate = *s.Sandbox.Network.DenyPrivateIPs
	}
	if s.Sandbox.Network.AllowedDomains != nil {
		allowed = append([]string(nil), s.Sandbox.Network.AllowedDomains...)
	}
	manager.SetNetworkPolicy(domain.NetworkPolicy{AllowedDomains: allowed, DenyPrivateIPs: denyPrivate})
}

// applyLiveHTTPSettings merges the settings.yaml http overlay over the
// config defaults and live-applies the result to the running HTTP backend
// (allowlist + request timeout). The startup path replays the same document.
func applyLiveHTTPSettings(backend *runtime.HTTPBackend, path string, cfg config.Config) {
	if backend == nil {
		return
	}
	s, err := settings.Load(path)
	if err != nil {
		return
	}
	hosts := append([]string(nil), cfg.Runtime.HTTPAllowedHosts...)
	timeout := cfg.Runtime.HTTPTimeoutSeconds
	if s.HTTP != nil {
		if s.HTTP.AllowedHosts != nil {
			hosts = append([]string(nil), *s.HTTP.AllowedHosts...)
		}
		if s.HTTP.TimeoutSeconds != 0 {
			timeout = s.HTTP.TimeoutSeconds
		}
	}
	backend.SetConfig(hosts, timeout)
}

func defaultModelFor(cfg config.Config, providerName string) string {
	switch providerName {
	case "anthropic":
		return cfg.Providers.Anthropic.DefaultModel
	default:
		return cfg.Providers.OpenAI.DefaultModel
	}
}

func providerConfigBaseline(cfg config.Config) (string, string) {
	providerName := cfg.Providers.Active
	return providerName, defaultModelFor(cfg, providerName)
}

// Run blocks until ctx is cancelled or the server fails. On cancellation
// it shuts down components in reverse startup order with a bounded grace
// period and returns the shutdown error, if any. A gateway-less assembly
// has no listener: it blocks on ctx alone while faces drive the in-process
// control plane, then takes the same reverse-order shutdown.
func (a *App) Run(ctx context.Context) error {
	a.service.StartInteractionSweeper(context.Background(), time.Second)
	defer a.service.StopInteractionSweeper()
	// Cron fires agent turns on a schedule; it must not outlive the
	// server loop and its in-flight watchers drain during shutdown below.
	if a.cfg.Runtime.Cron.Enabled {
		a.service.StartCronScheduler(context.Background(), runtime.CronSchedulerOptions{})
		defer a.service.StopCronScheduler()
	}
	// The listener is faces/web's effect (VIVY-FACE-PACK §7); a
	// gateway-less assembly has no server error source to wait on.
	errCh := make(chan error, 1)
	if a.httpServer != nil {
		go func() {
			a.logger.Info("vivy starting", "addr", a.cfg.Server.Addr)
			if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("http server: %w", err)
			}
			close(errCh)
		}()
	} else {
		close(errCh)
	}

	select {
	case err := <-errCh:
		if err == nil {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	a.logger.Info("vivy shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	// Reverse startup order with hard ordering guarantees (E4): channels
	// stop first so an adapter's Stop never races a cancelled run's final
	// delivery; runs are then cancelled and drained while storage is still
	// open, so every run.cancelled terminal persists before the journal
	// closes; only then do the HTTP server and the backend shut down.
	a.service.StopInteractionSweeper()
	if a.channels != nil {
		a.channels.StopAll(shutdownCtx)
	}
	// Stop the armed timer before cancelling runs: any in-flight cron
	// terminal watcher still writes its state back while storage is open
	// (bounded by StopCronScheduler's drain window).
	a.service.StopCronScheduler()
	a.service.CancelAll()
	if a.worker != nil {
		if err := a.worker.Close(shutdownCtx); err != nil {
			a.logger.Warn("child worker drain timed out", "err", err)
		}
	}
	if !a.service.WaitIdle(shutdownCtx) {
		a.logger.Warn("shutdown drain timed out; closing storage underneath live runs")
	}
	var mcpCloseDone chan error
	if a.mcpBackend != nil {
		mcpCloseDone = make(chan error, 1)
		go func() { mcpCloseDone <- a.mcpBackend.Close() }()
	}
	var httpCloseErr error
	if a.httpServer != nil {
		httpCloseErr = a.httpServer.Shutdown(shutdownCtx)
	}
	var mcpCloseErr error
	if mcpCloseDone != nil {
		select {
		case mcpCloseErr = <-mcpCloseDone:
		case <-shutdownCtx.Done():
			a.logger.Warn("MCP client shutdown timed out", "err", shutdownCtx.Err())
		}
	}
	var assemblyCloseErr error
	if a.assembly != nil {
		assemblyCloseErr = closeToolWorlds(shutdownCtx, a.assembly.Worlds)
	}
	if a.assembly != nil {
		assemblyCloseErr = errors.Join(assemblyCloseErr, a.assembly.Close(shutdownCtx))
	}
	if httpCloseErr != nil {
		return fmt.Errorf("shutdown http server: %w", httpCloseErr)
	}
	if mcpCloseErr != nil {
		return fmt.Errorf("close MCP clients: %w", mcpCloseErr)
	}
	if assemblyCloseErr != nil {
		return fmt.Errorf("close generated assembly: %w", assemblyCloseErr)
	}
	if err := a.backend.Close(); err != nil {
		return fmt.Errorf("close storage: %w", err)
	}
	return nil
}
