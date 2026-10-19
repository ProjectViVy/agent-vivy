package faceprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"agent-vivy/internal/app"
	"agent-vivy/internal/config"
	"agent-vivy/internal/logging"
	"agent-vivy/sdk/generation"
	plugin "agent-vivy/sdk/port/face"
)

// Selected-Face launcher (ACP-STDIO-FACE §294): the packed artifact's
// no-argument invocation drives the sealed Face Provider over its stdio
// streams against a freshly prepared private runtime. Neither this package
// nor internal/app imports the provider's plugin — the compiled assembly is
// the only datum.
var (
	// runSelectedFace is the app-level selected launch seam. Tests replace
	// it to exercise Run/Main without composing a real App.
	runSelectedFace = app.RunSelectedFaceWithAppOptions

	// teardownBudget is the accepted monotonic shutdown deadline for the
	// whole selected-artifact teardown (spec §490). It is a variable so
	// deadline tests can shrink it.
	teardownBudget = 10 * time.Second
)

// faceUsage is the selected artifact's help text. Protocol mode takes no
// arguments and no face selection — the sealed generation is the face.
const faceUsage = `usage: vivy [--inspect-generation]
       vivy --help

The selected artifact speaks its Face protocol over stdin/stdout when
invoked with no arguments. --inspect-generation prints the sealed
Generation Manifest on stdout without opening the protocol.
`

// Run launches the compiled generation's selected Face Provider against a
// private runtime instance: World=sandbox with the session-aware workspace
// manager over a private fallback root under the instance — sessions with
// workspace_path bind their own root, everything else stays out of the
// process launch directory (spec §115, §314).
//
// The invocation ctx ends on signal or protocol EOF. Teardown then runs
// under its own context inside teardownBudget: owned stream handles close
// first so a provider blocked on stdio unblocks, and a provider still
// running when the budget expires is reported — teardown continues in the
// background without claiming all goroutines finished (spec §486, §490).
func Run(ctx context.Context, cfg config.Config, opts plugin.Options) (plugin.Result, error) {
	if opts.In == nil || opts.Out == nil || opts.Err == nil {
		return plugin.Result{Status: "failed"}, errors.New("faceprocess: selected face requires input, output, and error streams")
	}
	cfg.Runtime.World = "sandbox"
	prepared, err := PreparePrivate(cfg, NamespaceFaceInstances)
	if err != nil {
		return plugin.Result{Status: "failed"}, err
	}
	fallback := filepath.Join(prepared.InstanceRoot, "workspace")
	if err := os.MkdirAll(fallback, 0o700); err != nil {
		return plugin.Result{Status: "failed"}, fmt.Errorf("faceprocess: allocate workspace fallback: %w", err)
	}
	prepared.Config.Runtime.WorkspaceRoot = fallback
	prepared.Config.Runtime.Sandbox.WorkspaceRoot = fallback
	if err := prepared.Config.Validate(); err != nil {
		return plugin.Result{Status: "failed"}, err
	}

	// Protocol stdout carries frames only, including startup failures: the
	// configured sink is file-only for this entry point, and the bootstrap
	// diagnostics in Main stay on the error stream.
	vivyLog, _, closeLog, err := logging.Setup(logging.Options{
		Level:         prepared.Config.Logging.Level,
		Format:        prepared.Config.Logging.Format,
		ConsoleFormat: prepared.Config.Logging.ConsoleFormat,
		Dir:           prepared.Config.LogDirectory(),
		RetentionDays: prepared.Config.Logging.RetentionDays,
		Stdout:        false,
	})
	if err != nil {
		return plugin.Result{Status: "failed"}, err
	}
	defer closeLog.Close()
	slog.SetDefault(vivyLog)

	appOpts := []app.AppOption{
		app.WithSettingsPath(prepared.SharedSettingsPath),
		app.WithInstructionRoot(fallback),
	}

	type outcome struct {
		result plugin.Result
		err    error
	}
	// One monotonic shutdown deadline for the whole teardown (spec §490):
	// the cancel path installs it once and both the stream-close bound and
	// the app's audited drain honor what remains of it.
	var closeDeadline atomic.Pointer[time.Time]

	done := make(chan outcome, 1)
	go func() {
		result, err := runSelectedFace(ctx, prepared.Config, opts, &closeDeadline, appOpts...)
		done <- outcome{result, err}
	}()

	// The launcher owns final stream closure on every exit: downstream
	// readers must see EOF once the provider is done, and closing owned
	// handles during teardown is what unblocks a provider stuck in I/O.
	defer func() {
		closeStream(opts.In)
		closeStream(opts.Out)
	}()

	select {
	case out := <-done:
		return out.result, out.err
	case <-ctx.Done():
	}

	deadline := time.Now().Add(teardownBudget)
	closeDeadline.Store(&deadline)
	teardown, cancel := context.WithDeadline(context.WithoutCancel(ctx), deadline)
	defer cancel()
	closeStream(opts.In)
	closeStream(opts.Out)
	select {
	case out := <-done:
		return out.result, out.err
	case <-teardown.Done():
		return plugin.Result{Status: "failed"}, fmt.Errorf("faceprocess: shutdown exceeded %s", teardownBudget)
	}
}

func closeStream(v any) {
	if c, ok := v.(io.Closer); ok {
		_ = c.Close()
	}
}

// Main is the selected artifact's process entry: no arguments starts the
// Face protocol over in/out, --help prints usage on errOut, and
// --inspect-generation prints the sealed manifest on out — both without
// opening the protocol (spec §294). Process exits: 0 clean run or EOF, 1
// startup/transport/cleanup failure, 2 invalid invocation (spec §492).
func Main(args []string, in io.ReadCloser, out io.WriteCloser, errOut io.Writer) int {
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			_, _ = io.WriteString(errOut, faceUsage)
			return 0
		case "--inspect-generation":
			raw, err := generation.EmbeddedManifest()
			if err != nil {
				_, _ = fmt.Fprintln(errOut, err)
				return 1
			}
			if _, err := out.Write(raw); err != nil {
				_, _ = fmt.Fprintln(errOut, err)
				return 1
			}
			return 0
		default:
			_, _ = fmt.Fprintf(errOut, "vivy: unknown argument %q\n", arg)
			_, _ = io.WriteString(errOut, faceUsage)
			return 2
		}
	}

	bootstrap := slog.New(slog.NewTextHandler(errOut, nil))
	cfg, source, err := config.LoadBoot()
	if err != nil {
		bootstrap.Error("startup aborted", "err", err)
		return 1
	}
	if source == "default" {
		bootstrap.Warn("config.yaml not found; using built-in defaults")
	}

	// os.Interrupt doubles as the Windows console-close signal: the Go
	// runtime delivers CTRL_CLOSE_EVENT to this handler before the OS
	// terminates the process, so the bounded teardown still runs. SIGTERM
	// is never delivered on Windows — signal teardown there is limited to
	// the console-close window.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result, err := Run(ctx, cfg, plugin.Options{In: in, Out: out, Err: errOut})
	if err != nil {
		bootstrap.Error("face run failed", "err", err)
		return 1
	}
	switch result.Status {
	case "completed", "cancelled":
		// A cancelled prompt settles inside the protocol; the process-level
		// drain still completed cleanly.
		return 0
	default:
		bootstrap.Error("face run failed", "status", result.Status)
		return 1
	}
}
