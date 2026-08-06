// Command vivy is the Agent-Vivy entry point. It loads and validates
// config.yaml, composes the process via internal/app and runs it until
// SIGINT/SIGTERM/Windows console close (bounded graceful shutdown).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"agent-vivy/internal/app"
	"agent-vivy/internal/config"
)

// configPath is the conventional location; absent file falls back to the
// built-in defaults with a warning (FR-10).
const configPath = "config.yaml"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := loadConfig(logger)
	if err != nil {
		logger.Error("startup aborted", "err", err)
		os.Exit(1)
	}

	// Ops override for the listen address. It bypasses file validation on
	// purpose: an invalid value fails fast at listen time.
	if addr := os.Getenv("VIVY_ADDR"); addr != "" {
		logger.Warn("VIVY_ADDR overrides server.addr", "addr", addr)
		cfg.Server.Addr = addr
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg)
	if err != nil {
		logger.Error("composition failed", "err", err)
		os.Exit(1)
	}

	if err := a.Run(ctx); err != nil {
		logger.Error("run failed", "err", err)
		os.Exit(1)
	}
}

// loadConfig reads config.yaml when present; otherwise it falls back to
// the built-in defaults with a warning. Invalid config aborts startup.
func loadConfig(logger *slog.Logger) (config.Config, error) {
	if _, err := os.Stat(configPath); err == nil {
		return config.Load(configPath)
	}
	logger.Warn("config.yaml not found; using built-in defaults", "path", configPath)
	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}
