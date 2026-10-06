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

	"agent-vivy/internal/buildinfo"
	"agent-vivy/internal/codeface"
	"agent-vivy/internal/config"
)

const configPath = "config.yaml"

func main() {
	parsed := parseArgs(os.Args[1:])
	switch {
	case parsed.Help:
		fmt.Fprint(os.Stdout, usage)
		return
	case parsed.Version:
		fmt.Fprintf(os.Stdout, "vivy-code %s\n", buildinfo.Version)
		return
	}
	for _, w := range parsed.Warnings {
		fmt.Fprintf(os.Stderr, "vivy-code: warning: %s\n", w)
	}
	if len(parsed.Errors) > 0 {
		for _, e := range parsed.Errors {
			fmt.Fprintf(os.Stderr, "vivy-code: %s\n", e)
		}
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
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
	opts := parsed.Options
	opts.In = os.Stdin
	opts.Out = os.Stdout
	opts.Err = os.Stderr
	// pi parity: a non-TTY stdin/stdout means the non-interactive print path
	// unless an explicit headless mode (json/rpc) was already chosen.
	if opts.Mode != "json" && opts.Mode != "rpc" && (!isTerminal(os.Stdin) || !isTerminal(os.Stdout)) {
		opts.Mode = "print"
	}
	result, err := codeface.Run(ctx, cfg, projectDir, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if result.Status != "completed" {
		os.Exit(1)
	}
}

func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	return err == nil && stat.Mode()&os.ModeCharDevice != 0
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

const usage = `vivy-code — independent VIVY CODE terminal

  vivy-code [flags] [prompt ...]     interactive TUI (default)
  vivy-code -p "prompt"              print mode: run once, print response
  vivy-code --mode json -p "..."     print mode with JSONL event records
  vivy-code --mode rpc               JSONL command protocol on stdin/stdout

Modes
  --mode text|json|rpc|print         face dispatch (default text)
  -p, --print [prompt]               one-shot print mode

Model and provider
  --provider <id>                    provider override for this launch
  --model <id>                       model override for this launch
  --api-key <key>                    process-scoped key (never persisted)
  --thinking <level>                 off|minimal|low|medium|high|xhigh|max
  --models <a,b,c>                   scoped model set for cycling
  --list-models [pattern]            list available models and exit
  --system-prompt <text>             replace the system prompt
  --append-system-prompt <text>      append to the system prompt (repeatable)

Session
  -c, --continue                     continue the newest session
  -r, --resume                       pick a prior session to continue
  --session <path>                   resume a session file
  --session-id <id>                  resume a session by id
  --fork <id>                        fork a new branch from a session/message
  --session-dir <dir>                session/instance storage root
  --no-session                       run without session persistence
  -n, --name <name>                  name this session
  --export <path>                    export the session to HTML on exit

Tools and context
  -t, --tools <a,b,c>                restrict the tool set
  -xt, --exclude-tools <a,b,c>       drop tools from the default set
  -nt, --no-tools                    disable all tools
  -nbt, --no-builtin-tools           disable built-in tools (keep MCP/module tools)
  --no-mcp                           disable MCP servers
  --skill <path>                     load an extra skill (repeatable)
  -ns, --no-skills                   disable skills
  --prompt-template <path>           load a prompt template (repeatable)
  -np, --no-prompt-templates         disable prompt templates
  -nc, --no-context-files            skip project instruction files (AGENTS.md)
  @file                              attach a file as prompt context

Interface
  --theme <path>                     load an extra theme (repeatable)
  --use-theme <name>                 select a theme
  --no-themes                        disable custom themes
  --tui-mode regular|fullscreen      terminal layout
  --debug-tools                      show raw tool input/output
  --verbose                          verbose logging
  --offline                          no network model calls

Policy
  -a, --approve                      pre-approve governed actions for this project
  -na, --no-approve                  force approval prompts (deny pre-approval)

Misc
  -h, --help                         this help
  -v, --version                      print version
  --                                 treat remaining arguments as prompt text

Provider/model/settings are shared with Vivy. Sessions, messages, approvals,
runs, checkpoints, and logs use a private instance directory for this launch.
`
