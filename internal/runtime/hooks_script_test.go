package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	goRuntime "runtime"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

// fakeRunner returns a runner stub with the given outcome, capturing the
// stdin payload the hook piped to the (pretend) process.
type fakeRunner struct {
	gotStdin  []byte
	gotCmd    string
	stdout    string
	stderr    string
	exitCode  int
	err       error
	onTimeout bool
}

func (f *fakeRunner) run(ctx context.Context, command string, stdin []byte) ([]byte, []byte, int, error) {
	f.gotCmd = command
	f.gotStdin = append([]byte(nil), stdin...)
	if f.onTimeout {
		<-ctx.Done()
		return nil, nil, -1, ctx.Err()
	}
	return []byte(f.stdout), []byte(f.stderr), f.exitCode, f.err
}

var hookCall = ToolHookCall{
	RunID:     domain.RunID("run_hk"),
	ToolName:  "write_note",
	Arguments: json.RawMessage(`{"path":"a.ts","content":"x"}`),
}

func TestScriptHookAllowPassthrough(t *testing.T) {
	runner := &fakeRunner{exitCode: 0}
	hook := &ScriptHook{Command: "guard --tool bash", run: runner.run}
	result, err := hook.PreToolUse(context.Background(), hookCall)
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result.Decision != hookAllow || len(result.UpdatedArgs) != 0 {
		t.Fatalf("result = %+v, want plain allow", result)
	}
	var payload hookCallPayload
	if err := json.Unmarshal(runner.gotStdin, &payload); err != nil {
		t.Fatalf("stdin payload: %v", err)
	}
	if payload.HookEventName != "PreToolUse" || payload.ToolName != "write_note" || string(payload.RunID) != "run_hk" {
		t.Fatalf("payload = %+v", payload)
	}
	var input map[string]any
	if err := json.Unmarshal(payload.ToolInput, &input); err != nil || input["path"] != "a.ts" {
		t.Fatalf("tool_input = %s (err=%v)", payload.ToolInput, err)
	}
}

// scopedHookTestCommand checks a relative marker and the explicit environment
// through the same platform shell used by runHookProcessScoped.
func scopedHookTestCommand(marker string) string {
	if goRuntime.GOOS == "windows" {
		return `if exist ` + marker + ` (if x%HOOK_ALLOWED%==xallowed (if defined VIVY_HOOK_PARENT_SECRET (exit /b 1) else (exit /b 0)) else (exit /b 1)) else (exit /b 1)`
	}
	return `test -f '` + marker + `' && test "$HOOK_ALLOWED" = allowed && test -z "${VIVY_HOOK_PARENT_SECRET+x}"`
}

func TestScriptHookScopedExecutionUsesExplicitCwdAndEnvironment(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, ".hook-scope-marker"), []byte("present"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIVY_HOOK_PARENT_SECRET", "must-not-be-inherited")
	hook := NewScriptHook(scopedHookTestCommand(".hook-scope-marker"), "*", time.Second)
	hook.Cwd = cwd
	hook.Env = []string{"HOOK_ALLOWED=allowed"}
	hook.ContextDigest = strings.Repeat("a", 64)
	hook.runScoped = runHookProcessScoped
	if _, err := hook.PreToolUse(context.Background(), hookCall); err != nil {
		t.Fatalf("scoped Hook failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "must-not-exist")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scoped Hook unexpectedly wrote into its cwd: %v", err)
	}
	for _, fixture := range []struct {
		name string
		cwd  string
		env  []string
	}{
		{name: "wrong cwd", cwd: t.TempDir(), env: hook.Env},
		{name: "missing environment", cwd: cwd, env: []string{}},
		{name: "wrong environment", cwd: cwd, env: []string{"HOOK_ALLOWED=denied"}},
		{name: "inherited secret", cwd: cwd, env: []string{"HOOK_ALLOWED=allowed", "VIVY_HOOK_PARENT_SECRET=must-not-be-inherited"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			scoped := *hook
			scoped.Cwd, scoped.Env = fixture.cwd, fixture.env
			if result, err := scoped.PreToolUse(context.Background(), hookCall); !errors.Is(err, ErrHookBlocked) {
				t.Fatalf("invalid scope was accepted: %+v, %v", result, err)
			}
		})
	}
}

func TestLimitedHookBufferBoundsCapturedOutput(t *testing.T) {
	var buffer limitedHookBuffer
	payload := []byte(strings.Repeat("x", maxHookOutputBytes*2))
	if n, err := buffer.Write(payload); err != nil || n != len(payload) {
		t.Fatalf("Write = %d, %v; want %d, nil", n, err, len(payload))
	}
	if buffer.Len() != maxHookOutputBytes {
		t.Fatalf("captured hook output = %d bytes, want %d", buffer.Len(), maxHookOutputBytes)
	}
}

func TestScriptHookExit2DeniesWithStderr(t *testing.T) {
	runner := &fakeRunner{exitCode: 2, stderr: "  no writes on fridays \n"}
	hook := &ScriptHook{Command: "guard", run: runner.run}
	result, err := hook.PreToolUse(context.Background(), hookCall)
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result.Decision != hookDeny || result.Reason != "no writes on fridays" {
		t.Fatalf("result = %+v, want deny with stderr reason", result)
	}
}

func TestScriptHookExit2WithoutStderrGetsDefaultReason(t *testing.T) {
	runner := &fakeRunner{exitCode: 2}
	hook := &ScriptHook{Command: "guard", run: runner.run}
	result, err := hook.PreToolUse(context.Background(), hookCall)
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result.Decision != hookDeny || result.Reason == "" {
		t.Fatalf("result = %+v, want deny with default reason", result)
	}
}

