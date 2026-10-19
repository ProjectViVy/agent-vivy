package runtime

import (
	"context"
	"errors"
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
		command   string
		decision  hookDecision
		reason    string
		rewritten string
		blocked   bool
	}{
		{name: "allow", command: `cat >/dev/null`, decision: hookAllow},
		{name: "exit two denies", command: `cat >/dev/null; echo blocked-by-policy >&2; exit 2`, reason: "blocked-by-policy", blocked: true},
		{name: "other exit fails closed", command: `cat >/dev/null; exit 1`, blocked: true},
		{name: "non json output allows", command: `cat >/dev/null; printf 'hook ran fine\n'`, decision: hookAllow},
		{name: "envelope rewrite", command: `cat >/dev/null; printf '%s' '{"reason":"normalized","updated_input":{"command":"echo rewritten"}}'`, decision: hookRewrite, rewritten: "echo rewritten"},
		{name: "invalid updated input fails closed", command: `cat >/dev/null; printf '%s' '{"updated_input":[1,2]}'`, blocked: true},
		{name: "timeout fails closed", command: `cat >/dev/null; sleep 2`, blocked: true},
		{name: "missing executable fails closed", command: `vivy-toolkit-no-such-hook`, blocked: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			t.Setenv("VIVY_TOOLKIT_HOOK_TEST_BINDING", "allowed")
			manager, backend, view, hook := newBundleHookFixture(t, tc.command)
			resolver := BundleHookExecutionResolverFunc(manager.ResolveHookExecution)
			dispatcher := NewBundleHookDispatcher(backend.BundleStore(), resolver, 2*time.Second)
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
			result, err := dispatcher.PreToolUse(captured, ToolHookCall{ToolName: "bash", Arguments: []byte(`{"command":"true"}`)})
			if tc.blocked {
				if !errors.Is(err, ErrHookBlocked) {
					t.Fatalf("dispatched %q = %+v, %v, want ErrHookBlocked", tc.command, result, err)
				}
				if tc.reason != "" && !strings.Contains(err.Error(), tc.reason) {
					t.Fatalf("blocked reason = %q, want %q", err, tc.reason)
				}
				return
			}
			if err != nil {
				t.Fatalf("dispatched %q = %v", tc.command, err)
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
