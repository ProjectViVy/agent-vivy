# Acceptance — VC-2 hooks (D8)

How a human can tell it works:

1. Add to config.yaml:
   ```yaml
   runtime:
     hooks:
       pre_tool_use:
         - matcher: bash
           command: "my-guard --strict"
   ```
   Start Vivy: the log contains a loud warning naming
   `pre_tool_use[0] (my-guard)` and saying it will not run until approved.
   The command never executes in this state.
2. Set `approved: true` on that entry and restart: the hook is armed.
3. With the armed hook, a matching tool call pipes
   `{"hook_event_name":"PreToolUse","tool_name":"bash","tool_input":{…}}`
   to the command's stdin. Exit 2 (stderr = reason) blocks the call and
   the model sees the reason; exit 0 allows; exit 0 with a stdout JSON
   envelope can deny or rewrite arguments via `updated_input` (top-level
   keys replace). Any other exit kills the call (fail closed).
4. Every armed decision appears in the run's journal as
   `hook.started` / `hook.completed` / `hook.blocked` events (visible via
   `run.log` and the UI event stream).
5. `matcher: "bash*"`-style globs scope the hook; non-matching tools never
   invoke it (no journal noise).
6. Invalid entries (empty command, bad glob, impossible timeout) fail
   startup with a targeted message instead of silently mis-configuring
   governance.

Contract notes: D8 ruling (ask on first registration, decisions into
Journal); Claude-Code protocol alignment is behavioral only — Crush is
FSL-1.1-MIT, zero code copied.
