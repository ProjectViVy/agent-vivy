package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"
)

// ScriptHook is one user-registered hook script (D8) riding the
// Claude-Code-style protocol: the call payload is piped to the command's
// stdin as JSON; exit 2 denies the tool call with stderr as the reason;
// exit 0 allows, optionally reading a stdout JSON envelope
// {"decision":"deny","reason":"...","updated_input":{...}} whose
// updated_input is shallow-merged (top-level keys replace) into the tool
// arguments; any other exit code or spawn failure fails closed. The hook
// only ever runs after the operator marked its config entry approved.
type ScriptHook struct {
	// Command is the shell command line (platform shell: cmd /c on
	// Windows, sh -c elsewhere).
	Command string
	// Matcher is a glob over the tool name (path.Match semantics);
	// empty or "*" matches every tool. Non-matching tools skip the hook
	// entirely — the chain never invokes it for them.
	Matcher string
	// Timeout bounds one invocation; 0 keeps the chain's bound. The
	// effective bound is the shorter of the two.
	Timeout time.Duration

	// run is the process runner; tests inject a fake here. The default
	// pipes stdin through the platform shell and reports the exit code.
	run func(ctx context.Context, command string, stdin []byte) (stdout, stderr []byte, exitCode int, err error)
}

// NewScriptHook builds a hook from a validated config entry.
func NewScriptHook(command, matcher string, timeout time.Duration) *ScriptHook {
	return &ScriptHook{Command: command, Matcher: matcher, Timeout: timeout}
}

// Name identifies the hook in governance events: the command's leading
// word, enough to point an operator at the right config entry.
func (h *ScriptHook) Name() string {
	head := strings.TrimSpace(h.Command)
	if i := strings.IndexAny(head, " \t"); i >= 0 {
		head = head[:i]
	}
	if head == "" {
		head = "script"
	}
	return "script:" + head
}

// GovernanceIdentity binds approvals to the full script-hook generation
// without exposing its command in approval metadata or logs.
func (h *ScriptHook) GovernanceIdentity() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", h.Command, h.Matcher, h.Timeout)))
	return "script:" + hex.EncodeToString(sum[:])
}

// MatchesTool applies the configured glob (empty and "*" match all).
func (h *ScriptHook) MatchesTool(toolName string) bool {
	if h.Matcher == "" || h.Matcher == "*" {
		return true
	}
	ok, err := path.Match(h.Matcher, toolName)
	return err == nil && ok
}

// PostToolUse is a no-op: pre_tool_use hooks have no post phase.
func (h *ScriptHook) PostToolUse(context.Context, ToolHookCall, string, error) error { return nil }

// scriptHookEnvelope is the optional stdout document on exit 0.
type scriptHookEnvelope struct {
	Decision     string          `json:"decision"`
	Reason       string          `json:"reason"`
	UpdatedInput json.RawMessage `json:"updated_input"`
}

