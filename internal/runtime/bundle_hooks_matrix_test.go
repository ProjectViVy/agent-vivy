package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	goRuntime "runtime"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/bundles"
)

// TestToolkitCapturedHookFailureMatrix runs every PreToolUse outcome through a
// captured run whose Hook is armed by exact consent, including the real
// platform shell, not just the ScriptHook unit primitives.
func TestToolkitCapturedHookFailureMatrix(t *testing.T) {
	cases := []struct {
		name      string
		mode      string
		decision  hookDecision
		reason    string
		rewritten string
		blocked   bool
	}{
		{name: "allow", mode: "allow", decision: hookAllow},
		{name: "exit two denies", mode: "deny", reason: "blocked-by-policy", blocked: true},
		{name: "other exit fails closed", mode: "exit-one", reason: "hook exited 1", blocked: true},
		{name: "non json output allows", mode: "non-json", decision: hookAllow},
		{name: "envelope rewrite", mode: "rewrite", decision: hookRewrite, rewritten: "echo rewritten"},
		{name: "invalid updated input fails closed", mode: "invalid-input", reason: "updated_input is not a JSON object", blocked: true},
		{name: "timeout fails closed", mode: "timeout", reason: "timeout-fixture-waiting", blocked: true},
		{name: "missing executable fails closed", mode: "missing", reason: "vivy-toolkit-no-such-hook", blocked: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			t.Setenv("VIVY_TOOLKIT_HOOK_TEST_BINDING", "allowed")
			command := toolkitHookTestCommand(t, tc.mode)
			manager, backend, view, hook := newBundleHookFixture(t, command)
			resolver := BundleHookExecutionResolverFunc(manager.ResolveHookExecution)
			// Leave enough chain budget for the timeout helper to return
			// success if the declared one-second Hook timeout is not enforced.
			dispatcher := NewBundleHookDispatcher(backend.BundleStore(), resolver, 4*time.Second)
			arm, err := manager.SetHookConsent(ctx, bundles.HookConsentRequest{
				OperationID: "82000000-0000-4000-8000-000000000001", Expected: view.Identity(), Ref: hook.Ref,
				ExpectedRevision: 0, Subject: "test-operator", Allowed: true,
			})
			if err != nil || arm.State != "committed" {
				t.Fatalf("arm Hook = %+v, %v", arm, err)
			}
			handle, _, check, err := manager.CaptureRun(ctx, "run-hook-matrix")
			if err != nil {
				t.Fatal(err)
			}
			defer handle.Release()
			captured := withToolkitWorkspaceRoot(withBundleCapture(withBundleView(ctx, handle.View()), check), t.TempDir(), true, nil)
			result, err := dispatcher.PreToolUse(captured, ToolHookCall{RunID: "run-hook-matrix", ToolName: "bash", Arguments: []byte(`{"command":"true"}`)})
			if tc.blocked {
				if !errors.Is(err, ErrHookBlocked) {
					t.Fatalf("dispatched %q = %+v, %v, want ErrHookBlocked", command, result, err)
				}
				if tc.reason != "" && !strings.Contains(err.Error(), tc.reason) {
					t.Fatalf("blocked reason = %q, want %q", err, tc.reason)
				}
				return
			}
			if err != nil {
				t.Fatalf("dispatched %q = %v", command, err)
			}
			if result.Decision != tc.decision {
				t.Fatalf("decision = %s, want %s (%+v)", result.Decision, tc.decision, result)
			}
			if tc.reason != "" && !strings.Contains(result.Reason, tc.reason) {
				t.Fatalf("reason = %q, want %q", result.Reason, tc.reason)
			}
			if tc.rewritten != "" && !strings.Contains(string(result.UpdatedArgs), tc.rewritten) {
				t.Fatalf("updated args = %s, want %q", result.UpdatedArgs, tc.rewritten)
			}
		})
	}
}

const toolkitHookHelperFlag = "-test.run=TestToolkitHookHelperProcess"

// toolkitHookTestCommand invokes this test binary through the production
// platform shell. Only the executable needs shell quoting; modes are fixed
// test-owned tokens. Disable the race detector's exit delay in the child so it
// does not consume the Hook's one-second execution budget after a clean exit.
func toolkitHookTestCommand(t *testing.T, mode string) string {
	t.Helper()
	if mode == "missing" {
		return "vivy-toolkit-no-such-hook"
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := " " + toolkitHookHelperFlag + " -- " + mode
	if goRuntime.GOOS == "windows" {
		return `set "GORACE=atexit_sleep_ms=0" && "` + executable + `"` + args
	}
	return `GORACE=atexit_sleep_ms=0 exec '` + strings.ReplaceAll(executable, "'", `'"'"'`) + `'` + args
}

// TestToolkitHookHelperProcess is entered only by the real Hook subprocess.
// Validate stdin and the resolved binding before producing any outcome, so
// missing input or broken process setup cannot silently satisfy the matrix.
func TestToolkitHookHelperProcess(t *testing.T) {
	if len(os.Args) != 4 || os.Args[1] != toolkitHookHelperFlag || os.Args[2] != "--" {
		return
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatalf("read Hook stdin: %v", err)
	}
	var payload hookCallPayload
	if err := json.Unmarshal(input, &payload); err != nil {
		t.Fatalf("decode Hook stdin: %v", err)
	}
	if payload.HookEventName != "PreToolUse" || payload.RunID != "run-hook-matrix" || payload.ToolName != "bash" || string(payload.ToolInput) != `{"command":"true"}` {
		t.Fatalf("unexpected Hook stdin: %s", input)
	}
	if got := os.Getenv("HOOK_ALLOWED"); got != "allowed" {
		t.Fatalf("resolved Hook binding = %q, want allowed", got)
	}

	var stdout string
	switch mode := os.Args[3]; mode {
	case "allow":
	case "deny":
		if _, err := io.WriteString(os.Stderr, "blocked-by-policy\n"); err != nil {
			t.Fatalf("write Hook stderr: %v", err)
		}
		os.Exit(2)
	case "exit-one":
		os.Exit(1)
	case "non-json":
		stdout = "hook ran fine\n"
	case "rewrite":
		stdout = `{"reason":"normalized","updated_input":{"command":"echo rewritten"}}`
	case "invalid-input":
		stdout = `{"updated_input":[1,2]}`
	case "timeout":
		if _, err := io.WriteString(os.Stderr, "timeout-fixture-waiting\n"); err != nil {
			t.Fatalf("write Hook timeout marker: %v", err)
		}
		// Deliberately outlive the declared one-second Hook deadline, then
		// exit successfully before the four-second chain deadline. Only a
		// real Hook timeout should block this case, with the marker proving
		// the child consumed stdin and reached its timeout stimulus.
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		<-timer.C
	default:
		t.Fatalf("unknown Hook helper mode %q", mode)
	}
	if _, err := io.WriteString(os.Stdout, stdout); err != nil {
		t.Fatalf("write Hook stdout: %v", err)
	}
	// Do not let the test harness append PASS to the protocol stdout.
	os.Exit(0)
}
