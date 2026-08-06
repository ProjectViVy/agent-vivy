// Package app is the composition root of the vivy process. It owns the
// startup order (storage -> runtime -> httpapi) and the reverse shutdown
// order with a bounded grace period (B2; storage/runtime/httpapi mount
// points land in later batches, see docs/IMPLEMENTATION-PLAN.md).
//
// The config is fully validated before New is called; app never re-reads
// files or environment for non-secret settings (config boundary, FR-10).
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"agent-vivy/internal/config"
)

// shutdownGrace bounds the whole graceful shutdown window. Individual
// components get a sub-budget inside it (E4 hardens per-component limits).
const shutdownGrace = 10 * time.Second

// App is the composed process. B2 wires only the health HTTP server;
// storage, runtime and httpapi are mounted here as they land.
type App struct {
	cfg    config.Config
	logger *slog.Logger

	httpServer *http.Server
}

// New builds the app from a validated config. It returns an error only
// for construction failures; an invalid config must be rejected earlier
// by config.Load / config.Validate.
func New(cfg config.Config) (*App, error) {
	logger := slog.Default()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","stage":"b2-config"}`))
	})

	return &App{
		cfg:    cfg,
		logger: logger,
		httpServer: &http.Server{
			Addr:              cfg.Server.Addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
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
	// Reverse startup order: httpapi first; storage/runtime follow as they
	// are mounted.
	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}
