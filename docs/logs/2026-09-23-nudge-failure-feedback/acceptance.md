# ND-1 acceptance mapping (docs/plans/nudge/ND-1.md)

| Plan requirement | Where it lands | Status |
|---|---|---|
| `classifyToolFailure(ctx, spec, err)` at the common governed adapter, not per tool | `internal/runtime/tool_failure.go`; invoked in `toolAdapter.invoke` | Done |
| Typed ArgError with unwrap → recoverable/invalid_arguments/unknown, model-visible diagnostic | classifier + invoke conversion; table test | Done |
| `fs.ErrNotExist` readonly invocation → recoverable/not_found/none; write invocation stays fatal | classifier (`spec.Readonly` gate); table test | Done |
| dispatch refusal → refused/invalid_arguments or policy_denied/not_executed | `toolRefusal.classification` set per site; refusal branch marks before returning | Done |
| human denial → refused/user_denied/not_executed | three denial sites mark explicitly | Done |
| reserved command nonzero exit → recoverable/command_failed/unknown, typed CommandResult intact | `markCommandFailure` decodes before framing; exit_code/stdout/stderr reach the model | Done |
| MCP IsError → recoverable/remote_tool_error/unknown via typed `mcphost.ToolExecutionError` | `tool_error.go` + `toolworld.Invoke` | Done |
| cancellation/deadline/native interrupt → NOT a failure, propagate before classification | classifier ordering + table test | Done |
| unknown error / storage / transport → fatal, same cause, no soft conversion | classifier default + `TestToolFailureTransportErrorStaysFatal` | Done |
| soft conversion: MarkFailure(current call id), untrusted diagnostic, nil error to Eino | `invoke` conversion branch | Done |
| missing call ID during a model-originated conversion = invariant failure | `MarkFailure("")` fails closed; gated test | Done |
| non-model shell paths keep existing error behavior | conversion gated on bound `nudgeState` | Done |
| `TestToolFailureCorrection` scripted-model arc | service-level test, same-Run recovery | Done |
| refusal test (denied writes never execute) | `TestToolFailureRefusalNeverExecutes` | Done |
| unknown-effect write test (invocation count is one) | `TestToolFailureUnknownEffectsCountOnce` | Done |
| test through Host.Invoke and governedTool, not only the constructor | contract harness drives `svc.Run` → engine → governedTool → adapter | Done |
| JSON-RPC/transport errors stay distinct | `TestToolWorldTransportErrorStaysUntyped` + fatal-path test | Done |
| `just ci` | full gate green | Done |

Commit: `feat: preserve recoverable tool failure feedback` (ND-1).
