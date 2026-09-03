// Command vivy-code is the independent VIVY CODE terminal product. It shares
// Vivy's operator configuration but owns a private Journal per launch.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"agent-vivy/internal/codeface"
	"agent-vivy/internal/config"
	"agent-vivy/internal/logging"
	"agent-vivy/internal/worker"
)

const configPath = "config.yaml"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		os.Exit(runWorker())
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h":
			fmt.Fprint(os.Stdout, usage)
			return
		default:
			fmt.Fprintf(os.Stderr, "vivy-code: unknown argument %q\n", os.Args[1])
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
	}

	bootstrap := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := loadConfig(bootstrap)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "vivy-code: resolve current project:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, err := codeface.Run(ctx, cfg, projectDir, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if result.Status != "completed" {
		os.Exit(1)
	}
}

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

func runWorker() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	wlog, closeWLog, wlogPath, err := logging.SetupWorker()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if wlog != nil {
		slog.SetDefault(wlog)
		defer closeWLog.Close()
		wlog.Info("worker started", "pid", os.Getpid(), "path", wlogPath)
	}
	err = worker.Run(ctx, os.Stdin, os.Stdout)
	if wlog != nil {
		if err != nil {
			wlog.Error("worker ended", "err", err)
		} else {
			wlog.Info("worker ended")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

const usage = `vivy-code — independent VIVY CODE terminal

  vivy-code

Provider/model/settings are shared with Vivy. Sessions, messages, approvals,
runs, checkpoints, and logs use a private instance directory for this launch.
`
