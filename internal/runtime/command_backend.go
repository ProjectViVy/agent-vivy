package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goRuntime "runtime"
	"strings"
	"sync"
	"time"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

const (
	defaultCommandTimeout = 5 * time.Second
	// defaultMaxCommandTimeout is the execute ceiling used when the caller
	// passes a non-positive maxTimeout (direct backend construction in tests).
	defaultMaxCommandTimeout = 30 * time.Second
	// hardMaxCommandTimeout is the unconfigurable ceiling: a configured
	// ceiling above it is clamped so one execute call can never hang a run
	// for hours even under a misconfigured config.
	hardMaxCommandTimeout = 10 * time.Minute
	maxCommandOutput      = 64 << 10
	maxCommandArgsBytes   = 64 << 10
)

type CommandBackend struct {
	manager        *WorkspaceManager
	sandbox        *SandboxManager
	allowed        map[string]struct{}
	maxOutputBytes int
	maxTimeout     time.Duration
	shellPath      string
	jobs           *tools.JobRegistry
}

var _ tools.CommandOperations = (*CommandBackend)(nil)
var _ tools.JobOperations = (*CommandBackend)(nil)
var _ interface {
	PrepareCommand(context.Context, domain.RunID, tools.CommandRequest) (domain.ToolProposal, error)
} = (*CommandBackend)(nil)

// NewCommandBackend wires the local process backend. maxTimeout bounds a
// single execute/commandline run (from runtime.execute_max_timeout_seconds);
// non-positive falls back to the 30s default and values above
// hardMaxCommandTimeout are clamped to it.
func NewCommandBackend(manager *WorkspaceManager, sandbox *SandboxManager, allowed []string, maxTimeout time.Duration) *CommandBackend {
	if len(allowed) == 0 {
		allowed = []string{"go", "git", "rg"}
	}
	if maxTimeout <= 0 {
		maxTimeout = defaultMaxCommandTimeout
	}
	if maxTimeout > hardMaxCommandTimeout {
		maxTimeout = hardMaxCommandTimeout
	}
	commands := make(map[string]struct{}, len(allowed))
	for _, command := range allowed {
		if name := normalizeCommandName(command); name != "" {
			commands[name] = struct{}{}
		}
	}
	shellPath, _ := exec.LookPath("bash")
	return &CommandBackend{manager: manager, sandbox: sandbox, allowed: commands, maxOutputBytes: maxCommandOutput, maxTimeout: maxTimeout, shellPath: shellPath, jobs: tools.NewJobRegistry()}
}

func (b *CommandBackend) ShellAvailable() bool {
	return b != nil && b.manager != nil && b.sandbox != nil && b.jobs != nil && (goRuntime.GOOS == "windows" || b.shellPath != "")
}

func (b *CommandBackend) Execute(ctx context.Context, runID domain.RunID, request tools.CommandRequest) (tools.CommandResult, error) {
	command, args, cwd, env, timeout, err := b.validateRequest(ctx, runID, request)
	if err != nil {
		return tools.CommandResult{}, err
	}
	path := b.shellPath
	if command != "bash" {
		path, err = exec.LookPath(command)
		if err != nil {
			return tools.CommandResult{}, fmt.Errorf("command: executable %q is unavailable: %w", command, err)
		}
	}
	if command == "bash" {
		return b.executeBash(ctx, path, args, cwd, env, timeout, request.Background)
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	cmd := exec.CommandContext(execCtx, path, args...)
	cmd.Dir = cwd
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return tools.CommandResult{}, fmt.Errorf("command: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return tools.CommandResult{}, fmt.Errorf("command: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return tools.CommandResult{}, fmt.Errorf("command: start: %w", err)
	}
	var out, errOut boundedCommandOutput
	out.limit, errOut.limit = b.maxOutputBytes, b.maxOutputBytes
	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); _, _ = io.Copy(&out, stdout) }()
	go func() { defer readers.Done(); _, _ = io.Copy(&errOut, stderr) }()
	waitErr := cmd.Wait()
	readers.Wait()
	result := tools.CommandResult{Command: strings.Join(append([]string{command}, args...), " "), Cwd: cwd, Stdout: out.String(), Stderr: errOut.String(), StdoutTrunc: out.truncated, StderrTrunc: errOut.truncated, DurationMS: time.Since(started).Milliseconds(), Untrusted: true}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	} else {
		result.ExitCode = -1
	}
	if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if waitErr != nil && result.ExitCode == -1 {
		return result, fmt.Errorf("command: wait: %w", waitErr)
	}
	return result, nil
}

