// Command vivy is the Agent-Vivy entry point. It loads and validates
// config.yaml, composes the process via internal/app and runs it until
// SIGINT/SIGTERM/Windows console close (bounded graceful shutdown).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"agent-vivy/internal/app"
	"agent-vivy/internal/config"
	"agent-vivy/internal/logging"
	"agent-vivy/internal/worker"
)

// configPath is the conventional location; absent file falls back to the
// built-in defaults with a warning (FR-10).
const configPath = "config.yaml"

func main() {
	// The worker protocol owns stdout. Keep this branch before the normal
	// logger is installed so startup diagnostics can never corrupt JSONL.
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := worker.Run(ctx, os.Stdin, os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "init" {
		os.Exit(runInit(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "tui" {
		os.Exit(runTUI(os.Args[2:]))
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := loadConfig(logger)
	if err != nil {
		logger.Error("startup aborted", "err", err)
		os.Exit(1)
	}

	// Swap the bootstrap logger for the configured one: same stdout stream
	// plus the daily-rotated file sink, level/format from config with
	// VIVY_LOG_LEVEL / VIVY_LOG_FORMAT overrides (docs/architecture/LOGGING.md).
	// Everything before this line lands on stdout only; everything after
	// lands in the file too. Synchronous writes mean the closer below is an
	// orderly-shutdown formality, not a flush dependency.
	vivyLog, eff, closeLog, err := logging.Setup(logging.Options{
		Level:         cfg.Logging.Level,
		Format:        cfg.Logging.Format,
		Dir:           cfg.LogDirectory(),
		RetentionDays: cfg.Logging.RetentionDays,
		Stdout:        cfg.Logging.Stdout,
	})
	if err != nil {
		logger.Error("startup aborted", "err", err)
		os.Exit(1)
	}
	defer closeLog.Close()
	slog.SetDefault(vivyLog)
	logger = vivyLog
	vivyLog.Info("logging initialized",
		"level", eff.Level,
		"format", eff.Format,
		"dir", cfg.LogDirectory(),
		"stdout", cfg.Logging.Stdout)

	// Ops override for the listen address. Revalidate the effective config so
	// a split UI's loopback exposure policy cannot be bypassed by the env var.
	if addr := os.Getenv("VIVY_ADDR"); addr != "" {
		logger.Warn("VIVY_ADDR overrides server.addr", "addr", addr)
		cfg.Server.Addr = addr
		if err := cfg.Validate(); err != nil {
			logger.Error("startup aborted", "err", err)
			os.Exit(1)
		}
	}

	// os.Interrupt doubles as the Windows console-close signal: the Go
	// runtime delivers CTRL_CLOSE_EVENT to this handler before the OS
	// terminates the process (~5s window), so the bounded graceful
	// shutdown still runs (E4).
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

// loadConfig reads VIVY_CONFIG when set, otherwise config.yaml when
// present; otherwise it falls back to the built-in defaults. A set
// VIVY_CONFIG never falls back to the working directory.
func loadConfig(logger *slog.Logger) (config.Config, error) {
	if path := os.Getenv("VIVY_CONFIG"); path != "" {
		return config.Load(path)
	}
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