func TestScriptHookOtherExitFailsClosed(t *testing.T) {
	runner := &fakeRunner{exitCode: 1, stderr: "boom"}
	hook := &ScriptHook{Command: "guard", run: runner.run}
	if _, err := hook.PreToolUse(context.Background(), hookCall); !errors.Is(err, ErrHookBlocked) {
		t.Fatalf("err = %v, want ErrHookBlocked", err)
	}
}

func TestScriptHookSpawnFailureFailsClosed(t *testing.T) {
	runner := &fakeRunner{err: errors.New("program not found")}
	hook := &ScriptHook{Command: "missing-program", run: runner.run}
	if _, err := hook.PreToolUse(context.Background(), hookCall); !errors.Is(err, ErrHookBlocked) {
		t.Fatalf("err = %v, want ErrHookBlocked", err)
	}
}

func TestScriptHookTimeoutFailsClosed(t *testing.T) {
	runner := &fakeRunner{onTimeout: true}
	hook := &ScriptHook{Command: "guard", Timeout: 20 * time.Millisecond, run: runner.run}
	if _, err := hook.PreToolUse(context.Background(), hookCall); !errors.Is(err, ErrHookBlocked) {
		t.Fatalf("err = %v, want ErrHookBlocked", err)
	}
}

func TestScriptHookEnvelopeRewriteShallowMerge(t *testing.T) {
	runner := &fakeRunner{exitCode: 0, stdout: `{"reason":"normalized","updated_input":{"path":"b.ts","extra":1}}`}
	hook := &ScriptHook{Command: "guard", run: runner.run}
	result, err := hook.PreToolUse(context.Background(), hookCall)
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result.Decision != hookRewrite {
		t.Fatalf("decision = %s, want rewrite", result.Decision)
	}
	var merged map[string]any
	if err := json.Unmarshal(result.UpdatedArgs, &merged); err != nil {
		t.Fatalf("merged args: %v", err)
	}
	if merged["path"] != "b.ts" || merged["content"] != "x" || merged["extra"] != float64(1) {
		t.Fatalf("merged = %#v, want override plus surviving keys", merged)
	}
}

func TestScriptHookEnvelopeDeny(t *testing.T) {
	runner := &fakeRunner{exitCode: 0, stdout: `{"decision":"DENY","reason":"policy says no"}`}
	hook := &ScriptHook{Command: "guard", run: runner.run}
	result, err := hook.PreToolUse(context.Background(), hookCall)
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result.Decision != hookDeny || result.Reason != "policy says no" {
		t.Fatalf("result = %+v, want envelope deny", result)
	}
}

func TestScriptHookUpdatedInputNotObjectFailsClosed(t *testing.T) {
	runner := &fakeRunner{exitCode: 0, stdout: `{"updated_input":[1,2]}`}
	hook := &ScriptHook{Command: "guard", run: runner.run}
	if _, err := hook.PreToolUse(context.Background(), hookCall); !errors.Is(err, ErrHookBlocked) {
		t.Fatalf("err = %v, want ErrHookBlocked", err)
	}
}

func TestScriptHookNonJSONStdoutIsAllowed(t *testing.T) {
	runner := &fakeRunner{exitCode: 0, stdout: "hook ran fine\n"}
	hook := &ScriptHook{Command: "guard", run: runner.run}
	result, err := hook.PreToolUse(context.Background(), hookCall)
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result.Decision != hookAllow {
		t.Fatalf("decision = %s, want allow", result.Decision)
	}
}

// TestScriptHookMatcherSkipsChain ensures a scoped hook is never invoked
// for tools outside its matcher — no events, no process, no journal noise.
func TestScriptHookMatcherSkipsChain(t *testing.T) {
	runner := &fakeRunner{exitCode: 1, stderr: "must not run"}
	scoped := &ScriptHook{Command: "guard", Matcher: "bash", run: runner.run}
	chain := NewToolHookChain(time.Second, scoped)
	args, err := chain.PreToolUse(context.Background(), ToolHookCall{ToolName: "write_note", Arguments: json.RawMessage(`{"a":1}`)})
	if err != nil {
		t.Fatalf("chain: %v", err)
	}
	if string(args) != `{"a":1}` {
		t.Fatalf("args = %s, want unchanged", args)
	}
	chain.PostToolUse(context.Background(), ToolHookCall{ToolName: "write_note"}, "ok", nil)
	if runner.gotCmd != "" {
		t.Fatal("scoped hook was invoked for a non-matching tool")
	}
}

// TestScriptHookRealProcessExit2 exercises the default platform-shell
// runner end to end: "exit 2" denies, "exit 0" allows.
func TestScriptHookRealProcessExit2(t *testing.T) {
	deny := NewScriptHook("exit 2", "", 0)
	result, err := deny.PreToolUse(context.Background(), hookCall)
	if err != nil {
		t.Fatalf("deny hook: %v", err)
	}
	if result.Decision != hookDeny {
		t.Fatalf("decision = %s, want deny", result.Decision)
	}
	allow := NewScriptHook("exit 0", "", 0)
	if _, err := allow.PreToolUse(context.Background(), hookCall); err != nil {
		t.Fatalf("allow hook: %v", err)
	}
	if deny.Name() != "script:exit" || !strings.HasPrefix(allow.Name(), "script:exit") {
		t.Fatalf("names = %q / %q", deny.Name(), allow.Name())
	}
}
