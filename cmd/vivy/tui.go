package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"agent-vivy/internal/app"
	"agent-vivy/internal/config"
	"agent-vivy/internal/logging"
	"agent-vivy/internal/tui"
	"agent-vivy/internal/tui/view"
	"agent-vivy/sdk/plugin"
)

func runTUI(args []string) int {
	addr := ""
	title := "TUI"
	mode := "" // local | demo | plain | live
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--demo":
			mode = "demo"
		case "--plain":
			mode = "plain"
		case "--live":
			mode = "live"
		case "--addr", "-H":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "vivy tui: --addr needs host:port")
				return 2
			}
			i++
			addr = args[i]
			if mode == "" {
				// Bare --addr without --live keeps the line REPL (compat).
				mode = "plain"
			}
		case "--title":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "vivy tui: --title needs a value")
				return 2
			}
			i++
			title = args[i]
		case "--help", "-h":
			fmt.Fprint(os.Stderr, tuiUsage)
			return 0
		default:
			fmt.Fprintf(os.Stderr, "vivy tui: unknown argument %q\n", args[i])
			fmt.Fprint(os.Stderr, tuiUsage)
			return 2
		}
	}
	if mode == "" {
		mode = "local"
	}
	if mode == "demo" {
		if err := view.RunDemo(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if addr == "" {
		addr = os.Getenv("VIVY_ADDR")
	}
	if addr == "" {
		addr = defaultListenAddr()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if mode == "local" {
		bootstrap := slog.New(slog.NewTextHandler(os.Stderr, nil))
		cfg, err := loadConfig(bootstrap)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "vivy tui: resolve current project:", err)
			return 1
		}
		cfg.Runtime.World = "local"
		cfg.Runtime.WorkspaceRoot = cwd
		cfg.Runtime.Sandbox.WorkspaceRoot = cwd
		if err := cfg.Validate(); err != nil {
			fmt.Fprintln(os.Stderr, "vivy tui:", err)
			return 1
		}
		vivyLog, _, closeLog, err := logging.Setup(logging.Options{
			Level: cfg.Logging.Level, Format: cfg.Logging.Format,
			Dir: cfg.LogDirectory(), RetentionDays: cfg.Logging.RetentionDays,
			Stdout: false,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "vivy tui:", err)
			return 1
		}
		defer closeLog.Close()
		slog.SetDefault(vivyLog)
		result, err := app.RunFace(ctx, cfg, tui.NewFace, plugin.FaceOptions{Out: os.Stdout, Err: os.Stderr})
		if err != nil {
			fmt.Fprintln(os.Stderr, "vivy tui:", err)
			return 1
		}
		if result.Status != "completed" {
			return 1
		}
		return 0
	}

	client, err := tui.Dial(ctx, addr, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, "start the gateway first: vivy")
		return 1
	}
	defer client.Close()

	if mode == "live" {
		live := tui.NewLive(client, tui.LiveOptions{Host: addr, Title: title, Face: "code"})
		defer live.Close()
		if err := view.Run(live); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if err := tui.RunREPL(ctx, client, tui.Options{Title: title}); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func defaultListenAddr() string {
	// Match cmd/vivy loadConfig: VIVY_CONFIG overlay, else user-workspace defaults.
	// A working-directory config.yaml is no longer a product entry.
	if path := os.Getenv("VIVY_CONFIG"); path != "" {
		if cfg, err := config.Load(path); err == nil && strings.TrimSpace(cfg.Server.Addr) != "" {
			return cfg.Server.Addr
		}
	}
	return config.Default().Server.Addr
}

const tuiUsage = `vivy tui — terminal face

  vivy tui                        real VIVY CODE in the current project
  vivy tui --demo                 fullscreen Crush-style skeleton (offline demo data)
  vivy tui --live [--addr host]   fullscreen shell on a resident gateway
  vivy tui --plain [--addr host]  line REPL over a resident gateway
  vivy tui --addr host:port       same as --plain

The default command composes the existing Vivy kernel in-process, uses the
current directory as its governed workspace, and starts turns as face=code.
--demo is an explicit development fixture and never a product fallback.
--live fails loudly if the gateway is down (does not fall back to demo).
`
