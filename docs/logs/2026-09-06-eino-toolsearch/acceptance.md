# Acceptance

The migration is accepted when the following observable paths hold:

1. With only fixed-visible tools, the model receives no official
   `tool_search` meta-tool. With at least one other allowlisted active tool,
   the first model call receives the fixed core plus exactly one official
   `tool_search`; a deterministic search result makes the selected tool
   available on the next call and it executes once.
2. A Skill mount starts hidden, mounts through the existing Skill operation,
   and appears on the following model call even if the previous Eino state
   omitted its `ToolInfo`. Reapplying the projection does not duplicate it.
3. Legacy config/settings lists lose `tool_search` at load/save/update while
   preserving the order of other names. A legacy-only list remains a non-nil
   empty active list, and an explicit empty `tools.enabled` loads as chat-only.
4. A business tool named `tool_search` is rejected as a reserved-name
   conflict; the builtin catalog and static registry do not register the
   retired custom tool.

The deterministic Runner, mount, config, settings, registry, and app overlay
tests exercise these paths without a live provider or network dependency.
