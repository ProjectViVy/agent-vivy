package runtime

import (
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
	manager     *WorkspaceManager
	sandbox     *SandboxManager
	allowed     map[string]struct{}
	maxTimeout  time.Duration
	shellPath   string
	shellPrefix string
	spillBytes  int64
	jobs        *tools.JobRegistry
}

// CommandBackendOptions carries optional runtime.* settings that only the
// module wiring layer supplies; zero values keep historical behavior.
type CommandBackendOptions struct {
	// ShellPrefix (runtime.shell_command_prefix) is prepended to every bash
	// script and wraps commandline argv as `bash -c '<prefix> "$@"'`.
	ShellPrefix string
	// SpillBytes (runtime.tool_output_spill_bytes) is the per-stream inline
	// bound beyond which the full output spills to a workspace file.
	SpillBytes int
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
func NewCommandBackend(manager *WorkspaceManager, sandbox *SandboxManager, allowed []string, maxTimeout time.Duration, options ...CommandBackendOptions) *CommandBackend {
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
	var opts CommandBackendOptions
	if len(options) > 0 {
		opts = options[0]
	}
	spillBytes := int64(opts.SpillBytes)
	if spillBytes <= 0 || spillBytes > maxCommandOutput {
		spillBytes = maxCommandOutput
	}
	shellPath, _ := exec.LookPath("bash")
	return &CommandBackend{manager: manager, sandbox: sandbox, allowed: commands, maxTimeout: maxTimeout, shellPath: shellPath, shellPrefix: opts.ShellPrefix, spillBytes: spillBytes, jobs: tools.NewJobRegistry()}
}

func (b *CommandBackend) ShellAvailable() bool {
	return b != nil && b.manager != nil && b.sandbox != nil && b.jobs != nil && (goRuntime.GOOS == "windows" || b.shellPath != "")
}

func (b *CommandBackend) Execute(ctx context.Context, runID domain.RunID, request tools.CommandRequest) (tools.CommandResult, error) {
	validated, err := b.validateRequest(ctx, runID, request)
	if err != nil {
		return tools.CommandResult{}, err
	}
	path := b.shellPath
	if validated.command != "bash" {
		path, err = exec.LookPath(validated.command)
		if err != nil {
			return tools.CommandResult{}, fmt.Errorf("command: executable %q is unavailable: %w", validated.command, err)
		}
	}
	if validated.command == "bash" {
		return b.executeBash(ctx, path, validated, request.Background)
	}
	// execute/commandline share the job registry's foreground path so the
	// output spill and tail-bound behavior match the bash surface.
	spec := tools.JobSpec{
		Display: strings.Join(append([]string{validated.command}, validated.args...), " "),
		Path:    path, Args: validated.args, Dir: validated.scope.cwd, Env: validated.scope.env,
		SpillDir: validated.scope.spillDir, SpillID: validated.scope.spillID, SpillBytes: b.spillBytes,
	}
	result, err := b.jobs.RunForeground(ctx, spec, validated.scope.timeout)
	if result.Command == "" {
		result.Command = spec.Display
	}
	result.Cwd, result.Untrusted = validated.scope.cwd, true
	if err != nil {
		return result, err
	}
	return result, nil
}

// executeBash runs the shell through the job registry: foreground runs get
// the timeout budget and are adopted as background jobs on timeout; explicit
// background runs return their job id immediately. Jobs are bound to the run
// context, so a finishing or cancelled run reaps them.
func (b *CommandBackend) executeBash(ctx context.Context, path string, validated commandValidated, background bool) (tools.CommandResult, error) {
	args, cwd, env, timeout := validated.args, validated.scope.cwd, validated.scope.env, validated.scope.timeout
	display := strings.Join(append([]string{"bash"}, args...), " ")
	spec := tools.JobSpec{Display: display, Path: path, Args: args, Dir: cwd, Env: env,
		SpillDir: validated.scope.spillDir, SpillID: validated.scope.spillID, SpillBytes: b.spillBytes}
	direct := isDirectShell(ctx)
	if direct && background {
		return tools.CommandResult{}, errors.New("command: direct shell cannot run in background")
	}
	if direct || goRuntime.GOOS == "windows" || path == "" {
		var script string
		switch {
		case len(args) == 2 && args[0] == "-c":
			script = args[1]
		case len(args) >= 4 && args[0] == "-c" && args[2] == shellPrefixArgv0:
			// commandline wrapped for the configured prefix: flatten to one
			// script for the embedded interpreter (prefix must end with a
			// shell separator, same contract as the OS-bash wrap).
			prefix := strings.TrimSuffix(args[1], " \"$@\"")
			script = prefix + " " + shellQuoteJoin(args[3:])
		default:
			return tools.CommandResult{}, errors.New("command: embedded bash requires -c script")
		}
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
			if runErr := runner.Run(runCtx, file); runErr != nil {
				var status interp.ExitStatus
				if errors.As(runErr, &status) {
					return tools.ExitStatusError{Code: int(status)}
				}
				return runErr
			}
			return nil
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

type embeddedShellExitStatus uint8

func (e embeddedShellExitStatus) Error() string { return fmt.Sprintf("exit status %d", e) }
func (e embeddedShellExitStatus) ExitCode() int { return int(e) }

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
	validated, err := b.validateRequest(ctx, runID, request)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	payload, _ := json.Marshal(request)
	preview := strings.Join(append([]string{validated.command}, validated.args...), " ")
	if len(preview) > 4096 {
		preview = preview[:4096] + "..."
	}
	return domain.ToolProposal{Action: "commandline", Target: filepath.Join(validated.scope.cwd, validated.command), Preview: fmt.Sprintf("%s (timeout %s)", preview, validated.scope.timeout), RiskFindings: []string{"local process execution", "command output is untrusted"}, Data: payload}, nil
}

// commandScope is the resolved execution context for one request: sandboxed
// cwd, sanitized env, bounded timeout, and the workspace spill target.
type commandScope struct {
	cwd      string
	env      []string
	timeout  time.Duration
	spillDir string
	spillID  string
}

// commandValidated is a request that passed policy/allowlist checks and is
// ready for process launch; args may already carry the shell prefix rewrite.
type commandValidated struct {
	command string
	args    []string
	scope   commandScope
}

func (b *CommandBackend) validateRequest(ctx context.Context, runID domain.RunID, request tools.CommandRequest) (commandValidated, error) {
	command := strings.TrimSpace(request.Command)
	if command == "" || strings.ContainsAny(command, " \t\r\n/\\;&|><$()") {
		return commandValidated{}, errors.New("command: command must be one allowlisted executable name without shell syntax")
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
			return commandValidated{}, fmt.Errorf("sandbox: %w", err)
		}
	}

	name := normalizeCommandName(command)
	if mode != domain.SandboxModeDangerFullAccess {
		if _, ok := b.allowed[name]; !ok {
			return commandValidated{}, fmt.Errorf("command: executable %q is not allowlisted", command)
		}
	}
	if len(request.Args) > 128 {
		return commandValidated{}, errors.New("command: too many arguments")
	}
	argsBytes := 0
	for _, arg := range request.Args {
		if len(arg) > 4096 {
			return commandValidated{}, errors.New("command: argument exceeds 4096 bytes")
		}
		argsBytes += len(arg)
		if strings.IndexByte(arg, 0) >= 0 {
			return commandValidated{}, errors.New("command: NUL in argument")
		}
	}
	if argsBytes > maxCommandArgsBytes {
		return commandValidated{}, errors.New("command: argument payload exceeds size limit")
	}
	scope, err := b.resolveCommandContext(ctx, runID, request.Cwd, request.TimeoutMS, request.Env)
	if err != nil {
		return commandValidated{}, err
	}
	args := append([]string(nil), request.Args...)
	if request.ApplyShellPrefix && b.shellPrefix != "" && b.shellPath != "" {
		// commandline opts into the configured prefix: the argv survives
		// verbatim through "$@"; only the trusted prefix string is shell.
		wrapped := append([]string{"-c", b.shellPrefix + " \"$@\"", shellPrefixArgv0, command}, args...)
		return commandValidated{command: "bash", args: wrapped, scope: scope}, nil
	}
	return commandValidated{command: command, args: args, scope: scope}, nil
}

// validateBashRequest is the bash-tool path: the sandbox command whitelist
// does not apply (the classifier deny table plus tiered approval own that
// risk), but read-only sandboxes still deny execution and the deny table is
// re-checked here as defense in depth.
func (b *CommandBackend) validateBashRequest(ctx context.Context, runID domain.RunID, request tools.CommandRequest, mode domain.SandboxMode) (commandValidated, error) {
	if b.shellPath == "" && goRuntime.GOOS != "windows" {
		return commandValidated{}, errors.New("command: bash is not available on this host")
	}
	if mode == domain.SandboxModeReadOnly {
		return commandValidated{}, fmt.Errorf("%w: command execution not allowed in read-only mode", ErrSandboxDenied)
	}
	if len(request.Args) != 2 || request.Args[0] != "-c" {
		return commandValidated{}, errors.New("command: bash expects a single -c script")
	}
	// The classifier always sees the caller's script alone; the configured
	// prefix is trusted operator config applied after classification.
	if class, findings, err := tools.ClassifyShellScript(request.Args[1]); err != nil {
		return commandValidated{}, err
	} else if class == tools.InvocationDenied {
		return commandValidated{}, fmt.Errorf("command: bash: %s", strings.Join(findings, "; "))
	}
	scope, err := b.resolveCommandContext(ctx, runID, request.Cwd, request.TimeoutMS, nil)
	if err != nil {
		return commandValidated{}, err
	}
	args := append([]string(nil), request.Args...)
	if b.shellPrefix != "" {
		args[1] = b.shellPrefix + " " + args[1]
	}
	return commandValidated{command: "bash", args: args, scope: scope}, nil
}

func (b *CommandBackend) resolveCommandContext(ctx context.Context, runID domain.RunID, cwdRequest string, timeoutMS int, envOverrides map[string]string) (commandScope, error) {
	if err := ctx.Err(); err != nil {
		return commandScope{}, err
	}
	if b.manager == nil {
		return commandScope{}, errors.New("command: workspace manager not wired")
	}
	workspace, err := b.manager.Ensure(ctx, runID)
	if err != nil {
		return commandScope{}, err
	}
	cwd := strings.TrimSpace(cwdRequest)
	if cwd == "" {
		cwd = "."
	}
	if filepath.IsAbs(cwd) {
		return commandScope{}, errors.New("command: cwd must be workspace-relative")
	}
	cwdPath := filepath.Join(workspace.Path, filepath.Clean(cwd))
	relativeCwd, err := filepath.Rel(workspace.Path, cwdPath)
	if err != nil || relativeCwd == ".." || strings.HasPrefix(relativeCwd, ".."+string(filepath.Separator)) || filepath.IsAbs(relativeCwd) {
		return commandScope{}, errors.New("command: cwd escapes workspace")
	}
	realCwd, err := filepath.EvalSymlinks(cwdPath)
	if err != nil {
		return commandScope{}, fmt.Errorf("command: resolve cwd: %w", err)
	}
	realWorkspace, _ := filepath.EvalSymlinks(workspace.Path)
	if !strings.EqualFold(filepath.Clean(realCwd), filepath.Clean(realWorkspace)) {
		if err := b.manager.ValidateRunPath(ctx, runID, realCwd); err != nil {
			return commandScope{}, errors.New("command: cwd symlink escapes workspace")
		}
	}
	info, err := os.Stat(realCwd)
	if err != nil || !info.IsDir() {
		return commandScope{}, errors.New("command: cwd is not a directory")
	}
	env, err := safeCommandEnv(envOverrides)
	if err != nil {
		return commandScope{}, err
	}
	env = appendRunLabels(ctx, env, runID)
	timeout := defaultCommandTimeout
	if timeoutMS > 0 {
		timeout = time.Duration(timeoutMS) * time.Millisecond
	}
	if timeout > b.maxTimeout {
		timeout = b.maxTimeout
	}
	spillID := tools.ToolCallIDFromContext(ctx)
	if spillID == "" {
		spillID = "run-" + string(runID)
	}
	return commandScope{
		cwd: realCwd, env: env, timeout: timeout,
		spillDir: filepath.Join(workspace.Path, ".vivy", "tool-output"),
		spillID:  spillID,
	}, nil
}

// appendRunLabels injects the run's identity labels into the sanitized
// process env. None of the values are secrets: session/run ids and the
// provider/model/thinking labels are operational metadata the spawned tool
// may legitimately observe. Values are flattened to one line.
func appendRunLabels(ctx context.Context, env []string, runID domain.RunID) []string {
	put := func(key, value string) {
		value = strings.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == 0 {
				return '_'
			}
			return r
		}, value)
		if value != "" {
			env = append(env, key+"="+value)
		}
	}
	put("VIVY_SESSION_ID", string(tools.SessionIDFromContext(ctx)))
	put("VIVY_RUN_ID", string(runID))
	labels := domain.RunLabelsFromContext(ctx)
	put("VIVY_PROVIDER", labels.Provider)
	put("VIVY_MODEL", labels.Model)
	if thinking := domain.ThinkingModeFromContext(ctx); thinking != "" {
		put("VIVY_THINKING", string(thinking))
	}
	return env
}

// shellPrefixArgv0 marks the commandline prefix wrap so the embedded-shell
// fallback can recognize and flatten it back into one script.
const shellPrefixArgv0 = "vivy-shell-prefix"

// shellQuoteJoin renders argv as a single-quoted POSIX command line for the
// embedded interpreter, which has no argv-preserving exec form.
func shellQuoteJoin(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(quoted, " ")
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
