package app

import (
	"strings"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
)

// scriptHooksForConfig arms the approved hook entries (D8: first
// registration needs ask). An unapproved entry is registered-but-inert —
// it never executes — and every startup says so loudly, naming the entry
// by index and leading command word so the operator can find it in
// config.yaml and flip approved: true deliberately.
func scriptHooksForConfig(cfg config.Config, warnf func(format string, args ...any)) []runtime.ToolHook {
	hooks := make([]runtime.ToolHook, 0, len(cfg.Runtime.Hooks.PreToolUse))
	for i, entry := range cfg.Runtime.Hooks.PreToolUse {
		if !entry.Approved {
			head := strings.TrimSpace(entry.Command)
			if j := strings.IndexAny(head, " \t"); j >= 0 {
				head = head[:j]
			}
			warnf("hook pre_tool_use[%d] (%s) is registered but not approved; it will not run "+
				"until runtime.hooks.pre_tool_use[%d].approved is true in config.yaml", i, head, i)
			continue
		}
		hooks = append(hooks, runtime.NewScriptHook(entry.Command, entry.Matcher, time.Duration(entry.TimeoutMs)*time.Millisecond))
	}
	return hooks
}
