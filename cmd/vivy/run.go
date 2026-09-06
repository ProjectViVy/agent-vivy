package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"agent-vivy/internal/app"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/generated/face"
	"agent-vivy/internal/logging"
	"agent-vivy/sdk/plugin"
)

// runUsage is the `vivy run` help text.
const runUsage = `usage: vivy run [--continue] "prompt"
   echo prompt | vivy run [--continue]

Runs one prompt through the headless face: assistant text streams to
stdout, tool and failure notices go to stderr, and the exit code mirrors
the terminal event (0 completed, 1 failed, 2 cancelled). --continue
attaches the prompt to the most recent session.
`

// runRun drives exactly one prompt through the headless face (D11). A
// piped stdin replaces a missing prompt argument; an interactive call
// with no argument is a usage error, not a hang waiting on a tty.
func runRun(args []string) int {
	continueNewest := false
	var words []string
	for _, arg := range args {
		switch arg {
		case "--continue", "-c":
			continueNewest = true
		case "--help", "-h":
			fmt.Print(runUsage)
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "vivy run: unknown flag %s\n", arg)
				fmt.Fprint(os.Stderr, runUsage)
				return 1
			}
			words = append(words, arg)
		}
	}
	prompt := strings.TrimSpace(strings.Join(words, " "))
	if prompt == "" {
		if stat, err := os.Stdin.Stat(); err == nil && stat.Mode()&os.ModeCharDevice == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "vivy run: read stdin: %v\n", err)
				return 1
			}
			prompt = strings.TrimSpace(string(data))
		}
	}
	if prompt == "" {
		fmt.Fprint(os.Stderr, runUsage)
		return 1
	}

	// The worker branch owns stdout the same way: no log line may corrupt
	// the assistant text stream. Bootstrap diagnostics go to stderr until
	// the configured file sink is installed; the file sink itself gets
	// Stdout=false permanently for this command.
	bootstrap := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := loadConfig(bootstrap)
	if err != nil {
		bootstrap.Error("startup aborted", "err", err)
		return 1
	}
	vivyLog, _, closeLog, err := logging.Setup(logging.Options{
		Level:         cfg.Logging.Level,
		Format:        cfg.Logging.Format,
		Dir:           cfg.LogDirectory(),
		RetentionDays: cfg.Logging.RetentionDays,
		Stdout:        false,
	})
	if err != nil {
		bootstrap.Error("startup aborted", "err", err)
		return 1
	}
	defer closeLog.Close()
	slog.SetDefault(vivyLog)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// A generation packed --face serves run through its organ (face-pack
	// §6); the committed body has none and keeps the built-in kernel
	// headless loop. Exit codes mirror the terminal either way.
	if ctor := face.Register(); ctor != nil {
		faceOpts := plugin.FaceOptions{
			Prompt:          prompt,
			ContinueNewest:  continueNewest,
			DebugToolOutput: cfg.TUI.Debug,
			Out:             os.Stdout,
			Err:             os.Stderr,
		}
		var appOpts []app.AppOption
		instructionRoot, rootErr := resolveInstructionRoot()
		if rootErr != nil {
			fmt.Fprintf(os.Stderr, "vivy run: %v\n", rootErr)
			return 1
		}
		appOpts = append(appOpts, app.WithInstructionRoot(instructionRoot))
		if ctor(faceOpts).Kind() == "tui" {
			appOpts = append(appOpts, app.WithCodeProjectRoot(instructionRoot))
		}
		result, err := app.RunFaceWithAppOptions(ctx, cfg, ctor, faceOpts, appOpts...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vivy run: %v\n", err)
			return 1
		}
		switch result.Status {
		case "completed":
			return 0
		case "cancelled":
			return 2
		default:
			return 1
		}
	}

	instructionRoot, rootErr := resolveInstructionRoot()
	if rootErr != nil {
		fmt.Fprintf(os.Stderr, "vivy run: %v\n", rootErr)
		return 1
	}
	result, err := app.RunHeadless(ctx, cfg, app.HeadlessOptions{
		Prompt:          prompt,
		ContinueNewest:  continueNewest,
		Out:             os.Stdout,
		Err:             os.Stderr,
		InstructionRoot: instructionRoot,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "vivy run: %v\n", err)
		return 1
	}
	switch result.Status {
	case domain.RunCompleted:
		return 0
	case domain.RunCancelled:
		return 2
	default:
		return 1
	}
}

// canonicalPackedTUIProjectRoot gives an explicitly packed TUI generation
// the same server-owned local-project seam as vivy-code. Other Vivy faces do
// not receive this option, so tenant/runtime workspace roots remain private.
func canonicalPackedTUIProjectRoot(projectDir string) (string, error) {
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return "", fmt.Errorf("resolve TUI project: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("inspect TUI project: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("TUI project is not a directory")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("canonicalize TUI project: %w", err)
	}
	return filepath.Abs(root)
}
