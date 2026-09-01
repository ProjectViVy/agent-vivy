# VC-2 hooks: user-configurable pre_tool_use scripts (D8)

## What changed

Vivy gains user hook scripts: `runtime.hooks.pre_tool_use[]` in config.yaml
runs an operator command before each tool call, riding the existing
`ToolHookChain`. Protocol aligned with Claude Code (behavior only — Crush
is FSL-1.1-MIT and zero code was copied).

- **Config** (`internal/config`): `runtime.hooks.pre_tool_use[]` entries
  carry `matcher` (glob over tool name; empty/`*` = all), `command`
  (platform shell line: `cmd /c` on Windows, `sh -c` elsewhere),
  `timeout_ms` (0 = governance default; validated 0..1h hard cap), and
  `approved`. Validation rejects empty commands, out-of-range timeouts,
  and invalid globs at startup.
- **Ask gate (D8: first registration needs ask)**: an entry with
  `approved: false` is registered but **inert** — it never executes — and
  every startup logs loudly, naming the entry and the exact config fix
  (`approved is true`). Flipping the flag is the explicit human gesture;
  registration and arming are two separate decisions.
- **Protocol** (`internal/runtime/hooks_script.go`): stdin JSON payload
  (`hook_event_name`, `run_id`, `tool_name`, `tool_input`); exit 2 denies
  with stderr as the model-visible reason; exit 0 allows, optionally
  reading a stdout JSON envelope `{"decision":"deny","reason":…,
  "updated_input":{…}}` whose `updated_input` is shallow-merged
  (top-level keys replace) into the tool arguments; any other exit code,
  spawn failure, or timeout fails closed (matches chain semantics).
- **Matcher scoping** (`internal/runtime/hooks.go`): new `ToolMatcher`
  interface — hooks implementing it are skipped entirely (no invocation,
  no journal noise) for non-matching tools.
- **Journalling (D8: decisions into Journal)**: every armed-hook decision
  rides the existing governance events (`hook.started` / `hook.completed`
  / `hook.blocked`) persisted per run.
- **Wiring** (`internal/app`): `scriptHooksForConfig` arms approved
  entries into the chain at `app.New`; headless and web faces get the
  same surface (shared composition). Config.yaml changes apply at
  process start, like every other config.yaml surface.

## What was explicitly NOT done

- No settings.yaml overlay and no UI panel for hooks yet (hook edits are
  a config.yaml surface this slice); live-apply on settings save can ride
  a rebuilt chain later.
- Hook **configuration changes** are audited via the structured log
  (file sink) and the per-run governance events; a run-less config edit
  cannot be a Journal RunEvent (journal rows are run-scoped), so D8's
  "hook config changes go into Journal" is realized as: decisions per run
  into the Journal + config-change audit log. Recorded as a conscious
  reading of the ruling, not silently.
- No post-tool-use hook surface (protocol aligned to pre_tool_use only).

Tests: config parse/validation matrix; ScriptHook protocol matrix (12
tests: allow, exit-2 deny w/ and w/o stderr, fail-closed exits/spawn/
timeout, envelope rewrite shallow-merge, envelope deny, non-object
updated_input, non-JSON stdout, matcher chain skip); arming-gate test;
real platform-shell integration (`exit 2` / `exit 0`).