// executeBash runs the shell through the job registry: foreground runs get
// the timeout budget and are adopted as background jobs on timeout; explicit
// background runs return their job id immediately. Jobs are bound to the run
// context, so a finishing or cancelled run reaps them.
func (b *CommandBackend) executeBash(ctx context.Context, path string, args []string, cwd string, env []string, timeout time.Duration, background bool) (tools.CommandResult, error) {
	display := strings.Join(append([]string{"bash"}, args...), " ")
	spec := tools.JobSpec{Display: display, Path: path, Args: args, Dir: cwd, Env: env}
	direct := isDirectShell(ctx)
	if direct && background {
		return tools.CommandResult{}, errors.New("command: direct shell cannot run in background")
	}
	if direct || goRuntime.GOOS == "windows" || path == "" {
		if len(args) != 2 || args[0] != "-c" {
			return tools.CommandResult{}, errors.New("command: embedded bash requires -c script")
		}
		script := args[1]
		spec.Path, spec.Args = "", nil
		spec.Run = func(runCtx context.Context, stdout, stderr io.Writer) error {
			file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
			if err != nil {
				return fmt.Errorf("command: parse shell: %w", err)
			}
			runner, err := interp.New(interp.Dir(cwd), interp.Env(expand.ListEnviron(env...)), interp.StdIO(nil, stdout, stderr), interp.ExecHandlers(portableShellCommands))
			if err != nil {
				return fmt.Errorf("command: build shell: %w", err)
			}
			return runner.Run(runCtx, file)
		}
	}
	if background {
		id, err := b.jobs.Launch(ctx, spec)
		if err != nil {
			return tools.CommandResult{}, err
		}
		return tools.CommandResult{Command: display, Cwd: cwd, DurationMS: 0, Untrusted: true, JobID: id, Background: true, JobStatus: string(tools.JobRunning)}, nil
	}
	if direct {
		result, err := b.jobs.RunForeground(ctx, spec, timeout)
		result.Command, result.Cwd, result.Untrusted = display, cwd, true
		return result, err
	}
	_, result, err := b.jobs.RunUntil(ctx, spec, timeout)
	if err != nil {
		result.Command, result.Cwd, result.Untrusted = display, cwd, true
		return result, err
	}
	result.Command, result.Cwd, result.Untrusted = display, cwd, true
	return result, nil
}

func portableShellCommands(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		if len(args) == 2 && args[0] == "sleep" {
			d, err := time.ParseDuration(args[1] + "s")
			if err != nil || d < 0 {
				return interp.NewExitStatus(2)
			}
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
		return next(ctx, args)
	}
}

// JobRead and JobKill expose the registry to the job_output/job_kill tools.
func (b *CommandBackend) JobRead(jobID string) (tools.JobReadResult, bool) {
	return b.jobs.Read(jobID)
}
func (b *CommandBackend) JobKill(jobID string) (tools.JobKillResult, error) {
	return b.jobs.Kill(jobID)
}
func (b *CommandBackend) PrepareCommand(ctx context.Context, runID domain.RunID, request tools.CommandRequest) (domain.ToolProposal, error) {
	command, args, cwd, _, timeout, err := b.validateRequest(ctx, runID, request)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	payload, _ := json.Marshal(request)
	preview := strings.Join(append([]string{command}, args...), " ")
	if len(preview) > 4096 {
		preview = preview[:4096] + "..."
	}
	return domain.ToolProposal{Action: "commandline", Target: filepath.Join(cwd, command), Preview: fmt.Sprintf("%s (timeout %s)", preview, timeout), RiskFindings: []string{"local process execution", "command output is untrusted"}, Data: payload}, nil
}

