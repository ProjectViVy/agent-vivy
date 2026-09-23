# ND-1 — Selective tool failure feedback

Goal (docs/plans/nudge/ND-1.md): "A supported failed invocation becomes an
unsuccessful tool result that allows same-Run correction." Classification
happens at the common governed adapter; per-call failure metadata is marked
on the ND-2 side channel; native fatal/interrupt paths are retained;
first-party MCP IsError crosses the internal error channel as a typed error.

## What changed

- `internal/runtime/tool_failure.go` — added the §5 vocabulary
  (status `recoverable|refused`; reasons `invalid_arguments`, `not_found`,
  `policy_denied`, `user_denied`, `command_failed`, `remote_tool_error`;
  effects `not_executed|none|unknown`) plus:
  - `classifyToolFailure(ctx, spec, err)` — the §5 allowlist. Parent
    ctx cancellation/deadline and native Eino interrupts are checked first
    and never classify. `*tools.ArgError` (with unwrap) → recoverable /
    invalid_arguments / unknown. `fs.ErrNotExist` under a **readonly** spec →
    recoverable / not_found / none; under a write spec it stays fatal.
    `*mcphost.ToolExecutionError` → recoverable / remote_tool_error / unknown.
    Everything else (transport, storage, policy, unknown) stays fatal.
  - `refusalFailure(reason, diagnostic)` — refusals are always
    not_executed; an empty reason falls back to policy_denied.
  - `markInvocationFailure(ctx, failure)` — publishes the record under
    `compose.GetToolCallID(ctx)` when a leg-bound `nudgeState` exists;
    missing state is a no-op (non-model governed-shell callers keep their
    error behavior); missing call ID inside a leg is the §5 invariant
    failure.
- `internal/runtime/tooladapter.go` —
  - `toolRefusal` gained `classification`; every refusal site assigns its
    vocabulary word explicitly (ValidateArgs → invalid_arguments, every
    safety/policy/plan/deny-table/rewrite/approval-eval site → policy_denied).
    The refusal branch marks the record before returning the existing
    refusal result.
  - All three human-denial sites (authorizeToolDispatch, middleware deny,
    approval deny) mark refused / user_denied / not_executed.
  - `invoke` converts an allowlisted invocation error into
    `MarkFailure(current call id) + bounded untrusted diagnostic + nil
    error` — but only when a nudge state is bound; non-model callers keep
    the original error.
  - `run` decodes the actual `tools.CommandResult` before framing for the
    reserved names (bash/execute/commandline); a nonzero exit marks
    command_failed / unknown while stdout/stderr/exit_code stay
    model-visible. Background launches carry no terminal exit and stay
    unmarked.
- `internal/runtime/pretool_bridge.go` — the middleware rewrite-deny
  refusal passes policy_denied.
- `internal/mcphost/tool_error.go` (new) — `ToolExecutionError{Text}`: a
  remote tool-result error on the internal error channel, distinct from
  JSON-RPC/transport failures.
- `internal/mcphost/toolworld.go` — `Invoke` returns `*ToolExecutionError`
  on IsError instead of flattening the remote text into a successful
  result.

## Deliberate non-changes

- No detector/Journal wiring beyond the ND-2 mark channel; no new public
  Port, no automatic replay, no extra model request.
- Non-model governed-shell paths retain their error behavior (conversion
  is gated on a bound `nudgeState` in ctx).
- `internal/app/assembly_tools.go`, `internal/toolhost/host.go`,
  `internal/tools/command.go`, `internal/runtime/command_backend.go` were
  read for the seam only — Host.Invoke already propagates the typed error
  unwrapped, so no changes were needed there.
