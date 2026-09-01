package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
)

// TestScriptHooksArmingGate verifies D8's ask gate: unapproved entries are
// registered-but-inert with a loud warning, approved entries arm as
// ScriptHooks carrying the configured matcher and timeout.
func TestScriptHooksArmingGate(t *testing.T) {
	cfg := config.Config{}
	cfg.Runtime.Hooks.PreToolUse = []config.PreToolUseHook{
		{Matcher: "bash", Command: "guard --strict", TimeoutMs: 2500, Approved: true},
		{Command: "second-guard"},
		{Command: "third-guard", Approved: true},
	}
	var warnings []string
	hooks := scriptHooksForConfig(cfg, func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	})
	if len(hooks) != 2 {
		t.Fatalf("armed hooks = %d, want 2 (only approved entries)", len(hooks))
	}
	first, ok := hooks[0].(*runtime.ScriptHook)
	if !ok {
		t.Fatalf("hooks[0] is %T, want *runtime.ScriptHook", hooks[0])
	}
	if first.Command != "guard --strict" || first.Matcher != "bash" || first.Timeout != 2500*time.Millisecond {
		t.Fatalf("armed hook = %+v", first)
	}
	if !first.MatchesTool("bash") || first.MatchesTool("write_note") {
		t.Fatal("matcher not applied to armed hook")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "pre_tool_use[1]") ||
		!strings.Contains(warnings[0], "second-guard") {
		t.Fatalf("warnings = %#v, want one naming pre_tool_use[1] and its command word", warnings)
	}
}
