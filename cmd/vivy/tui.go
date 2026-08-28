package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"agent-vivy/internal/config"
	"agent-vivy/internal/tui"
)

func runTUI(args []string) int {
	addr := ""
	title := "TUI"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr", "-H":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "vivy tui: --addr needs host:port")
				return 2
			}
			i++
			addr = args[i]
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

	client, err := tui.Dial(ctx, addr, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, "start the gateway first: vivy")
		return 1
	}
	defer client.Close()

	if err := tui.RunREPL(ctx, client, tui.Options{Title: title}); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func defaultListenAddr() string {
	if path := os.Getenv("VIVY_CONFIG"); path != "" {
		if cfg, err := config.Load(path); err == nil && strings.TrimSpace(cfg.Server.Addr) != "" {
			return cfg.Server.Addr
		}
	}
	if _, err := os.Stat(configPath); err == nil {
		if cfg, err := config.Load(configPath); err == nil && strings.TrimSpace(cfg.Server.Addr) != "" {
			return cfg.Server.Addr
		}
	}
	return config.Default().Server.Addr
}

const tuiUsage = `vivy tui — terminal face over the resident control plane

  vivy tui [--addr host:port] [--title name]

Connects to a running vivy gateway (default 127.0.0.1:8787). This process
does not start a second kernel, does not listen, and does not embed the
web UI. The gateway keeps running after /quit.

This is the first-cut TTY mouth. The packed faces/tui organ in
docs/architecture/VIVY-FACE-PACK.md is a later generation.
`
