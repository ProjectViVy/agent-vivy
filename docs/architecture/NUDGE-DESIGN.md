# Tool Failure Recovery and Nudge — Detailed Design

Revision **ND-D1**, 2026-09-23. Issue [#58](https://github.com/ProjectViVy/agent-vivy/issues/58).
Baseline: `a8d361b0244a1c40be513622bbdaebb5c9d40014` (local and fetched main).
Delivery authority: [plan index](../plans/nudge/README.md). This engineering specification supersedes the preliminary issue comment where explicitly refined below. It is a design deliverable, not shipped behavior.

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

Pinned Eino v0.9.13; EinoExt Claude v0.1.25, OpenAI v0.1.13, MCP v0.0.9. Inspected upstream `adk/handler.go`, `adk/chatmodel.go`, `adk/chatmodel_test.go`, `compose/tool_node.go`.

- `compose.GetToolCallID(ctx)` is the correlation source in runtime adapters. Do not match parallel results solely by tool name.
- `adk.ChatModelAgentMiddleware.WrapModel` returns `model.BaseModel[*schema.Message]` and wraps both Generate and Stream. Eino owns tool binding separately.
- `BeforeModelRewriteState` persists message edits in ADK state. We deliberately choose WrapModel for a **single-request transient reminder** to avoid replaying synthetic instructions after resume/compaction. Upstream discourages general message rewriting here because it is not persisted; that property is intentional for this bounded trailing reminder. Stable instruction prefix and tool catalog remain untouched.
- `compose.ToolMiddleware` supports ordinary and enhanced invocation. It is available but not needed for broad error catching: the existing ordinary adapter is the common governed boundary.
- UnknownToolsHandler is available but is **not enabled by this design**. Hallucinated-tool recovery is adjacent scope and currently stays fatal; naming an available upstream hook is not a requirement to use it.
- Native interrupts, Runner checkpoints and MaxIterations are retained.

Custom policy is needed for Vivy's recoverable failure allowlist and Journal ordering. It lives within runtime, with a first-party MCP error carrier in mcphost. Eino stays quarantined to runtime/provider. Replace the small adapter if upstream offers equivalent scoped semantics later.

Source inspection resolves API selection. Executable integration evidence is still required in ND-0: no claim that an installed source tree is a passing probe. Go and just are absent from this planning environment.

## 4. Internal contracts (proposed unless already present)

All new names below are package-private except the MCP error carrier consumed across existing internal packages. No SDK/Port or public RPC extension.

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
func (s *nudgeState) Register(ids []string) error
func (s *nudgeState) MarkFailure(id string, failure toolFailure) error
func (s *nudgeState) Failure(id string) (toolFailure, bool)
func (s *nudgeState) Complete(call completedCall) error
func (s *nudgeState) Seal(err error)
func (s *nudgeState) Take(ctx context.Context) (*nudgeNotice, error)
func (s *nudgeState) Abort(err error)
func withNudgeState(ctx context.Context, state *nudgeState) context.Context
func nudgeStateFromContext(ctx context.Context) *nudgeState
```

Register receives one model turn's IDs in request order before dispatch; empty, duplicate or overlapping outstanding IDs are invariant errors. One outstanding batch only. Failure/Complete are mutex-protected and keyed by ID. Complete stores an already redacted, bounded mapped outcome; it does not publish or inject. Seal is called by the consuming Service after all tool.finished events for the batch have successfully persisted. Seal flushes completed outcomes in request order into the shared loop window, prepares the highest-threshold pending notice (tie: earliest request position), then releases the waiting model boundary. On error, Seal/Abort wakes waiters with the original cause. Neither holds a lock during Journal I/O or model calls.

Take waits for the batch to seal or context cancellation, consumes at most one notice and returns terminal error if present. A second Take without a new batch gets no notice. Abort is idempotent and prevents any further model handoff. Keep only ten signature/count entries and current in-flight batch metadata; never retain historical raw arguments. Bound the in-flight batch by the existing tool/budget admission constraints; reject invalid oversized batches before allocating state. No unbounded set of historical IDs.

The existing loopWindow becomes the sole detector owned by nudgeState; remove the mapper's independent window. Extend its record API to return the matching count plus the existing errLoopDetected sentinel. Keep canonical argument/result hashing semantics; do not mix generated reminders into signatures. No two synchronized detectors.

## 5. Selective recovery and provenance

| Origin | Classification | Effects / behavior |
| --- | --- | --- |
| Existing dispatch validation refusal | refused / invalid_arguments | not_executed; existing result remains readable |
| Existing policy, safety or human refusal | refused / policy_denied or user_denied | not_executed; explicit boundary-respecting reminder |
| `*tools.ArgError` from actual tool invocation | recoverable / invalid_arguments | unknown unless known validation occurred before dispatch; preserve unwrap |
| `fs.ErrNotExist` from invocation of a readonly tool | recoverable / not_found | none; no global storage-error catch |
| Known execute/bash CommandResult with nonzero exit | recoverable / command_failed | unknown; preserve exit code/output; never call it a safe-to-retry result |
| First-party MCP IsError converted to ToolExecutionError | recoverable / remote_tool_error | unknown; preserve bounded untrusted response |
| Parent ctx cancellation/deadline or native Eino interruption | not a tool failure | propagate before classification |
| MCP JSON-RPC/transport errors, unknown errors, local timeouts without explicit side-effect evidence | fatal | preserve current cause chain; no generic retry or recovery |
| Policy engine, Journal, storage or invariant failure | fatal | never softened |

Command failures are recognized only for the reserved command-tool implementation/result contract, before output framing/truncation. Do not parse arbitrary JSON from unrelated tools or search for the word “error”. `CommandResult.exit_code` is typed data. A nonzero code is an unsuccessful command even where a CLI uses it for “no match”; it remains model-visible and only identical repetition produces a reminder.

At the inner invocation seam, retain error provenance. The runtime can convert only allowlisted execution failures; hook/authorization/storage errors passing through dispatch do not become recoverable merely because their messages resemble an argument error. Existing refusal branches should explicitly mark reason/status rather than infer them from refusal text.

On soft conversion: MarkFailure(current call ID), preserve diagnostic as untrusted result text, return nil error to Eino, and later populate tool.finished.error from the side-channel metadata. Do not export Go error text before redaction. Missing call ID during a model-originated conversion is an invariant failure, not a name-based fallback. Non-model shell paths retain their existing error behavior and have no nudge state.

MCP ToolWorld should return its internal typed ToolExecutionError when IsError is true. Transport failures stay unchanged. No `toolworld.Result` or plugin API field is added. Verify that ordinary and enhanced adapters preserve the text/parts payload and the error identity; do not wrap a media envelope into unparsable text.

## 6. Ordering, lifecycle and hard stop

```text
model tool request -> register batch IDs -> governed tool dispatch
-> adapter marks typed failures -> Eino yields correlated results
-> mapper creates tool.finished with real error metadata
-> Service persists every finished result -> Seal in request order
-> model wrapper Take -> persist tool.nudge -> call model
```

The wrapper must wait **before** both inner.Generate and inner.Stream, not on their returned output. Registration for streaming models occurs before Eino receives the completed tool call; for non-streaming models, ND-0 must pin the equivalent event-before-dispatch order. No busy-wait, new goroutine per reminder or global lock.

On the sixth matching result, persist the actual tool.finished, seal with errLoopDetected, and emit one run.failed through existing Service terminal handling. This explicitly fixes the baseline's missing sixth result event while preserving the hard-stop threshold. Already-dispatched parallel siblings may have run; this feature cannot roll them back. They must be settled/audited by the existing batch consumer before shutdown; no seventh model request is admitted. Cancel the drive context on terminal paths and Abort waiters even when Journal persistence fails, so stopping the consumer cannot strand the producer.

Normal success calls count toward the existing hard guard but never produce failure reminders. Counts 3 and 5 are proposal constants, not measured optimum. Sliding-window counts can fall and reach a threshold again; a new source call may then produce a new notice. Deduplication is by the current batch's source call, not a permanent per-signature blacklist. Successful/different results naturally alter the ten-entry window.

Create state when drive/resume starts. Share exactly that pointer across Service, mapper and wrapper via context; Engine middleware itself is immutable. Approval/question interruption aborts the current waiting boundary without consuming the interrupt as a failure. Resume creates fresh detector state, consistent with baseline, and does not replay an old pending reminder. Process restart uses existing orphan/checkpoint recovery. No new checkpoint field, cross-Run counter, cross-Goal counter or persisted detector cache.

## 7. Reminder and Journal contract

New event: `tool.nudge`, payload version 1:

```json
{"tool_call_id":"call-3","tool_name":"read_file","reason":"not_found","repeat_count":3,"template_version":"nudge-v1"}
```

All five fields required, additionalProperties=false; repeat_count is 3 or 5. It denotes a reminder scheduled for the next model invocation, not remote receipt. Add it to domain.EventTypes and event-envelope/payload schemas. No database migration: reuse the existing Journal envelope. Read/fork/rewind must retain it as an event, not synthesize a user-authored session message.

Extend tool.finished with optional `outcome`, `reason`, `effects`; preserve existing error/result/parts fields. Old rows lacking metadata remain readable. Update strict JSON schemas and any exhaustive event consumers. Do not silently broaden payload validators globally. The pre-existing parts/schema mismatch is relevant only where this Story's multimodal fixtures exercise it; reconcile that touched schema field explicitly.

One coalesced notice per model boundary. ND-3's Service-supplied emitter appends/publishes it through the existing Journal and budget path before wrapper handoff; false/error terminates the call. Append failure never degrades to an unrecorded reminder. Existing model.request is emitted at Run admission, so it is not evidence that every later reminder was delivered. Do not relabel it or add a misleading delivery guarantee. Fake-provider request capture supplies acceptance evidence; live telemetry may only claim scheduling/handoff.

The single trailing `schema.UserMessage` is tagged with internal source metadata (kind=runtime_nudge, call ID, template version) and contains a fixed template plus bounded trusted identifiers. It is not stored in the human transcript or ADK checkpoint. Use a copy of the request slice. Do not change ToolInfos, system identity, AGENTS/skill ordering or tool-result pairs. Attach only after current tool results and after compaction; respect the existing context budget before handoff. Provider retry sees the same immutable request and must not schedule a second event.

Template (ordinary failure): “Runtime reminder: this unsuccessful tool call has repeated {count} times. Inspect the previous result, correct the arguments or choose another permitted approach. If blocked, report the blocker. A failed call may already have caused effects; inspect state before repeating a mutation.”

Refusal variant: “Runtime reminder: this refused call has repeated {count} times. Respect the policy or user decision. Do not bypass it through another tool. Continue only within existing authorization, or report the blocker.”

No raw arguments or diagnostic text in the notice. Limit rendered reminder to 1024 UTF-8 bytes and honor a smaller available context budget; failure to fit is a classified context/budget stop, not silent injection beyond budget. No new user configuration. Fixed templates belong alongside current runtime prompt composition, tested as trusted instruction text; do not migrate unrelated prompts.

## 8. Compatibility, cost and rollback

No change to public tool/Port interfaces, RPC or configured provider retry behavior. Only additive Journal vocabulary/fields plus correction of sixth-result audit behavior. Existing UI error rows consume error unchanged; generic Inspect shows tool.nudge. Add a minimal projection only if the existing generic viewer drops the new event; no dedicated feature panel.

Per completed call: existing hash work plus a scan of ten entries; per batch: request-order traversal. Memory is ten signatures plus admitted in-flight outcomes. No added model request solely for nudge and no synchronous remote I/O except existing Journal persistence. These are algorithmic properties, not benchmark claims. Result bytes and reminder tokens remain bounded.

Rollback the implementation as one feature series; never erase emitted Journal events. Readers must tolerate the additive event before runtime emission is enabled. Keep decoder support if an emitting build was released. No migration rollback is required.

## 9. Readiness and verification

The architecture fixes boundaries and algorithms; ND-0 is an executable compatibility gate, not permission for an implementer to redesign public interfaces. It must prove Generate/Stream ordering, plain/enhanced tool correlation, failure metadata, a failing Journal and cancellation wake-up without deadlock. If a pinned Eino assumption fails, stop ND-2/1/3, update this spec and all consumers together; do not bypass Service or ship a second agent loop.

Run the Story commands in the plan index, then mandatory `just ci` in the repository-supported environment and real-path smoke. Go is pinned to 1.26.4; justfile uses PowerShell, so a generic Linux shell is not an equivalent CI environment. This planning turn has no runtime/CI evidence. Each plan distinguishes future test commands from checks actually performed.
