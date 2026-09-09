package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"agent-vivy/internal/codeface"
	"agent-vivy/internal/config"
	"agent-vivy/internal/tui"
	"agent-vivy/sdk/tui/live"
	"agent-vivy/sdk/tui/surface"
	"agent-vivy/sdk/tui/view"
)

func runTUI(args []string) int {
	addr := ""
	title := "TUI"
	remote := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--live":
			remote = true
		case "--addr", "-H":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "vivy tui: --addr needs host:port")
				return 2
			}
			i++
			addr = args[i]
			remote = true
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
	if addr == "" {
		addr = os.Getenv("VIVY_ADDR")
	}
	if addr == "" {
		addr = defaultListenAddr()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if !remote {
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
		result, err := codeface.Run(ctx, cfg, cwd, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "vivy tui:", err)
			return 1
		}
		if result.Status != "completed" {
			return 1
		}
		return 0
	}

	debugToolOutput, err := remoteTUIDebug()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	client, err := tui.Dial(ctx, addr, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, "start the gateway first: vivy")
		return 1
	}
	defer client.Close()

	if err := runRemoteTUI(ctx, client, live.Options{Host: addr, Title: title}, debugToolOutput, view.Run); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runRemoteTUI(ctx context.Context, transport live.Transport, opts live.Options, debugToolOutput bool, runView func(surface.Driver, ...view.Options) error) error {
	controller, err := live.New(ctx, transport, opts)
	if err != nil {
		return err
	}
	defer controller.Close()
	if err := runView(controller, view.Options{DebugToolOutput: debugToolOutput, Locale: controller.Locale()}); err != nil {
		return err
	}
	controller.Shutdown()
	return nil
}

func remoteTUIDebug() (bool, error) {
	path := os.Getenv("VIVY_CONFIG")
	if path == "" {
		return config.Default().TUI.Debug, nil
	}
	cfg, err := config.Load(path)
	if err != nil {
		return false, err
	}
	return cfg.TUI.Debug, nil
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
  vivy tui --live [--addr host]   fullscreen shell on a resident gateway
  vivy tui --addr host:port       same as --live

The default command composes the existing Vivy kernel in-process, uses the
current directory as its governed workspace, and starts turns as face=code.
The remote form fails loudly if the gateway is down.
`