func (b *CommandBackend) validateRequest(ctx context.Context, runID domain.RunID, request tools.CommandRequest) (string, []string, string, []string, time.Duration, error) {
	command := strings.TrimSpace(request.Command)
	if command == "" || strings.ContainsAny(command, " \t\r\n/\\;&|><$()") {
		return "", nil, "", nil, 0, errors.New("command: command must be one allowlisted executable name without shell syntax")
	}

	mode := sandboxMode(ctx)
	if !mode.Valid() && b.sandbox != nil {
		mode = b.sandbox.Mode()
	}
	// The bash tool runs shell syntax by contract: its risk is owned by the
	// classifier's deny table and the tiered approval gate, not by the
	// per-executable allowlist. Read-only sandboxes still deny execution.
	if normalizeCommandName(command) == "bash" {
		return b.validateBashRequest(ctx, runID, request, mode)
	}

	// Sandbox validation: check command against sandbox policy (D-021)
	if b.sandbox != nil {
		if err := b.sandbox.ConfineCommandWithMode(command, request.Args, mode); err != nil {
			return "", nil, "", nil, 0, fmt.Errorf("sandbox: %w", err)
		}
	}

	name := normalizeCommandName(command)
	if mode != domain.SandboxModeDangerFullAccess {
		if _, ok := b.allowed[name]; !ok {
			return "", nil, "", nil, 0, fmt.Errorf("command: executable %q is not allowlisted", command)
		}
	}
	if len(request.Args) > 128 {
		return "", nil, "", nil, 0, errors.New("command: too many arguments")
	}
	argsBytes := 0
	for _, arg := range request.Args {
		if len(arg) > 4096 {
			return "", nil, "", nil, 0, errors.New("command: argument exceeds 4096 bytes")
		}
		argsBytes += len(arg)
		if strings.IndexByte(arg, 0) >= 0 {
			return "", nil, "", nil, 0, errors.New("command: NUL in argument")
		}
	}
	if argsBytes > maxCommandArgsBytes {
		return "", nil, "", nil, 0, errors.New("command: argument payload exceeds size limit")
	}
	cwdPath, env, timeout, err := b.resolveCommandContext(ctx, runID, request.Cwd, request.TimeoutMS, request.Env)
	if err != nil {
		return "", nil, "", nil, 0, err
	}
	return command, append([]string(nil), request.Args...), cwdPath, env, timeout, nil
}

// validateBashRequest is the bash-tool path: the sandbox command whitelist
// does not apply (the classifier deny table plus tiered approval own that
// risk), but read-only sandboxes still deny execution and the deny table is
// re-checked here as defense in depth.
func (b *CommandBackend) validateBashRequest(ctx context.Context, runID domain.RunID, request tools.CommandRequest, mode domain.SandboxMode) (string, []string, string, []string, time.Duration, error) {
	if b.shellPath == "" && goRuntime.GOOS != "windows" {
		return "", nil, "", nil, 0, errors.New("command: bash is not available on this host")
	}
	if mode == domain.SandboxModeReadOnly {
		return "", nil, "", nil, 0, fmt.Errorf("%w: command execution not allowed in read-only mode", ErrSandboxDenied)
	}
	if len(request.Args) != 2 || request.Args[0] != "-c" {
		return "", nil, "", nil, 0, errors.New("command: bash expects a single -c script")
	}
	if class, findings, err := tools.ClassifyShellScript(request.Args[1]); err != nil {
		return "", nil, "", nil, 0, err
	} else if class == tools.InvocationDenied {
		return "", nil, "", nil, 0, fmt.Errorf("command: bash: %s", strings.Join(findings, "; "))
	}
	cwdPath, env, timeout, err := b.resolveCommandContext(ctx, runID, request.Cwd, request.TimeoutMS, nil)
	if err != nil {
		return "", nil, "", nil, 0, err
	}
	return "bash", append([]string(nil), request.Args...), cwdPath, env, timeout, nil
}

