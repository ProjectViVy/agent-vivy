// Package app is the composition root of the vivy process. It owns the
// startup order (storage -> providers -> runtime -> JSON-RPC control plane), restart
// recovery of non-terminal runs before the server listens (E2), and the
// reverse shutdown order with a bounded grace period.
//
// The config is fully validated before New is called; app never re-reads
// files or environment for non-secret settings (config boundary, FR-10).
// The provider API key is the exception by design: it is resolved from
// the environment at model construction time (D-010), and a missing key
// aborts startup with an actionable message (FR-11).
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

	service *runtime.Service
	backend storage.Engine
	worker  *workerManager

	httpServer *http.Server
	rpcToken   string
}

// New builds the app from a validated config. It returns an error only
// for construction failures; an invalid config must be rejected earlier
// by config.Load / config.Validate.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	logger := slog.Default()

	// Operator-managed model provider selection (Settings page). It lives
	// in an independent agent working dir, never the production config.yaml
	// or the Journal, and stores no secrets. Overlay it onto the validated
	// config before any provider/model is built, so a saved change takes
	// effect on next launch (no live engine hot-swap).
	cfg = applySettingsOverlay(ctx, logger, cfg)
	originPolicy, err := controlrpc.NewOriginPolicy(cfg.Server.AllowedOrigins)
	if err != nil {
		return nil, fmt.Errorf("app: configure browser origins: %w", err)
	}

	// dataRoot is the process data directory; operator settings live in a
	// subdir of it (an independent agent working dir, not the Journal).
	dataRoot := cfg.DataDirectory()
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		return nil, fmt.Errorf("app: create data dir: %w", err)
	}

	backend, err := openEngine(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Provider bundles from the A2 fixtures; missing files abort startup.
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

	providerName := cfg.Providers.Active
	if cfg.Runtime.Mock {
		providerName = "mock"
	}
	ref, err := catalog.For(providerName)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: resolve provider: %w", err)
	}
	defaultModel := defaultModelFor(cfg, providerName)
	chatModel, err := ref.Model(ctx, defaultModel)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: build chat model: %w", err)
	}
	modelID := defaultModel
	if modelID == "" {
		modelID = "bundle-default"
	}

	var workspaces runtime.WorkspaceAllocator
	var fileOps tools.FileOperations
	var skillOps tools.SkillOperations
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
		skillBackend, err := runtime.NewEinoSkillBackend(cfg.Runtime.SkillsRoot, backend)
		if err != nil {
			_ = backend.Close()
			return nil, fmt.Errorf("app: build skills backend: %w", err)
		}
		skillOps = skillBackend
	}
	todoBackend := runtime.NewEinoTodoBackend(backend, filepath.Join(dataRoot, "todos"))
	todoOps = todoBackend
	searchOps = runtime.NewNetworkSearchService(nil, nil)
	httpOps = runtime.NewEinoHTTPBackend(cfg.Runtime.HTTPAllowedHosts, cfg.Runtime.HTTPMaxResponseBytes, sandboxManager)
	mcpConfigs := make([]runtime.MCPServerConfig, 0, len(cfg.Runtime.MCPServers))
	for _, server := range cfg.Runtime.MCPServers {
		mcpConfigs = append(mcpConfigs, runtime.MCPServerConfig{Name: server.Name, Endpoint: server.Endpoint, AuthEnv: server.AuthEnv})
	}
	mcpOps = runtime.NewEinoMCPBackend(mcpConfigs, nil)
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
	eng, err := runtime.NewEngine(ctx, chatModel, ts, runtime.EngineConfig{
		StreamBuffer:         cfg.Runtime.StreamBuffer,
		MaxEventPayloadBytes: cfg.Runtime.MaxEventPayloadBytes,
		MaxToolTurns:         cfg.Runtime.MaxToolTurns,
		MaxContextBytes:      cfg.Runtime.MaxContextBytes,
		MaxHistoryMessages:   cfg.Runtime.MaxHistoryMessages,
		MaxToolResultBytes:   cfg.Runtime.MaxToolResultBytes,
		Checkpoints:          checkpoints,
		Policy:               policy,
		ToolHooks:            hooks,
	})
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
		PolicyDefaultProfile: domain.PolicyProfile(cfg.Governance.Profile),
		Hooks:                []runtime.RunHook{runtime.AuditHook{Sink: runtime.SlogAuditSink{Logger: logger}}},
		Sink:                 bus,
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
		Approvals: backend, Questions: backend, Reviews: backend, Bus: bus, Service: svc,
		Studio: studioSvc,
		Live: studio.LiveView{
			Provider:      providerName,
			PolicyProfile: liveProfile,
			PolicyHash:    liveSnap.Hash,
			Tools:         liveTools,
		},
		Eval:     evalRunner,
		Children: workerManager,
		// Operator-managed model provider selection lives in an independent
		// agent working dir, never the production config or Journal.
		SettingsPath:   settings.Path(dataRoot),
		ConfigProvider: providerName,
		ConfigModel:    defaultModelFor(cfg, providerName),
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

// applySettingsOverlay reads the operator-managed settings document and
// overlays its non-secret values onto cfg. A missing or empty document is
// a no-op: the config defaults stand. The base URL is applied through the
// existing VIVY_API_BASE environment mechanism (provider.openai already
// honors it), so no provider plumbing changes.
func applySettingsOverlay(ctx context.Context, logger *slog.Logger, cfg config.Config) config.Config {
	dataRoot := cfg.DataDirectory()
	path := settings.Path(dataRoot)
	s, err := settings.Load(path)
	if err != nil {
		// A corrupt settings file must not abort startup; log and ignore.
		logger.Warn("settings overlay skipped", "path", path, "err", err)
		return cfg
	}
	if s == (settings.Settings{}) {
		return cfg
	}
	if s.Provider != "" {
		if s.Provider == settings.ProviderMock {
			cfg.Runtime.Mock = true
		} else {
			cfg.Providers.Active = s.Provider
		}
		switch s.Provider {
		case settings.ProviderOpenAI:
			cfg.Providers.OpenAI.DefaultModel = s.DefaultModel
		case settings.ProviderAnthropic:
			cfg.Providers.Anthropic.DefaultModel = s.DefaultModel
		}
	}
	if s.BaseURL != "" {
		if err := os.Setenv(provider.APIBaseEnvVar, s.BaseURL); err != nil {
			logger.Warn("settings base_url not applied", "err", err)
		}
	}
	logger.Info("settings overlay applied", "provider", cfg.Providers.Active, "model", s.DefaultModel, "base_url_set", s.BaseURL != "")
	return cfg
}

// defaultModelFor picks the configured default model of the active
// provider; the mock and empty values fall back to the bundle default
// inside the Ref.
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
