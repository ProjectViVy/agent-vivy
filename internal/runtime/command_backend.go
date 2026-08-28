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
	"strings"
	"sync"
	"time"

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

type EinoCommandBackend struct {
	manager        *WorkspaceManager
	sandbox        *SandboxManager
	allowed        map[string]struct{}
	maxOutputBytes int
	maxTimeout     time.Duration
}

var _ tools.CommandOperations = (*EinoCommandBackend)(nil)
var _ interface {
	PrepareCommand(context.Context, domain.RunID, tools.CommandRequest) (domain.ToolProposal, error)
} = (*EinoCommandBackend)(nil)

// NewEinoCommandBackend wires the local process backend. maxTimeout bounds a
// single execute/commandline run (from runtime.execute_max_timeout_seconds);
// non-positive falls back to the 30s default and values above
// hardMaxCommandTimeout are clamped to it.
func NewEinoCommandBackend(manager *WorkspaceManager, sandbox *SandboxManager, allowed []string, maxTimeout time.Duration) *EinoCommandBackend {
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
	return &EinoCommandBackend{manager: manager, sandbox: sandbox, allowed: commands, maxOutputBytes: maxCommandOutput, maxTimeout: maxTimeout}
}
func (b *EinoCommandBackend) Execute(ctx context.Context, runID domain.RunID, request tools.CommandRequest) (tools.CommandResult, error) {
	command, args, cwd, env, timeout, err := b.validateRequest(ctx, runID, request)
	if err != nil {
		return tools.CommandResult{}, err
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return tools.CommandResult{}, fmt.Errorf("command: executable %q is unavailable: %w", command, err)
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
func (b *EinoCommandBackend) PrepareCommand(ctx context.Context, runID domain.RunID, request tools.CommandRequest) (domain.ToolProposal, error) {
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

func (b *EinoCommandBackend) validateRequest(ctx context.Context, runID domain.RunID, request tools.CommandRequest) (string, []string, string, []string, time.Duration, error) {
	command := strings.TrimSpace(request.Command)
	if command == "" || strings.ContainsAny(command, " \t\r\n/\\;&|><$()") {
		return "", nil, "", nil, 0, errors.New("command: command must be one allowlisted executable name without shell syntax")
	}

	mode := sandboxMode(ctx)
	if !mode.Valid() && b.sandbox != nil {
		mode = b.sandbox.Mode()
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
	if err := ctx.Err(); err != nil {
		return "", nil, "", nil, 0, err
	}
	if b.manager == nil {
		return "", nil, "", nil, 0, errors.New("command: workspace manager not wired")
	}
	workspace, err := b.manager.Ensure(ctx, runID)
	if err != nil {
		return "", nil, "", nil, 0, err
	}
	cwd := strings.TrimSpace(request.Cwd)
	if cwd == "" {
		cwd = "."
	}
	if filepath.IsAbs(cwd) {
		return "", nil, "", nil, 0, errors.New("command: cwd must be workspace-relative")
	}
	cwdPath := filepath.Join(workspace.Path, filepath.Clean(cwd))
	relativeCwd, err := filepath.Rel(workspace.Path, cwdPath)
	if err != nil || relativeCwd == ".." || strings.HasPrefix(relativeCwd, ".."+string(filepath.Separator)) || filepath.IsAbs(relativeCwd) {
		return "", nil, "", nil, 0, errors.New("command: cwd escapes workspace")
	}
	realCwd, err := filepath.EvalSymlinks(cwdPath)
	if err != nil {
		return "", nil, "", nil, 0, fmt.Errorf("command: resolve cwd: %w", err)
	}
	realWorkspace, _ := filepath.EvalSymlinks(workspace.Path)
	if !strings.EqualFold(filepath.Clean(realCwd), filepath.Clean(realWorkspace)) {
		if err := b.manager.ValidatePath(realCwd); err != nil {
			return "", nil, "", nil, 0, errors.New("command: cwd symlink escapes workspace")
		}
	}
	info, err := os.Stat(realCwd)
	if err != nil || !info.IsDir() {
		return "", nil, "", nil, 0, errors.New("command: cwd is not a directory")
	}
	env, err := safeCommandEnv(request.Env)
	if err != nil {
		return "", nil, "", nil, 0, err
	}
	timeout := defaultCommandTimeout
	if request.TimeoutMS > 0 {
		timeout = time.Duration(request.TimeoutMS) * time.Millisecond
	}
	if timeout > b.maxTimeout {
		timeout = b.maxTimeout
	}
	return command, append([]string(nil), request.Args...), realCwd, env, timeout, nil
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
