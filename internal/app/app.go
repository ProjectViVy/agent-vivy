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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/eval"
	"agent-vivy/internal/events"
	genplugins "agent-vivy/internal/generated/plugins"
	"agent-vivy/internal/pluginhost"
	"agent-vivy/internal/provider"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/postgres"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/studio"
	"agent-vivy/internal/tools"
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

	service  *runtime.Service
	backend  storage.Engine
	worker   *workerManager
	resolver *ModelResolver

	httpServer *http.Server
	rpcToken   string
}

// New builds the app from a validated config. It returns an error only
// for construction failures; an invalid config must be rejected earlier
// by config.Load / config.Validate.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	logger := slog.Default()

	// Operator-managed preferences (network search, execute ceiling) overlay
	// the validated config. Provider keys are NOT applied to the process
	// environment; ModelResolver reads settings.yaml / frozen ENV per call.
	cfg = applySettingsOverlay(ctx, logger, cfg)
	originPolicy, err := controlrpc.NewOriginPolicy(cfg.Server.AllowedOrigins)
	if err != nil {
		return nil, fmt.Errorf("app: configure browser origins: %w", err)
	}

	dataRoot := cfg.DataDirectory()
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		return nil, fmt.Errorf("app: create data dir: %w", err)
	}

	backend, err := openEngine(ctx, cfg)
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
	resolver := newModelResolver(cfg, settings.Path(dataRoot), catalog)
	cur := resolver.Current()
	providerName := cur.Provider
	if providerName == "" {
		providerName = cfg.Providers.Active
	}
	modelID := cur.Model
	if modelID == "" {
		modelID = defaultModelFor(cfg, providerName)
	}
	chatModel := provider.NewResolvingChatModel(catalog, resolver)

	var workspaces runtime.WorkspaceAllocator
	var fileOps tools.FileOperations
	var skillOps tools.SkillOperations
	var skillBackend *runtime.EinoSkillBackend
	var todoOps tools.TodoOperations
	var searchOps tools.SearchOperations
	var httpOps tools.HTTPOperations
	var mcpOps tools.MCPOperations
	var sequentialOps tools.SequentialThinkingOperations
	var commandOps tools.CommandOperations
	var workspaceManager *runtime.WorkspaceManager
	var sandboxManager *runtime.SandboxManager
	if cfg.Runtime.WorkspaceRoot != "" {
		manager, err := runtime.NewWorkspaceManager(cfg.Runtime.WorkspaceRoot)
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

		fileOps = runtime.NewEinoFilesystemBackend(manager, sandboxManager)
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
	todoBackend := runtime.NewEinoTodoBackend(backend, filepath.Join(dataRoot, "todos"))
	todoOps = todoBackend
	searchService := runtime.NewNetworkSearchService(nil, nil)
	searchService.SetPreferredProvider(cfg.Tools.NetworkSearch.Provider)
	searchOps = searchService
	httpOps = runtime.NewEinoHTTPBackend(cfg.Runtime.HTTPAllowedHosts, cfg.Runtime.HTTPMaxResponseBytes, sandboxManager)
	mcpBackend := runtime.NewEinoMCPBackend(mcpRuntimeConfigs(cfg.Runtime.MCPServers), nil)
	mcpOps = mcpBackend
	sequentialOps = runtime.NewEinoSequentialThinkingBackend()
	commandOps = runtime.NewEinoCommandBackend(workspaceManager, sandboxManager, cfg.Runtime.ExecuteAllowedCommands, time.Duration(cfg.Runtime.ExecuteMaxTimeoutSeconds)*time.Second)
	ts, err := tools.BuiltinWithCommands(backend, fileOps, skillOps, todoOps, searchOps, httpOps, mcpOps, sequentialOps, commandOps).Resolve(cfg.Tools.Enabled)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: resolve tools: %w", err)
	}
	var lookup pluginhost.WorkspaceLookup
	if workspaceManager != nil {
		lookup = func(ctx context.Context) (string, error) {
			ws, err := workspaceManager.Ensure(ctx, tools.RunIDFromContext(ctx))
			if err != nil {
				return "", err
			}
			return ws.Path, nil
		}
	}
	ts = append(ts, pluginhost.Adapt(genplugins.Register(), lookup)...)
	// The checkpoint bridge fail-closes on its engine version, so an
	// unknown build version aborts startup rather than suspend runs on
	// unverifiable checkpoints (C6).
	engineVersion := runtime.EinoEngineVersion()
	if engineVersion == "" {
		_ = backend.Close()
		return nil, errors.New("app: eino engine version unavailable; checkpoint store cannot be anchored")
	}
	checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), engineVersion)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: build checkpoint store: %w", err)
	}
	policy := policyEngine(cfg)
	hooks := runtime.NewToolHookChain(cfg.Governance.HookTimeout)
	// Resolve the effective context-compression policy against the model's
	// context window (settings overlay already folded into cfg at startup).
	modelWindow := 0
	if info, infoErr := catalog.ResolveModelInfo(ctx, providerName, modelID); infoErr == nil {
		modelWindow = info.ContextWindow
	}
	cmp := compactionPolicyFor(cfg, nil, modelWindow)
	engineCfg := buildEngineConfig(cfg, skillBackend, checkpoints, policy, hooks, &cmp)
	eng, err := runtime.NewEngine(ctx, chatModel, ts, engineCfg)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: build engine: %w", err)
	}

	bus := events.NewBus(cfg.Runtime.StreamBuffer)
	svc := runtime.NewService(eng, providerName, modelID, runtime.ServiceDeps{
		Journal:            backend,
		Runs:               backend,
		Messages:           backend,
		Notes:              backend,
		Approvals:          backend,
		Questions:          backend,
		ApprovalExpiration: cfg.Tools.Approval.Expiration,
		Budget: runtime.BudgetPolicy{
			MaxEvents: cfg.Runtime.MaxRunEvents, MaxModelCalls: cfg.Runtime.MaxModelCalls,
			MaxToolCalls: cfg.Runtime.MaxRunToolCalls, MaxRetries: cfg.Runtime.MaxRunRetries,
		},
		Workspaces:           workspaces,
		Sessions:             backend,
		PolicyDefaultProfile: domain.PolicyProfile(cfg.Governance.Profile),
		Hooks:                []runtime.RunHook{runtime.AuditHook{Sink: runtime.SlogAuditSink{Logger: logger}}},
		Sink:                 bus,
		Compactions:          backend,
		Crons:                backend,
		RebuildEngine: func(ctx context.Context, ec runtime.EngineConfig) (*runtime.Engine, error) {
			return runtime.NewEngine(ctx, chatModel, ts, ec)
		},
	})
	svc.SetCatalog(catalog)
	workerManager := newWorkerManager(svc, backend, backend, policy, hooks, ts, cfg.Runtime.MaxToolResultBytes, cfg.Tools.Approval.Expiration, chatModel)
	svc.SetChildApprovalRouter(workerManager)

	liveProfile := domain.PolicyProfile(cfg.Governance.Profile)
	if !liveProfile.Valid() {
		liveProfile = domain.PolicyProfileDefault
	}
	liveSnap, err := policy.Snapshot(liveProfile)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: snapshot default policy: %w", err)
	}
	liveTools := make([]domain.ToolSpec, 0, len(ts))
	for _, tool := range ts {
		liveTools = append(liveTools, tool.Spec())
	}
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
	controlHandler, err := controlrpc.NewControlHandler(controlrpc.ControlDeps{
		Sessions: backend, Messages: backend, Runs: backend, Journal: backend,
		Approvals: backend, Questions: backend, Reviews: backend, Todos: backend, Skills: skillOps, Bus: bus, Service: svc,
		Crons: backend, CronRunner: svc,
		Studio: studioSvc,
		Live: studio.LiveView{
			Provider:      providerName,
			PolicyProfile: liveProfile,
			PolicyHash:    liveSnap.Hash,
			Tools:         liveTools,
		},
		Eval:                           evalRunner,
		Children:                       workerManager,
		SettingsPath:                   settings.Path(dataRoot),
		ConfigProvider:                 providerName,
		ConfigModel:                    modelID,
		ConfigNetworkSearchProvider:    cfg.Tools.NetworkSearch.Provider,
		ConfigExecuteMaxTimeoutSeconds: cfg.Runtime.ExecuteMaxTimeoutSeconds,
		DefaultPermissionPreset:        defaultPermissionPreset(cfg),
		SandboxWorkspaceRoot:           cfg.Runtime.WorkspaceRoot,
		ExecuteAllowedCommands:         append([]string(nil), cfg.Runtime.ExecuteAllowedCommands...),
		ConfigSandboxDenyPrivateIPs:    cfg.Runtime.Sandbox.Network.DenyPrivateIPs,
		ConfigSandboxAllowedDomains:    append([]string(nil), cfg.Runtime.Sandbox.Network.AllowedDomains...),
		ConfigCompaction:               cmp,
		// Write-time env apply: a settings/providers save updates the
		// running process environment (base_url → VIVY_API_BASE, resolved
		// api_key → active bundle env_key) immediately; the startup overlay
		// replays the same document on the next launch.
		ApplySettingsEnv: func(s settings.Settings) { applySettingsEnv(logger, cfg, s) },
		TokenUsage:       backend,
		MCP:              mcpBackend,
		Frozen:           resolver.Frozen(),
		OnSettingsChanged: func() {
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
			applyLiveSandboxSettings(sandboxManager, settings.Path(dataRoot), cfg)
			s, err := settings.Load(settings.Path(dataRoot))
			if err != nil {
				logger.Warn("mcp overlay reload skipped", "err", err)
				return
			}
			mcpBackend.ReplaceServers(liveMCPConfigs(cfg, s))
			// Compaction live-apply: rebuild the engine (reduction +
			// summarization middleware) only when the effective policy
			// changed. The rebuild lands immediately when idle, otherwise
			// at the next idle run start.
			window := svc.GetModelInfo(context.Background()).ContextWindow
			cmp := compactionPolicyFor(cfg, s.Compaction, window)
			if !sameCompactionPolicy(svc.CompactionPolicy(), &cmp) {
				if err := svc.ScheduleEngineReload(buildEngineConfig(cfg, skillBackend, checkpoints, policy, hooks, &cmp)); err != nil {
					logger.Warn("compaction engine reload failed", "err", err)
				}
			}
		},
	})
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: build rpc control plane: %w", err)
	}
	rpcToken := controlrpc.NewSessionToken()

	// Restart recovery before the server listens (E2, FR-8): every
	// non-terminal run either re-registers on its pending approval or
	// closes with a definitive run.failed. A listing failure means the
	// storage truth is unreachable; startup aborts.
	if err := svc.Recover(ctx); err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: restart recovery: %w", err)
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

	return &App{
		cfg:      cfg,
		logger:   logger,
		service:  svc,
		backend:  backend,
		worker:   workerManager,
		resolver: resolver,
		rpcToken: rpcToken,
		httpServer: &http.Server{
			Addr:              cfg.Server.Addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
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
func applySettingsOverlay(ctx context.Context, logger *slog.Logger, cfg config.Config) config.Config {
	dataRoot := cfg.DataDirectory()
	path := settings.Path(dataRoot)
	s, err := settings.Load(path)
	if err != nil {
		logger.Warn("settings overlay skipped", "path", path, "err", err)
		return cfg
	}
	if s.IsZero() {
		return cfg
	}
	if s.Provider != "" && s.Provider != settings.ProviderMock {
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
	logger.Info("settings overlay applied", "provider", cfg.Providers.Active, "model", s.DefaultModel, "network_search_provider", cfg.Tools.NetworkSearch.Provider, "execute_max_timeout_seconds", cfg.Runtime.ExecuteMaxTimeoutSeconds, "sandbox_preset", s.Sandbox.DefaultPreset, "mcp_servers", len(cfg.Runtime.MCPServers), "compaction_enabled", cfg.Runtime.Compaction.Enabled)
	return cfg
}

// enabledMCPFromSettings returns the enabled MCP servers from the overlay,
// or nil when the overlay has never been written (config default stands).
func enabledMCPFromSettings(s settings.Settings) []config.MCPServer {
	if s.MCPServers == nil {
		return nil
	}
	out := make([]config.MCPServer, 0, len(*s.MCPServers))
	for _, server := range *s.MCPServers {
		if !settings.MCPServerEnabled(server) {
			continue
		}
		out = append(out, config.MCPServer{Name: server.Name, Endpoint: server.Endpoint, AuthEnv: server.AuthEnv})
	}
	return out
}

func mcpRuntimeConfigs(servers []config.MCPServer) []runtime.MCPServerConfig {
	out := make([]runtime.MCPServerConfig, 0, len(servers))
	for _, server := range servers {
		out = append(out, runtime.MCPServerConfig{Name: server.Name, Endpoint: server.Endpoint, AuthEnv: server.AuthEnv})
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

func openEngine(ctx context.Context, cfg config.Config) (storage.Engine, error) {
	switch cfg.Storage.Backend {
	case "postgres":
		dsn := os.Getenv(cfg.Storage.Postgres.DSNEnv)
		if dsn == "" {
			return nil, fmt.Errorf("app: %s is empty; postgres DSN is read from the environment (D-010)", cfg.Storage.Postgres.DSNEnv)
		}
		backend, err := postgres.Open(ctx, dsn)
		if err != nil {
			return nil, fmt.Errorf("app: open postgres storage: %w", err)
		}
		return backend, nil
	default:
		if dir := filepath.Dir(cfg.Storage.SQLite.Path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, fmt.Errorf("app: create storage dir: %w", err)
			}
		}
		backend, err := sqlite.Open(ctx, cfg.Storage.SQLite.Path)
		if err != nil {
			return nil, fmt.Errorf("app: open storage: %w", err)
		}
		if err := backend.TakeOrganismLease(ctx); err != nil {
			_ = backend.Close()
			return nil, fmt.Errorf("app: occupy shared workspace: %w", err)
		}
		return backend, nil
	}
}

func defaultModelFor(cfg config.Config, providerName string) string {
	switch providerName {
	case "mock":
		if cfg.Runtime.MockScenario != "" {
			return "mock:" + cfg.Runtime.MockScenario
		}
		return "mock"
	case "anthropic":
		return cfg.Providers.Anthropic.DefaultModel
	default:
		return cfg.Providers.OpenAI.DefaultModel
	}
}

// Run blocks until ctx is cancelled or the server fails. On cancellation
// it shuts down components in reverse startup order with a bounded grace
// period and returns the shutdown error, if any.
func (a *App) Run(ctx context.Context) error {
	a.service.StartInteractionSweeper(context.Background(), time.Second)
	defer a.service.StopInteractionSweeper()
	// Cron fires agent turns on a schedule; it must not outlive the
	// server loop and its in-flight watchers drain during shutdown below.
	if a.cfg.Runtime.Cron.Enabled {
		a.service.StartCronScheduler(context.Background(), runtime.CronSchedulerOptions{})
		defer a.service.StopCronScheduler()
	}
	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("vivy starting", "addr", a.cfg.Server.Addr)
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
		close(errCh)
	}()

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

	// Reverse startup order with hard ordering guarantees (E4): runs are
	// cancelled and then drained while storage is still open, so every
	// run.cancelled terminal persists before the journal closes; only
	// then do the HTTP server and the backend shut down.
	a.service.StopInteractionSweeper()
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
	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	if err := a.backend.Close(); err != nil {
		return fmt.Errorf("close storage: %w", err)
	}
	return nil
}
