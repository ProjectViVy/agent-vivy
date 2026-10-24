# Tool Failure Recovery and Nudge — Detailed Design

Revision **D04 producer boundary**, 2026-10-21. Issue [#58](https://github.com/ProjectViVy/agent-vivy/issues/58).
Baseline: `a8d361b0244a1c40be513622bbdaebb5c9d40014` (local and fetched main).
Delivery authority: [plan index](../archive/plans/nudge/README.md). This engineering specification supersedes the preliminary issue comment where explicitly refined below. D04 updates the shipped interim core implementation; #93 owns ordinary first-party plugin extraction.

## 1. Intent and requirements

The requested capability is correction after failed tool calls within the current Run.

| ID | Contract | Acceptance owner |
| --- | --- | --- |
| N1 | Allowlisted tool failures become unsuccessful model-visible results; the model can choose a corrected permitted action in the same Run. | ND-1, ND-4 |
| N2 | Unsuccessful identical completed calls receive advisory reminders at counts 3 and 5 within a ten-call window; count 6 stops further model work. | ND-2, ND-3, ND-4 |
| N3 | Policy, approval, cancellation, Journal durability, tool/iteration/budget limits and uncertain effects remain authoritative. | All Stories |
| N4 | Journal records unsuccessful calls and reminder scheduling; traces distinguish scheduling from provider handoff. | ND-2, ND-3, ND-4 |
| N5 | Bounded state, direct/enhanced parity, concurrent calls, compaction and resume have explicit behavior. | ND-0, ND-2, ND-3, ND-4 |

Exclude human steering/RPC, Goal continuation or verification, automatic tool retries, semantic task evaluation, new public plugin Ports, new dependencies and general guard frameworks. No dependency on #47's implementation. Model correction is an opportunity, not a guarantee of eventual task success.

## 2. Decision and alternatives

Use the existing tool adapter for selective failure adaptation, one run-local observation object shared with the mapper for repetition/audit, and an Eino ADK model wrapper for transient delivery after durable results settle. Reuse the existing loop window, limits, Service lifecycle and Journal. Prompt-only changes cannot recover a propagated tool error; a separate controller/verifier adds lifecycle and model calls. A new ToolWorld result field would change the public SDK: carry the first-party MCP failure through a typed internal error instead. The only new state is bounded in-flight observation and one pending reminder. No independent scheduler, retry engine or storage table is needed.

## 3. Source evidence and Eino check

| Source | Verified fact / consequence |
| --- | --- |
| `internal/runtime/tooladapter.go`, `InvokableRun`, `dispatch`, `invoke` | Refusals already return results; ordinary errors propagate; redactedToolError preserves Unwrap. Classify only at the known validation/execution boundary, not arbitrary infrastructure errors. |
| `internal/runtime/enhanced_tooladapter.go` | Enhanced invocation delegates to the ordinary adapter, then normalizes result parts. Recovery must occur before normalization and preserve valid media envelopes. |
| `internal/mcphost/toolworld.go`, `ToolWorld.Invoke` | IsError currently becomes a text prefix, losing typed identity. |
| `internal/toolhost/host.go` and `internal/app/assembly_tools.go` | Dynamic provider result/error passes to governedTool; an internal typed error can reach the runtime without changing SDK schemas. |
| `internal/runtime/loopguard.go`, `mapper.go` | Ten entries, same tool/arguments/result/error, count greater than five stops; state resets on resume. Current sixth result batch is discarded. |
| `internal/runtime/service.go`, `withLiveModelStreamObserver`; `model_stream_observer.go` | Existing waitForToolsSettled occurs while consuming output chunks, after inner.Stream has been called. It is not a request-before-send barrier. |
| `schemas/events/payloads/tool.finished.json` | Strict additionalProperties=false: additive payload fields require schema updates, not just Go structs. |
| `ui/src/lib/run-rows.ts` | Existing error projection consumes tool.finished.error; preserve this field. |

Pinned Eino v0.9.21; EinoExt Claude v0.1.25, OpenAI v0.1.13, MCP v0.0.9. Inspected upstream `adk/handler.go`, `adk/chatmodel.go`, `adk/chatmodel_test.go`, `compose/tool_node.go`.

- `compose.GetToolCallID(ctx)` is the correlation source in runtime adapters. Do not match parallel results solely by tool name.
- `adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage].WrapModel` returns `model.BaseModel[*schema.AgenticMessage]` and wraps both Generate and Stream. Eino owns tool binding separately.
- `BeforeModelRewriteState` persists message edits in ADK state. We deliberately choose WrapModel for a **single-request transient reminder** to avoid replaying synthetic instructions after resume/compaction. Upstream discourages general message rewriting here because it is not persisted; that property is intentional for this bounded trailing reminder. Stable instruction prefix and tool catalog remain untouched.
- `adk.TypedChatModelAgentMiddleware.WrapInvokableToolCall` covers Eino-owned readonly metadata tools (`tool_search`, `skill`) at their producer boundary. Ordinary and enhanced governed tools retain their existing common adapter. `compose.ToolMiddleware` also supports ordinary and enhanced invocation. It is available but not needed for broad error catching: the existing ordinary adapter is the common governed boundary.
- UnknownToolsHandler is available but is **not enabled by this design**. Hallucinated-tool recovery is adjacent scope and currently stays fatal; naming an available upstream hook is not a requirement to use it.
- Native interrupts, Runner checkpoints and MaxIterations are retained.

Custom policy is needed for Vivy's recoverable failure allowlist and Journal ordering. It lives within runtime, with a first-party MCP error carrier in mcphost. Eino stays quarantined to runtime/provider. Replace the small adapter if upstream offers equivalent scoped semantics later.

D04 rechecked `compose.NewAgenticToolsNode` / native ToolsNode completion and its completed-sibling checkpoint reuse, `compose.GetToolCallID`, typed `WrapModel`, and `adk.TypedRunner.ResumeWithParams` at v0.9.21. Native ToolsNode already joins completed calls; it does not own Vivy database claims or typed durable results. D03's existing coordinator supplies those records. No second join, outcome service, executor or checkpoint format is needed. The legacy ND-0 test-local barrier is retired; current tests exercise the actual native producer and Service path.

## 4. Internal contracts (proposed unless already present)

All new names below are package-private except the MCP error carrier consumed across existing internal packages. The advisory SDK/core factory seam is inert; public RPC stays unchanged.

```go
// internal/runtime/tool_failure.go
// Status: "recoverable" or "refused". Absence means ordinary success.
type toolFailure struct {
    Status string
    Reason string
    Diagnostic string // redacted, bounded; never raw credentials
    Effects string // "not_executed", "none", "unknown"
}
func classifyToolFailure(ctx context.Context, spec domain.ToolSpec, err error) (toolFailure, bool)

// internal/mcphost/tool_error.go
// This is a remote *tool result* error, not MCPRemoteError (JSON-RPC error).
type ToolExecutionError struct { Text string }
func (e *ToolExecutionError) Error() string

// internal/runtime/nudge_state.go
// Actual Run ID is already carried by context; instances are per drive/resume.
type completedCall struct {
    ID, Name, ArgsJSON, Result, Error string
    Failure *toolFailure
}
type nudgeNotice struct {
    CallID, ToolName, Reason string
    Count int
    TemplateVersion string
}
func newNudgeState() *nudgeState
func (s *nudgeState) MarkFailure(id string, failure toolFailure) error
func (s *nudgeState) Failure(id string) (toolFailure, bool)
func (s *nudgeState) Take(ctx context.Context, nativeIDs []string) (*nudgeNotice, bool, error)
func (s *nudgeState) Abort(err error)
```

Tool producers record a leg-local coverage projection before returning to Eino.
An operation-backed entry contains only its ID/name and freshness, recorded after
`CompleteToolOperation` atomically commits the typed result and lifecycle event.
At `WrapModel`, the existing coordinator resolves that immutable row. Work and
pre-claim refusals use bounded explicitly typed producer values under their
existing effect owner; no second operation is claimed. Classification never
comes from parsing a result prefix.

`Take` synchronously validates coverage for the entire ordered native batch,
then advances only fresh samples in native request order. Completed siblings
reused directly by Eino are registered as non-fresh coverage on resume. A missing
producer fails closed; there is no wait, consumer acknowledgment or parked
completion. The entire ordered ID batch and selected notice are memoized, even
when every sibling is non-fresh, so retries count/schedule once. Schedule failure
aborts the state with the original cause, preventing unscheduled retry injection.

One fresh detector belongs to each drive/resume/steer leg. The sole window holds
ten exact `(tool, canonical arguments, bounded result, bounded error)` tuples;
field separators cannot alias. Canonical JSON preserves number literals; malformed-JSON refusals retain exact raw argument bytes. Producer argument/result/error values and canonical expansion are capped at 1 MiB, failing closed without truncating tuple identity.
Success advances the same window; failure selects reminders at counts 3/5;
count 6 retains the existing hard stop. No hash, historical ID set or durable
nudge cache is added. The mapper's separate observed-stream settlement ordering
remains because it protects output accumulation, independent of model entry.

The inert `sdk/port/loopexp` policy DTOs and optional closed
`core/nudge-factory@v1` slot (sole T1 owner `vivy/nudge`) prepare #93. No
provider is selected or constructed here; the interim core remains the only
coaching implementation. The later cutover must delete that core body when it
starts executing the ordinary plugin, including the fused hard-stop decision.

## 5. Selective recovery and provenance

| Origin | Classification | Effects / behavior |
| --- | --- | --- |
| Existing dispatch validation refusal | refused / invalid_arguments | not_executed; existing result remains readable |
| Existing policy, safety or human refusal | refused / policy_denied or user_denied | not_executed; explicit boundary-respecting reminder |
| Per-call governance refusal: deny table, plan-mode denial, a tool outside the run selection, tool-hook `deny`, a public-fetch target outside the internet surface, or a work action this run is not admitted for | refused / policy_denied | not_executed; the refusal text becomes the tool result and the run continues |
| `*tools.ArgError` from actual tool invocation | recoverable / invalid_arguments | unknown unless known validation occurred before dispatch; preserve unwrap |
| `fs.ErrNotExist` from invocation of a readonly tool | recoverable / not_found | none; no global storage-error catch |
| Known execute/bash CommandResult with nonzero exit | recoverable / command_failed | unknown; preserve exit code/output; never call it a safe-to-retry result |
| First-party MCP IsError converted to ToolExecutionError | recoverable / remote_tool_error | unknown; preserve bounded untrusted response |
| `*tools.ChildTaskError` from the delegated `agent` tool | recoverable / child_failed | unknown; bounded child diagnostic becomes the tool result, the parent run stays healthy |
| Network-shaped invocation failure (`*url.Error`, `net.Error`, a per-call timeout whose parent context is still live) | recoverable / network_failed | unknown; bounded diagnostic; whether to retry is the model's decision |
| Any other invocation failure on a model-driven leg | recoverable / tool_error | unknown; bounded diagnostic; never presented as a success |
| Parent ctx cancellation/deadline or native Eino interruption | not a tool failure | propagate before classification |
| Runtime-integrity faults: the nudge invariant (`MarkFailure` without a call ID), durable tool-operation coordinator / Journal / storage failure, policy-engine evaluation failure, tool-hook **chain execution** failure (spawn, timeout, malformed envelope, empty or invalid rewrite), stale approval binding, preflight backend/IO failure, `adk` guardrails, provider failure | fatal | preserve the cause chain; never softened |

This is the pi `agent-loop` model: `runToolCall` returns an `isError` tool result for
an unknown tool, invalid arguments, a blocked call or a thrown error, appends it to
context, and only a provider stop/abort or an explicit terminate ends the loop. Vivy
applies the same rule on a **model-driven leg only** — a leg with bound nudge state —
because Eino's `ToolsNode` escalates every non-interrupt tool error into a run-fatal
`NodeRunError` (`compose/tool_node.go`) and offers no continue-on-error switch. The
governed shell and other non-model callers keep their original error behavior: they
have no nudge state and no model to correct the call.

Command failures are recognized only for the reserved command-tool implementation/result contract, before output framing/truncation. Do not parse arbitrary JSON from unrelated tools or search for the word “error”. `CommandResult.exit_code` is typed data. A nonzero code is an unsuccessful command even where a CLI uses it for “no match”; it remains model-visible and only identical repetition produces a reminder.

At the inner invocation seam, retain error provenance. On a model-driven leg every
invocation error becomes a result except control flow, explicit governance refusals
and runtime-integrity faults. The typed allowlist still owns the precise reason; the
`network_failed` / `tool_error` fallback only covers what the allowlist does not name,
and it never relabels a refusal as a tool error. Hook and authorization failures
passing through dispatch keep their own classification rather than inheriting one from
error text. Existing refusal branches should explicitly mark reason/status rather than
infer them from refusal text.

On soft conversion: preserve the diagnostic as bounded untrusted result text and the explicit classification; commit both through the existing operation owner before returning nil error to Eino. The mapper resolves durable classification first, using a producer failure mark only for existing non-operation paths. Do not export Go error text before redaction. Missing call ID during a model-originated conversion is an invariant failure, not a name-based fallback. Non-model shell paths retain their existing error behavior and have no nudge state.

MCP ToolWorld should return its internal typed ToolExecutionError when IsError is true. Transport failures stay unchanged. No `toolworld.Result` or plugin API field is added. Verify that ordinary and enhanced adapters preserve the text/parts payload and the error identity; do not wrap a media envelope into unparsable text.

## 6. Ordering, lifecycle and hard stop

```text
model tool request -> governed dispatch -> typed effect completion commit
-> producer coverage -> native ToolsNode joins correlated ToolMessages
-> WrapModel validates/resolves ordered outcomes -> exact tuple evaluation
-> persist tool.nudge once -> call model
```

Both inner Generate and Stream use the same synchronous boundary. Native
interrupts/cancellation remain control flow, never detector samples. Mandatory
operation completion and nudge scheduling cannot be delayed or bypassed by
advisory policy. A derived `tool.finished` consumer may lag without controlling
continuation; its own Journal failure still closes the Run through the existing
Service terminal owner.

Create state when drive/resume starts. Share exactly that pointer across Service, mapper and wrapper via context; Engine middleware itself is immutable. Approval/question interruption aborts the current leg without consuming the interrupt as a failure. Resume creates fresh detector state, consistent with baseline, and does not replay an old pending reminder. Process restart uses existing orphan/checkpoint recovery. No new checkpoint field, cross-Run counter, cross-Goal counter or persisted detector cache.

## 7. Reminder and Journal contract

New event: `tool.nudge`, payload version 1:

```json
{"tool_call_id":"call-3","tool_name":"read_file","reason":"not_found","repeat_count":3,"template_version":"nudge-v1"}
```

All five fields required, additionalProperties=false; repeat_count is 3 or 5. It denotes a reminder scheduled for the next model invocation, not remote receipt. Add it to domain.EventTypes and event-envelope/payload schemas. No database migration: reuse the existing Journal envelope. Read/fork/rewind must retain it as an event, not synthesize a user-authored session message.

Extend tool.finished with optional `outcome`, `reason`, `effects`; preserve existing error/result/parts fields. Old rows lacking metadata remain readable. Update strict JSON schemas and any exhaustive event consumers. Do not silently broaden payload validators globally. The pre-existing parts/schema mismatch is relevant only where this Story's multimodal fixtures exercise it; reconcile that touched schema field explicitly.

One coalesced notice per model boundary. ND-3's Service-supplied emitter appends/publishes it through the existing Journal and budget path before wrapper handoff; error returns the original cause to consume, the sole terminal owner for that boundary. Append failure never degrades to an unrecorded reminder. Existing model.request is emitted at Run admission, so it is not evidence that every later reminder was delivered. Do not relabel it or add a misleading delivery guarantee. Fake-provider request capture supplies acceptance evidence; live telemetry may only claim scheduling/handoff.

The single trailing `schema.UserMessage` is tagged with internal source metadata (kind=runtime_nudge, call ID, template version) and contains a fixed template plus bounded trusted identifiers. It is not stored in the human transcript or ADK checkpoint. Use a copy of the request slice. Do not change ToolInfos, system identity, AGENTS/skill ordering or tool-result pairs. Attach only after current tool results and after compaction; respect the existing context budget before handoff. Provider retry sees the same immutable request and must not schedule a second event.

Template (ordinary failure): “Runtime reminder: this unsuccessful tool call has repeated {count} times. Inspect the previous result, correct the arguments or choose another permitted approach. If blocked, report the blocker. A failed call may already have caused effects; inspect state before repeating a mutation.”

Refusal variant: “Runtime reminder: this refused call has repeated {count} times. Respect the policy or user decision. Do not bypass it through another tool. Continue only within existing authorization, or report the blocker.”

No raw arguments or diagnostic text in the notice. Limit rendered reminder to 1024 UTF-8 bytes and honor a smaller available context budget; failure to fit is a classified context/budget stop, not silent injection beyond budget. No new user configuration. Fixed templates belong alongside current runtime prompt composition, tested as trusted instruction text; do not migrate unrelated prompts.

## 8. Compatibility, cost and rollback

No change to public tool/Port interfaces, RPC or configured provider retry behavior. Only additive Journal vocabulary/fields plus correction of sixth-result audit behavior. Existing UI error rows consume error unchanged; generic Inspect shows tool.nudge. Add a minimal projection only if the existing generic viewer drops the new event; no dedicated feature panel.

Per completed call: an exact scan of ten tuples; per batch: request-order traversal and existing operation reads. Memory is ten bounded tuples plus admitted producer coverage and one retry memo. No added model request solely for nudge and no synchronous remote I/O except existing Journal persistence. These are algorithmic properties, not benchmark claims. Result bytes and reminder tokens remain bounded.

Rollback the implementation as one feature series; never erase emitted Journal events. Readers must tolerate the additive event before runtime emission is enabled. Keep decoder support if an emitting build was released. No migration rollback is required.

## 9. Readiness and verification

The architecture fixes boundaries and algorithms; ND-0 is an executable compatibility gate, not permission for an implementer to redesign public interfaces. It must prove Generate/Stream ordering, plain/enhanced tool correlation, failure metadata, a failing Journal and cancellation wake-up without deadlock. If a pinned Eino assumption fails, stop ND-2/1/3, update this spec and all consumers together; do not bypass Service or ship a second agent loop.

Run the named Story regressions and the owner-authorized `just wiring-ci` plus `just kernel-ci` from the independent Linux-filesystem WSL checkout with pinned Laputa. The real isolated nonce-read App probe covers model/tool/model/final/idle. D13 adds split-browser mock acceptance; D14 retains full-product CI and real-token release evidence. Do not substitute repeated Windows full CI for this kernel scope.