func (b *CommandBackend) resolveCommandContext(ctx context.Context, runID domain.RunID, cwdRequest string, timeoutMS int, envOverrides map[string]string) (string, []string, time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, 0, err
	}
	if b.manager == nil {
		return "", nil, 0, errors.New("command: workspace manager not wired")
	}
	workspace, err := b.manager.Ensure(ctx, runID)
	if err != nil {
		return "", nil, 0, err
	}
	cwd := strings.TrimSpace(cwdRequest)
	if cwd == "" {
		cwd = "."
	}
	if filepath.IsAbs(cwd) {
		return "", nil, 0, errors.New("command: cwd must be workspace-relative")
	}
	cwdPath := filepath.Join(workspace.Path, filepath.Clean(cwd))
	relativeCwd, err := filepath.Rel(workspace.Path, cwdPath)
	if err != nil || relativeCwd == ".." || strings.HasPrefix(relativeCwd, ".."+string(filepath.Separator)) || filepath.IsAbs(relativeCwd) {
		return "", nil, 0, errors.New("command: cwd escapes workspace")
	}
	realCwd, err := filepath.EvalSymlinks(cwdPath)
	if err != nil {
		return "", nil, 0, fmt.Errorf("command: resolve cwd: %w", err)
	}
	realWorkspace, _ := filepath.EvalSymlinks(workspace.Path)
	if !strings.EqualFold(filepath.Clean(realCwd), filepath.Clean(realWorkspace)) {
		if err := b.manager.ValidatePath(realCwd); err != nil {
			return "", nil, 0, errors.New("command: cwd symlink escapes workspace")
		}
	}
	info, err := os.Stat(realCwd)
	if err != nil || !info.IsDir() {
		return "", nil, 0, errors.New("command: cwd is not a directory")
	}
	env, err := safeCommandEnv(envOverrides)
	if err != nil {
		return "", nil, 0, err
	}
	timeout := defaultCommandTimeout
	if timeoutMS > 0 {
		timeout = time.Duration(timeoutMS) * time.Millisecond
	}
	if timeout > b.maxTimeout {
		timeout = b.maxTimeout
	}
	return realCwd, env, timeout, nil
}

type boundedCommandOutput struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (w *boundedCommandOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	remaining := w.limit - w.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			_, _ = w.buffer.Write(data[:remaining])
			w.truncated = true
		} else {
			_, _ = w.buffer.Write(data)
		}
	} else if len(data) > 0 {
		w.truncated = true
	}
	return len(data), nil
}

func (w *boundedCommandOutput) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

func normalizeCommandName(command string) string {
	command = strings.ToLower(strings.TrimSpace(command))
	command = filepath.Base(command)
	if ext := filepath.Ext(command); ext == ".exe" || ext == ".cmd" || ext == ".bat" {
		command = strings.TrimSuffix(command, ext)
	}
	return command
}

func safeCommandEnv(overrides map[string]string) ([]string, error) {
	allowed := map[string]bool{
		"CI": true, "GIT_CONFIG_NOSYSTEM": true, "GIT_CONFIG_NOGLOBAL": true,
		"GIT_TERMINAL_PROMPT": true, "GOFLAGS": true, "GOWORK": true,
		"GOCACHE": true, "GOMODCACHE": true, "LANG": true, "LC_ALL": true,
		"NO_COLOR": true, "RUST_BACKTRACE": true, "TERM": true,
	}
	env := make([]string, 0, 4+len(overrides))
	for _, key := range []string{"PATH", "PATHEXT", "SYSTEMROOT", "WINDIR", "TEMP", "TMP"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	for key, value := range overrides {
		key = strings.ToUpper(strings.TrimSpace(key))
		if !allowed[key] {
			return nil, fmt.Errorf("command: environment variable %q is not allowlisted", key)
		}
		if len(value) > 4096 || strings.IndexByte(value, 0) >= 0 {
			return nil, fmt.Errorf("command: environment variable %q is invalid or too large", key)
		}
		env = append(env, key+"="+value)
	}
	return env, nil
}
