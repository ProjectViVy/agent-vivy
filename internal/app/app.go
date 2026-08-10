// Package app is the composition root of the vivy process. It owns the
// startup order (storage -> providers -> runtime -> httpapi), restart
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

	"agent-vivy/internal/config"
	"agent-vivy/internal/events"
	"agent-vivy/internal/httpapi"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/sqlite"
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
	backend *sqlite.Backend

	httpServer *http.Server
}

// New builds the app from a validated config. It returns an error only
// for construction failures; an invalid config must be rejected earlier
// by config.Load / config.Validate.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	logger := slog.Default()

	// Storage first: every later component depends on it.
	if dir := filepath.Dir(cfg.Storage.SQLite.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("app: create storage dir: %w", err)
		}
	}
	backend, err := sqlite.Open(ctx, cfg.Storage.SQLite.Path)
	if err != nil {
		return nil, fmt.Errorf("app: open storage: %w", err)
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

	ts, err := tools.Builtin(backend).Resolve(cfg.Tools.Enabled)
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
	checkpoints, err := runtime.NewVersionedCheckpointStore(backend.Blobs(), engineVersion)
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: build checkpoint store: %w", err)
	}
	var workspaces runtime.WorkspaceAllocator
	if cfg.Runtime.WorkspaceRoot != "" {
		manager, err := runtime.NewWorkspaceManager(cfg.Runtime.WorkspaceRoot)
		if err != nil {
			_ = backend.Close()
			return nil, fmt.Errorf("app: build workspace isolation: %w", err)
		}
		workspaces = manager
	}
	eng, err := runtime.NewEngine(ctx, chatModel, ts, runtime.EngineConfig{
		StreamBuffer:         cfg.Runtime.StreamBuffer,
		MaxEventPayloadBytes: cfg.Runtime.MaxEventPayloadBytes,
		MaxToolTurns:         cfg.Runtime.MaxToolTurns,
		MaxContextBytes:      cfg.Runtime.MaxContextBytes,
		MaxHistoryMessages:   cfg.Runtime.MaxHistoryMessages,
		MaxToolResultBytes:   cfg.Runtime.MaxToolResultBytes,
		Checkpoints:          checkpoints,
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
		Workspaces: workspaces,
		Sink:       bus,
	})

	api, err := httpapi.New(httpapi.Deps{
		Sessions:  backend,
		Messages:  backend,
		Runs:      backend,
		Journal:   backend,
		Approvals: backend,
		Questions: backend,
		Bus:       bus,
		Service:   svc,
	})
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: build http api: %w", err)
	}

	// Restart recovery before the server listens (E2, FR-8): every
	// non-terminal run either re-registers on its pending approval or
	// closes with a definitive run.failed. A listing failure means the
	// storage truth is unreachable; startup aborts.
	if err := svc.Recover(ctx); err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("app: restart recovery: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","stage":"e2-recovery"}`))
	})
	// Everything else is the embedded UI shell (single binary, D3).
	mux.Handle("/", ui.Handler())

	return &App{
		cfg:     cfg,
		logger:  logger,
		service: svc,
		backend: backend,
		httpServer: &http.Server{
			Addr:              cfg.Server.Addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

// defaultModelFor picks the configured default model of the active
// provider; the mock and empty values fall back to the bundle default
// inside the Ref.
func defaultModelFor(cfg config.Config, providerName string) string {
	switch providerName {
	case "mock":
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
	a.service.CancelAll()
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