// hookCallPayload is the stdin document handed to the script.
type hookCallPayload struct {
	HookEventName string          `json:"hook_event_name"`
	RunID         string          `json:"run_id,omitempty"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
}

func (h *ScriptHook) PreToolUse(ctx context.Context, call ToolHookCall) (PreToolUseResult, error) {
	if len(call.Arguments) == 0 {
		call.Arguments = json.RawMessage(`{}`)
	}
	payload, err := json.Marshal(hookCallPayload{
		HookEventName: "PreToolUse",
		RunID:         string(call.RunID),
		ToolName:      call.ToolName,
		ToolInput:     call.Arguments,
	})
	if err != nil {
		return PreToolUseResult{}, fmt.Errorf("%w: %s: %v", ErrHookBlocked, h.Name(), err)
	}

	runner := h.run
	if runner == nil {
		runner = runHookProcess
	}
	if h.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}
	stdout, stderr, exitCode, runErr := runner(ctx, h.Command, payload)
	if runErr != nil {
		// Spawn failure or context death fails closed (the chain has
		// already attributed timeouts for us, but a shell that dies
		// before exec is indistinguishable from exit -1 here).
		if ctx.Err() != nil {
			return PreToolUseResult{}, fmt.Errorf("%w: %s timed out", ErrHookBlocked, h.Name())
		}
		return PreToolUseResult{}, fmt.Errorf("%w: %s: %v", ErrHookBlocked, h.Name(), runErr)
	}

	switch exitCode {
	case 0:
	default:
		reason := strings.TrimSpace(string(stderr))
		if reason == "" {
			reason = strings.TrimSpace(string(stdout))
		}
		if exitCode == 2 {
			if reason == "" {
				reason = "hook blocked execution (exit 2)"
			}
			return PreToolUseResult{Decision: hookDeny, Reason: reason}, nil
		}
		if reason == "" {
			reason = fmt.Sprintf("hook exited %d", exitCode)
		}
		return PreToolUseResult{}, fmt.Errorf("%w: %s: %s", ErrHookBlocked, h.Name(), reason)
	}

	var envelope scriptHookEnvelope
	if trimmed := bytes.TrimSpace(stdout); len(trimmed) > 0 && json.Valid(trimmed) {
		if err := json.Unmarshal(trimmed, &envelope); err != nil {
			return PreToolUseResult{}, fmt.Errorf("%w: %s returned invalid JSON envelope: %v", ErrHookBlocked, h.Name(), err)
		}
	}
	if strings.EqualFold(envelope.Decision, "deny") {
		reason := strings.TrimSpace(envelope.Reason)
		if reason == "" {
			reason = "hook denied execution"
		}
		return PreToolUseResult{Decision: hookDeny, Reason: reason}, nil
	}
	if len(envelope.UpdatedInput) > 0 {
		merged, err := shallowMergeArgs(call.Arguments, envelope.UpdatedInput)
		if err != nil {
			return PreToolUseResult{}, fmt.Errorf("%w: %s: %v", ErrHookBlocked, h.Name(), err)
		}
		return PreToolUseResult{Decision: hookRewrite, UpdatedArgs: merged, Reason: envelope.Reason}, nil
	}
	return PreToolUseResult{Decision: hookAllow, Reason: envelope.Reason}, nil
}

// shallowMergeArgs overlays the envelope's updated_input on top of the
// original tool arguments (top-level keys replace, others survive). Both
// sides must be JSON objects.
func shallowMergeArgs(args, updated json.RawMessage) (json.RawMessage, error) {
	var updatedMap map[string]any
	if err := json.Unmarshal(updated, &updatedMap); err != nil {
		return nil, fmt.Errorf("updated_input is not a JSON object: %v", err)
	}
	base := map[string]any{}
	if len(args) > 0 && json.Valid(args) {
		if err := json.Unmarshal(args, &base); err != nil {
			return nil, fmt.Errorf("original arguments are not a JSON object: %v", err)
		}
	}
	for key, value := range updatedMap {
		base[key] = value
	}
	merged, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("merge failed: %v", err)
	}
	return merged, nil
}

// runHookProcess pipes stdin through the platform shell and returns the
// captured stdout/stderr and exit code. A missing binary or a failed spawn
// surfaces as err; a real exit code (including non-zero) does not.
func runHookProcess(ctx context.Context, command string, stdin []byte) (stdout, stderr []byte, exitCode int, err error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Stdin = bytes.NewReader(stdin)
	var out, errBuf limitedHookBuffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return out.Bytes(), errBuf.Bytes(), exitErr.ExitCode(), nil
		}
		return out.Bytes(), errBuf.Bytes(), -1, err
	}
	return out.Bytes(), errBuf.Bytes(), 0, nil
}

const maxHookOutputBytes = 64 << 10

type limitedHookBuffer struct {
	bytes.Buffer
}

func (b *limitedHookBuffer) Write(p []byte) (int, error) {
	written := len(p)
	remaining := maxHookOutputBytes - b.Len()
	if remaining > 0 {
		if remaining < len(p) {
			_, _ = b.Buffer.Write(p[:remaining])
		} else {
			_, _ = b.Buffer.Write(p)
		}
	}
	// Report the complete write so os/exec keeps draining the pipe while the
	// retained audit material remains bounded.
	return written, nil
}
