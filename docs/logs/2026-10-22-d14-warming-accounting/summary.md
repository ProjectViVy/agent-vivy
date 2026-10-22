# D14 paid warming accounting and lifecycle correction

Paid cache refreshes now use the existing Run model-call observer and shared
BudgetLedger. Native Generate reserves admission first, records source
`maintenance`, and settles reported prompt/completion usage and finish even when
the provider returns a message with an error or cancellation. Maintenance cannot
schedule another warmer or produce transcript deltas/assistant completions. The
existing off/streaming/idle modes, capability/savings gates, token floor,
16-token output limit and 60-second timeout remain.

Mandatory observation failures propagate to the owning Run and wake a blocked
foreground provider. Drive legs stop admission, cancel the scheduler and join
admitted paid work before finalization, suspension, steering or WaitIdle. Failed
usage persistence still attempts finish. Exhausted diagnostic event quota may
skip its marker; admitted paid settlement remains mandatory.

Steering preserves the admitted Run context, policy, ledger, engine, tool state
and checkpoint. Replacement legs receive fresh observers, warmers and native ADK
cancel handles, and register with the Service wait group. The original Run Go
cancellation owner remains authoritative. Replay excludes maintenance from
foreground completion credits. Trajectory gives concurrent maintenance independent
lifecycle rows without interrupting foreground calls, resetting their deltas,
advancing their step or claiming their assistant text.

## Architecture and Eino capability check

This implements D14's correction under D05 and the shared deletion design C11 /
section 11.5. The shipped baseline remains; plugin extraction belongs to #93.
No second runner, ledger, Journal, policy owner or converter was introduced.

Inspected pinned Eino v0.9.21 `components/model/interface.go`: native
`BaseModel.Generate`, `BaseModel.Stream` and `AgenticModel` remain the boundary.
Refresh uses Generate, `model.WithMaxTokens` and existing tool binding. Inspected
`adk/cancel.go` and Engine.Resume options: replacement legs use native
`adk.WithCancel` and Eino's existing checkpoint/safe-boundary behavior. The
existing Vivy observer adapter supplies product-owned budget admission and
mandatory Journal settlement, which a raw provider call does not supply. No Eino
type escaped runtime/provider.

The existing request/usage/finish schemas accept source `maintenance`. Storage
usage projection already supports call-ID/source attribution and was exercised
with actual events. Face/TUI/UI source-consumer searches found no additional
closed source enum requiring changes.

## Scope limits

No tenant Journal, production database, live provider, schema migration, plugin
extraction, UI implementation, release or performance claim was included.
Independent Linux wiring/kernel/race gates and final D14 acceptance remain with
the integration owner; the human real-model check is separate.
