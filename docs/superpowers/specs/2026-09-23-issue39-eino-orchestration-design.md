# Issue 39: Eino-native child and bounded workflow design

> Status (2026-09-27): provisional implementation through ORCH-07 is present in the current worktree, including Service/Eino child execution, direct ChildSession mailbox and parent inbox delivery, validated immutable DAG revisions, model-facing `workflow`, `reply_parent` and `child_inbox` tools, RPC, and UI projections. One fix pass addressed eight Important findings from the whole-branch review. G0/G1 and release remain BLOCKED: PostgreSQL DSN-backed behavior, repository `just ci`, real browser/E2E, and integrated R1-R14 evidence have not been completed. Focused local checks are recorded in [provisional verification](../../../logs/2026-09-26-issue39-orch02-provisional/verification.md). No product interface is released. Baseline for this freeze: agent-vivy `a836c4088953dd6f8997cfe0fba4c479bc4908fb`, Eino v0.9.13 `c5e6aef927cca02bea934541f8dff2ea711b2ca7`. Decision record: [Issue #39](https://github.com/ProjectViVy/agent-vivy/issues/39#issuecomment-5798636327). Execution package and sole Story-state DAG: [index](../plans/issue39-eino-orchestration/index.md).

## Outcome and boundaries

An agent can delegate a bounded task to a governed child, inspect and interrupt it, continue its task-scoped conversation when authorized, and compose children into a validated acyclic dependency workflow. Vivy's existing Service remains the sole Run and model admission path; Journal is the product evidence; policy, Host and budget broker all model and tool effects. Eino compiles and executes the workflow and owns its opaque execution checkpoint. The design introduces no alternate agent engine, model loop, graph scheduler, external CLI delegation, or independent personality store.

**Durable direct parent-child messaging is required.** Route messages to the stable ChildSession, never an ephemeral activation Run. Persist each admitted message with a stable message ID, sender-scoped idempotency key, recipient mailbox sequence, payload digest and lifecycle state before acknowledging admission. Identical retries return the same message ID; a reused key with different content conflicts. Each recipient may admit at most 128 messages over a ChildSession's lifetime, and each body is limited to 32 KiB; an exact idempotent retry still resolves to its original message at the cap. Acknowledgement means durable admission, not recipient completion or reply. Child inbox messages are consumed at child activation safe points; child replies are injected into the next active parent Run's bounded model input and consumed at the parent turn completion safe point. Both directions keep durable receipts/cursors and at-least-once redelivery across failure/restart. The explicit `child_inbox` tool returns and acknowledges a bounded set of direct replies. A continuable child can use `reply_parent` to send a separately addressed direct reply; its idempotency identity includes both activation Run and model tool-call ID. One-shot and workflow children cannot use this tool. No exactly-once model/tool effect is claimed. Follow-up starts a new activation Run and is distinct from delivery to an active child. Peer and swarm messages remain unauthorized and deferred.

This contract freeze is not a green light to ship. Every implementation still requires its stated gates, including G0/G1. Context sharing/fork, named-mask inheritance, swarm collaboration and peer traffic, external delegation, and optional workflow product #40 remain outside scope. The approved child context is clean: explicit task, admitted direct messages, and explicitly approved dependency outputs only. No transcript, hidden history, personality or implicit context is copied.

## Verified baseline and comparator

| Area | Current source / observation | Architectural consequence |
| --- | --- | --- |
| Child execution | `internal/app/worker.go` starts a child Run in the parent's Session, supervises `internal/worker`'s handwritten model/tool loop; live handles enable `WaitChild`; existing depth 4, concurrent children per parent 4, turns 8. | Preserve admission limits, migrate execution through Service and Eino; a stable child conversation requires a new durable session boundary and migration strategy. |
| Existing surfaces | `internal/rpc/control.go` implements `child/start,get,list,wait,cancel`; `ui/src/components/chat/RunInspector.tsx` presents these controls. | Extend the contracts compatibly; do not create a separate child API/UI state source. |
| Vivy evidence | `internal/runtime/service.go`, `internal/storage/contracts.go`, Journal, SnapshotStore, BlobStore and LeaseStore; `internal/runtime/checkpoint.go` verifies version, checksum and prompt binding. | Reuse stores, Run transitions, existing approval and deletion fences; fail closed on checkpoint mismatch. |
| Eino 0.9.13 | Pinned local source: `compose.Workflow.Compile(ctx, ...GraphCompileOption)`, `Workflow.End()`, `WorkflowNode.AddInput`, `WorkflowNode.AddDependency`; Workflow uses `AllPredecessor` and forbids cycles. `AddEnd` is deprecated. `WithMaxRunSteps` is a Graph compile option but is unsupported for DAG/Workflow mode. Current production engine uses ADK agent runner; `internal/runtime/graph_conformance_test.go` is not proof of child/approval/graph integration. | Host validates finite node count, depth, width and output limits. Workflow dependencies govern execution; selected outputs must flow through explicit mappings to a consumed node or Workflow.End. G0 remains required. |
| Reference products | DeepSeek Harness demonstrates bounded direct parent-child control and conversation distinction; ZCode illustrates workflow product affordances; Codex interaction design and OpenFang origin provide comparisons. | DSH is the primary **behavioral** comparator; none supplies Vivy's authority or persistence implementation. |

Pinned comparison clones (read-only research): deepseek-harness `46a7f68`, ZCode `328c1a0`, minimax-code `a914a30`, codex `cb1eea3`, OpenHarness `9b2efd7`, oh-my-pi `73a11421fe34fbab8ad058ab9c1a2e4f447852ea`, openfang `acf2587e46be174c10200489c9a2d23a39a98aeb`, agent-diva `0fd005a105d8987df02ae7b796a3c591e04b9ca3`. These are snapshot identifiers, not equivalent capability claims.

For mailbox compatibility, the [pinned DeepSeek Harness control reference](https://github.com/deepseek-ai/deepseek-harness/blob/ddefc45fbc7f8e46dd73185e68295696d1297887/packages/subagent/tool-subagent-control/README.md) is a useful behavior comparator: `send_message` targets the direct parent/child relationship, accepts into a FIFO inbox and returns a stable message ID; a resident target sees mail at a step boundary and an inactive continuable target can resume. Its `interrupt_agent` stops the current turn while leaving inbox and descendants intact. This does not imply sibling messaging, general peer messaging, or a Vivy implementation contract; Vivy's Host must own authorization and persistence.

## Ownership and execution topology

```mermaid
flowchart TD
  A["Agent-authored task or DAG proposal"] --> B["Host validation and policy"]
  B --> C["Service Run and authority"]
  C --> D["Eino native child / Workflow"]
  D --> E["Brokered model and tools"]
  D --> F["Opaque checkpoint bridge"]
  C --> G["Vivy Journal and projections"]
```

| Fact | Authoritative owner | Rule |
| --- | --- | --- |
| Authorization, budgets, tool selection, workspace | Original parent Run's immutable authority ceiling, current authorizer Run, policy and Service | Origin lineage is immutable. A new active Run in the same original parent Session may reauthorize continuation after the origin parent Run terminates. Effective authority is current authorization intersected with the original ceiling, on every activation/resume. Deleting the parent Session fences continuation. Child tool selection may narrow, never widen, the immutable ceiling. A mask cannot grant tools or alter policy. |
| Run state, approval outcome, graph/node status, UI evidence | Vivy Run/Journal; durable projection derived from Journal | All public transitions and results have stable IDs and journal sequence. A terminal Run receives no later append. UI only reads the host. |
| Execution continuation | Eino-native runner/Workflow checkpoint bytes through `VersionedCheckpointStore` and `BlobStore` | Separate checkpoint keys per graph Run/child activation; verify engine, prompt identity and checksum. The checkpoint is not a competing product event log. |
| Task-scoped conversation | Durable ChildSession and its Messages; multiple activation Runs | No independent identity or evolution and no implicit parent/shared transcript. Immutable origin parent Run and Session lineage; current authorizer Run is recorded separately. |
| Workflow revision and input mapping | Immutable host-validated workflow descriptor and digest; Run/Journal maps descriptor nodes to execution IDs | Compile same descriptor on recovery; no dynamic mutation of a running revision. Result handoff is explicit, bounded and recorded. |
| Direct mailbox | Host-owned inbox addressed by stable ChildSessionID; durable admitted envelope and recipient cursor/receipt | Only direct parent-child messages are authorized. Stable message ID and sender operation key make admission idempotent; each recipient has an independently monotonic inbox order and cursor, preserved across restart. No cross-direction total order is promised. Each recipient is capped at 128 admitted messages per ChildSession lifetime; each body is capped at 32 KiB. Delivery/consumption is at-least-once, never exactly-once effects. Eino input is never mutated behind a running invocation; consume only at the Service/Eino activation safe point. A continuable child replies through `reply_parent`; one-shot and workflow children cannot access it. |

### Domain and host contracts

The first implementation gate must confirm the narrowest existing Service extension. Signatures below specify contracts, not an instruction to add every type before it is used.

```go
// Internal host-owned records; no Eino type escapes internal/runtime.
type ChildSessionBinding struct {
    SessionID domain.SessionID       // durable task conversation
    OriginParentRunID domain.RunID    // immutable lineage
    LastRunID domain.RunID            // latest activation, not identity
    AuthorityDigest string           // versioned parent-derived envelope
}
type WorkflowRevision struct {
    ID string; Digest string; ParentRunID domain.RunID
    Nodes []TaskNode; Edges []Dependency
}
type TaskNode struct { Key string; Task string; MaskHint string }
type Dependency struct { From string; To string; OutputKey string }
// Contract sketch only; final wire names follow existing domain conventions.
type ChildMessageEnvelope struct {
    ID string; TargetSessionID domain.SessionID
    SenderPrincipal string; OperationID string; Sequence uint64
    Body string
}
```

The child control interface keeps existing `child/start,get,list,wait,cancel` behavior for existing callers, adding explicit `child/followup` and `child/interrupt` only after the new admission path is proven. `followup(child_session_id, operation_id, text)` creates a fresh activation Run in the same ChildSession and never revives a terminal Run. It is a later activation, not in-flight mailbox delivery. `interrupt(run_id)` interrupts the active activation while preserving its ChildSession; terminal Run interruption is idempotent. Existing `child/cancel` stays compatible and is not an alias for interrupt or Session close. `wait` reads terminal durable status after restart and honors caller cancellation. Admission operation ID is mandatory for retry-safe creation; preserve old requests through server compatibility until consumers migrate. This API does not replace the required durable direct parent-child mailbox; ORCH-04 implements the message path under D8.

Host derives sender identity from the live participant Run and verifies direct parent-child relationship; caller-supplied identity is untrusted. A new active Run in the original parent Session can inspect and consume replies after policy/sandbox compatibility checks, even when the binding still records an earlier authorizer. A recipient mailbox sequence defines stable order. Persist admission, receipt and cursor through Journal/host storage, not only process memory. Retry after interruption or restart may redeliver a message until durable consumption is recorded. This is at-least-once delivery, not exactly-once execution or effect. At the verified Service/Eino safe point, consume only admitted messages and record the durable receipt/cursor. Never mutate a child activation's input or history behind the runner.

Successful send acknowledges durable admission with a stable ID and does not wait for or return a reply. A reply is a separate addressed message. Interrupt stops the activation but preserves ChildSession and admitted pending messages. A later message can activate a continuable idle child only after current authority is revalidated. Caller cancellation after durable admission does not retract the message; retry with the same key returns its ID.

ORCH-04 implements and tests sender/recipient authorization, ordering under concurrent senders, lifecycle states, retry/duplicate outcomes, expiry/retention/redaction, backpressure, interruption and Session deletion/closure, and host-derived UI projection. This does not imply peer-to-peer child messaging or shared state.

### Child lifecycle and management controls

ChildSession lifecycle and activation Run lifecycle are separate. **Interrupt** stops the activation at a supported boundary and leaves the task Session eligible for a later Run. **Cancel** terminates the named active Run and does not silently close the ChildSession. A terminal origin parent Run does not itself prevent later continuation by a new authorizer Run in the same parent Session. **Close**, if offered, prevents later messages/follow-ups and defines pending-mail and descendant outcomes. Parent Session deletion is a privacy/resource fence, not normal interruption; it fences continuation.

Delegation uses clean context only: explicit task, admitted direct messages, and approved dependency outputs. No parent transcript or completed-turn context fork, hidden history, personality, or implicit context is copied. This is the complete context contract; no context-fork capability is in scope.

The child uses the parent's model; there is no child model picker or model override. Tool selection may only narrow the original immutable parent tool ceiling and cannot widen it. Workspace authority remains governed by the parent snapshot and Host policy; a child request cannot widen or override it. Continuable children and workflow nodes use the read-only baseline (with the mailbox-only `reply_parent` exception for continuable children). The existing one-shot Service path may receive an explicitly selected parent-authorized effectful tool; it still uses the parent's approval policy and private child workspace. This narrow compatibility path is not a general writable-child mode, and mask or mailbox content cannot grant tools. Broader concurrent writable children still require workspace ownership/isolation, conflict detection, integration/merge review, and partial-completion evidence. Budget admission must account for the full descendant tree, not just each visible child; surface limits, actual token/tool usage, and partial results/errors without double counting. Existing in-memory concurrency counters require a restart/lease test before being treated as durable admission authority.

Follow-up and message admission require a new active authorizer Run in the same original parent Session. This remains valid after the origin parent Run terminates. Preserve immutable origin parent Run/Session lineage, record each current authorizer Run, and calculate effective authority as current authority intersected with the original parent's immutable ceiling. Parent Session deletion fences continuation and messages; historical access remains scoped to the authorized parent Session. A ChildSession never appears as an unrelated ordinary sidebar Session. Both SQL backends require conformance before schema acceptance.

One-shot and continuable modes coexist. Legacy synchronous agent and DAG nodes default to one-shot and are non-addressable. Only an explicitly continuable child receives a stable ChildSession and supports follow-up/mail. A one-shot task returns bounded result/error and cannot become addressable after completion.

The mask hint is a literal task-scoped instruction processed through existing prompt assembly and immutable Run snapshot, subject to authority. Named catalog inheritance is out of scope. A child has no independent personality/evolution record. Its context contains only explicit task, admitted direct messages and approved dependency outputs. Redact and bound outputs under existing policy.

The model-facing built-in `workflow` tool accepts a strict JSON descriptor and derives its idempotency operation from the model tool-call identity. The host validates and persists an immutable revision before starting an Eino `compose.Workflow`; nested workflows are excluded from child tool ceilings. Current descriptor bounds are 12 nodes, 24 edges, depth 6, width 4, 4 outputs, 4 KiB per task, 16 KiB total task text, and 8 KiB total outputs. Node children are one-shot and read-only; each receives its own task plus explicitly mapped dependency outputs. The tool returns the workflow Run ID and bounded declared outputs. RPC/UI proposal and inspection remain backed by host state; they do not authorize a release while G0/G1 are blocked.

## Child activation lifecycle

1. Parent tool or UI asks Host to create a child/follow-up; Host checks the active current authorizer Run belongs to the original parent Session, scope, quotas, policy, selected tools and workspace. Persist idempotent admission and immutable origin lineage plus current authorizer Run before exposing an ID; reserve budget and child slot through the existing broker. Deletion fences continuation.
2. Service creates/activates the child Run, fixes the immutable prompt and parent authority, then runs the Eino-native agent path. Model calls and tool effects travel through existing Service/Host policy, approvals and ledger. In-memory handles are accelerators, not proof a child exists.
3. Tool approvals/questions suspend at the same Service interaction boundary. Persist the Eino checkpoint before publishing the interrupt; resume with the same Run/prompt identity and deduplicated side-effect key. Persist messages before acknowledging admission; consume at a proven safe point and record receipt. Delivery is at-least-once; do not claim exactly-once effects.
4. Cancellation propagates from parent, direct child control or session deletion through Service to Eino invocation; an approval pending cancellation remains cancelled. Record one terminal event and release slots once. A repeated request returns durable terminal state.
5. On restart, derive nonterminal work from durable Runs/Journal/leases, verify checkpoint compatibility and authority, restore mailbox cursor/pending delivery from durable state, and either resume once or mark a reasoned terminal failure; never silently rerun a tool. Recover terminal `wait` from durable state.

Admission keys remain `(origin parent Run, operation ID)` for child admission and `(workflow Run, revision digest, node key)` for graph node admission. Tool operation identity follows D15 below. ORCH-01 must establish the minimum durable tool-operation and proof-node admission boundaries before its integrated G0 proof; ORCH-02 extends the proven boundary to continuable ChildSessions and mailbox records. It is not a prerequisite for ORCH-01. Repeated admission with the same key and payload returns the same ID; changed payload conflicts. An in-memory map is insufficient.

## Recovery safety boundary (D15, approved 2026-09-26)

**Decision:** Retain Service/Journal ownership and Eino scheduling. Move the minimum durable operation identity, atomic execution claim and result resolution into ORCH-01 before G0. Reuse existing Journal/storage transactions where they can prove uniqueness and concurrent admission; add a narrow storage contract and paired migration only if that reuse is insufficient. The SQL operation row preserves the private invocation bytes required for recovery; the D15 `tool.operation` lifecycle event records digests and state only, never raw invocation arguments. The row and lifecycle event commit in the same transaction. This decision does not change the existing `tool.requested` and approval-required event payload contracts or their established redaction/projection behavior. Do not build a general transaction platform, outbox, scheduler or alternate execution loop. This costs a small foundation change before the proof and avoids the prior circular dependency. No public child write capability is authorized by this recovery work.

**Evidence and limit:** At branch baseline `bbcbd10`, `ExecuteBrokerTool` accepts no stable operation identity; `internal/app/worker.go` records `tool.started`, invokes the tool, then ignores errors recording `tool.finished`. The historical [Task 5 evidence](../../logs/2026-09-26-issue39-native-proof/task-5-summary.md) only exercises two direct same-process calls and an in-memory counter. It proves neither a crash replay nor an Eino limitation. Its NO-GO remains historical evidence. The current attempt implements the D15 boundary and a bounded Eino proof, but G0 remains BLOCKED: a post-policy-fix full-suite rerun was blocked by automatic review when a test attempted an unapproved request to `api.deepseek.com`; the repository `just ci` recipe and PostgreSQL DSN-backed conformance are also unverified. See the current dated attempt log in `docs/logs/`.

### Identity, ownership and admission

- Service owns a durable logical operation key scoped to the activation Run, using a stable tool-call ID where the runner preserves it, otherwise an explicit persisted operation ID. Re-entry and recovery use that same key. Execution attempts are audit metadata, never part of the deduplication identity. Different operation IDs with identical arguments remain distinct valid calls.
- Bind the operation to its tool identity, original request digest, pre-middleware input and a deterministic digest of effective validated arguments. Keep raw request/argument bytes in the private operation row; the D15 `tool.operation` lifecycle event carries digests only. Existing `tool.requested` and approval-required event payloads retain their prior contracts and redaction/projection behavior. On recovery, re-evaluate current public middleware against the stored pre-middleware input and require the output to equal the admitted effective arguments. Never rerun tool hooks or silently substitute changed arguments. Current authority, approval and deletion fences still govern execution and result access.
- Persist admission, then atomically claim execution under the existing Run/lease fence before invoking the tool. At most one current owner may execute a key. Concurrent callers must observe that owner or the durable result; lease expiry alone never proves an external effect did not occur and never authorizes another invocation.
- Persist the bounded, redacted result or classified failure before reporting completion to Eino. Propagate completion-write failures; never discard them or return an ordinary successful completion. A failure to persist leaves the prior claim as the recovery fence. Apply the same boundary to approved broker calls; approval does not bypass recovery safety.

| Durable operation state | Recovery action |
| --- | --- |
| Admitted, unclaimed; no execution is possible without the claim | Revalidate authority, atomically claim, then invoke. |
| Claimed with a live valid owner | Wait under existing bounded Service controls or report in progress; do not invoke concurrently. |
| Completed with durable result/failure | Return the persisted resolution under current access checks, without invoking again. |
| Claimed, owner lost, no reliable completion; includes a crash just before invocation | Treat effects as unknown, block automatic replay, and surface an explicit reason. Fail closed under the existing Run state machine; no new public state enum is assumed. |
| Same key, different tool or payload | Reject as an operation conflict before effects. |

For resumed approvals, current middleware must see the same pre-middleware input it saw during admission. Persist that input alongside the effective arguments so rewrites can be rechecked after a crash or approval pause without rerunning tool hooks. If a rewrite changes, mark the approval stale and stop before the operation claim; the effect remains unexecuted.

A local record cannot atomically commit an arbitrary external effect. A crash after the effect but before its result is durable is therefore allowed to stop progress; safety takes precedence over automatic completion. Unknown outcomes do not become success, do not restart automatically under a fresh operation ID, and do not append to terminal Runs. Downstream dependent nodes must not consume an unknown outcome as a successful result. A future tool-specific reconciliation path may use verified provider idempotency or authoritative result lookup, but only with explicit evidence for that tool; it is not required for this delivery. A deduplication table or Eino checkpoint alone does not guarantee exactly-once external effects.

### Required recovery evidence

Use the real Service/broker/Eino path and an observable durable effect fixture outside the Journal transaction, not only an in-memory counter. Inject abrupt process termination at the boundaries below and reopen persisted stores/checkpoints in a new process with fresh Service/Engine instances. Record operation IDs, graph/node/child Run IDs, checkpoint keys, Journal sequences, durable effects and parent-visible outcomes.

| Crash boundary | Required observation after recovery |
| --- | --- |
| Admission committed, before execution claim | Same operation can be claimed and executed once; no duplicate child admission. |
| Claim committed, before invocation | Unknown outcome blocks automatic replay, even if the fixture proves no effect happened; recovery cannot infer that from the claim alone. |
| Effect occurred, before durable completion | Effect remains once, automatic replay is blocked and the unknown outcome is visible; successful continuation is not required. |
| Completion committed, before graph checkpoint | Same logical operation returns its durable result without re-invocation, permitting continuation when the remaining graph state is recoverable. |

Also prove concurrent same-key admission, conflicting payload rejection, two distinct same-argument operations both executing, and completion-write failure propagation. Then prove parallel join, approval pause with sibling progress, cancellation and invalid engine/prompt rejection in the integrated graph. General child/graph product delivery remains gated. PostgreSQL deferral permits only explicitly labelled provisional evidence; schema acceptance and release still require both backends.

## Bounded native DAG

The parent agent proposes a declarative graph with stable node keys/tasks and dependency edges. Host validates finite node count, unique keys, referential integrity, no cycles, depth/width, authority/tool bounds, revision digest and output-size limits before creating a Run. These Host limits are authoritative: Eino v0.9.13 `WithMaxRunSteps` is unsupported in DAG/Workflow mode. Retries, loops, dynamic graph mutation, custom branching expressions and peer/swarm messaging are excluded.

Host atomically persists a validated immutable descriptor and opens a graph Run under the authorizer. In `internal/runtime`, compile `compose.Workflow` with brokered node lambdas. Use `WorkflowNode.AddDependency` for execution-only edges and `AddInput` only for approved output mappings. Use `Workflow.End()` and connect its inputs/dependencies so the declared workflow output consumes required final outputs. Every node must be reachable from the Workflow start and contribute through dependency/data flow to a consumed workflow output/end; reject unreachable or unconsumed nodes. Eino decides runnable order/parallelism. Node lambdas use idempotent durable admission and wait through Service. The graph Run owns events, checkpoint and terminal result; child Runs keep their Journal and Session. The projection derives from authoritative events and descriptor. Revision is immutable. Parent Session deletion fences unfinished work.

Graph-wide budget is inherited from the parent's ledger. Node execution uses existing parent concurrency cap rather than a second scheduler. Width validation alone cannot guarantee runtime admission if unrelated children occupy slots: define deterministic bounded backpressure or fail with a clear capacity outcome at the Service seam; do not spin or silently enlarge the limit. First prove parallel join, approval interrupt, cancellation, crash/restart and zero duplicate side effects together in Story 01. If v0.9.13 does not provide the needed recovery semantics, narrow scope to sequential native child controls and return to #39 for a revised decision. No handwritten DAG engine fallback is authorized.

## Failure semantics and user surface

| Failure | Persisted outcome / action |
| --- | --- |
| Invalid proposal or no authority | Reject before execution with structured validation error, no graph/child Run. |
| Model/tool refusal, approval denial | Reuse existing Service semantics; node reports bounded failure; dependent nodes do not start. |
| Node error / parent cancellation | Mark graph Run failed/cancelled and fence pending/running descendants; unaffected independent nodes may already have completed, whose results remain visible. No implicit retry. |
| Crash between checkpoint and Journal | Recovery reconciles stable operation IDs and committed events; emits missing projection once or fails closed, never redoes an unknown write. |
| Corrupt or different-version checkpoint/prompt | Fail closed with recoverable diagnostic and terminal/blocked state chosen by the existing Service state machine; never infer completion from Eino bytes. |
| Mail admitted but not consumed at interruption/restart | Keep the same message ID and pending receipt against ChildSessionID; report pending/failed/expired truthfully. Do not silently drop it or mark it consumed merely because a Run ended. |
| Child Session closed or parent authority revoked | Reject new mail/follow-up; resolve pending messages and descendant Runs under the recorded close policy, never deliver under stale authority. |
| Parent Session deletion | Fence all activations, cleanup session-owned child records/checkpoints under existing deletion rules; orphan repair conformance tests. |

Existing `child/*` RPC and RunInspector remain the initial interaction seam. Proposed graph `workflow/propose,start,get,list,cancel` is a **contract sketch**, gated by Story 01 and exact UI/host ownership review; no public method is advertised before it is functional. Show bounded node states and provenance from host projection, no fake local graph state. Expose child Session history through scoped retrieval; hide task mask hint and private authority details when projecting public events. Review `docs/architecture/VIVY-FACE-PACK.md` before changing Face contracts. Internationalize added UI copy in `ui/src/i18n/en.ts` and `zh.ts`.

## Verification and delivery gates

| Gate | Required evidence | Consequence if absent |
| --- | --- | --- |
| G0 technical feasibility | ORCH-01 recovery foundation, then real Service/Eino parallel join, approval, cancellation, checkpoint identity and process-restart matrix under D15. Completed results are reused; unknown effects stop without replay. | Missing foundation/evidence is BLOCKED; a reproduced native incompatibility is NO-GO. Both keep downstream work gated; neither authorizes a fallback engine. |
| G1 child parity | Parent/child authority, readonly tools, per-parent caps, same-origin budget accounting, parent cancellation, durable wait, clean isolated conversation, masked prompt snapshot. | Do not switch production worker path. |
| G2 graph safety | Cycle/limit rejection, idempotent node admission, no replayed write, bounded fanout and dependent-failure behavior across both SQL backends. | No public workflow start. |
| G3 product acceptance | Host/UI shared control operations, list/inspect/interrupt/follow-up, stable tree and graph projection, accessibility and two locales. | Keep gated/hidden surfaces. |
| G4 release | `just ci`, focused native integration tests, UI end-to-end at `http://127.0.0.1:3015`, `docs/logs/YYYY-MM-DD-slug/` summary/verification/acceptance, update `docs/TODO.md` §0.1. | Do not mark complete. |

## Decision register

| Key | State | Evidence needed / owner |
| --- | --- | --- |
| D1 child Session separation for task history | Design recommendation; requires G0/G1 and data lifecycle review | Service/storage owner: migration, read isolation, delete fence, historic same-Session child compatibility. |
| D2 child context fork/share | **OUT OF SCOPE · D10 clean context only** | No parent transcript, completed-turn fork, or implicit shared context. |
| D3 named catalog mask inheritance | **NOT APPROVED** | Separate product decision; free-text mask hint remains task-scoped. |
| D4 parent-terminal follow-up | **APPROVED** | A new active Run in the same original parent Session may reauthorize continuation after origin parent Run termination. Preserve origin Run lineage; effective authority is current authority intersected with original ceiling. Parent Session deletion fences continuation. |
| D5 swarm/peer messages | **DEFERRED** | Membership, task claiming, conflict resolution, authority and termination contract. |
| D6 external ACP/SDK/CLI delegation and optional #40 workflow/PTC | **DEFERRED** | Separate authority/hosting/product proposals. |
| D7 parallel join + approval/resume in native Eino | **UNVERIFIED** | Story 01 executable proof; hard gate. |
| D8 direct parent↔child mailbox delivery and acknowledgement contract | **APPROVED · REQUIRED** | Complete durable direct parent-child mailbox required. Stable ChildSession address, message ID/idempotency key, per-recipient inbox order/cursor, durable lifecycle/receipt, safe-point consumption, restart-safe pending state and at-least-once delivery. Admission ack is not completion; no exactly-once effects or cross-direction total order. Each recipient is capped at 128 messages per ChildSession lifetime and each body at 32 KiB. Continuable children reply through `reply_parent`; one-shot and workflow children cannot use it. D5 peer/swarm remains deferred. |
| D9 interrupt, cancel, close and descendant lifecycle | **PARTIAL · OPEN** | Compatibility floor: interrupt stops only the current activation and preserves Session, queued mail and descendants. Preserve legacy `child/cancel`; separately specify its relationship to Session close, pending mail and descendant teardown. Review against domain Run states. |
| D10 context | **APPROVED · clean context only** | Explicit task, admitted direct messages, approved dependency outputs only. No hidden transcript, personality or history; no context fork. |
| D11 write-capable children and workspace isolation | **PARTIAL · NARROW ONE-SHOT COMPATIBILITY** | Continuable children and workflow nodes remain read-only (except direct `reply_parent` messaging). The existing one-shot Service path can select a parent-authorized effectful tool under the existing approval policy and a private child workspace. This does not authorize general writable or concurrent children; those still need workspace ownership/isolation, conflict, integration and rollback evidence. |
| D12 model/tools | **APPROVED · parent model only** | Child uses parent model. Tool selection may narrow but never widen the immutable parent ceiling. Mask/mail cannot grant capability. |
| D13 descendant resource/cost roll-up and admission recovery | **VERIFY** | Prove full-tree budget aggregation, actual token/tool usage projection, restart-safe child slots/backpressure and partial result accounting without double counting. |
| D14 child modes | **APPROVED** | One-shot and continuable modes coexist. Legacy synchronous agent and DAG nodes default one-shot/non-addressable. Only explicit continuable mode has stable ChildSession and follow-up/mail. |
| D15 recovery and G0 prerequisite | **APPROVED · IMPLEMENTED PARTIALLY · G0 BLOCKED** | Run-scoped durable tool-operation admission/claim/result transitions use paired SQL schemas; the `tool.operation` Journal lifecycle event carries hashes and state only, while the private SQL row retains invocation bytes. Existing `tool.requested` and approval-required event payloads keep their prior contracts. A full `go test ./...` passed before the final broker-policy fix; focused post-fix app/runtime/storage tests, provider conformance, `go vet ./...` and UI checks pass. The final full-suite rerun was blocked by automatic review after a test attempted an unapproved request to `api.deepseek.com`; it was not retried. The bounded Eino Workflow covers approval while a sibling runs, cancellation, prompt/engine validation and same-checkpoint resume. Current `PolicyDeny` overrides prior approval in broker execution and replay. The repository `just ci` recipe remains unverified because `just` and PowerShell are unavailable; PostgreSQL DSN-backed conformance is unverified. Unknown effects block replay. No arbitrary external exactly-once or guaranteed automatic continuation claim. |

## Reference pointers

- [#39 initial proposal and decision comments](https://github.com/ProjectViVy/agent-vivy/issues/39), [#40 optional workflow product](https://github.com/ProjectViVy/agent-vivy/issues/40).
- [#39 inventory follow-up](https://github.com/ProjectViVy/agent-vivy/issues/39#issuecomment-5754140743) identifies bounded parent-child messages as a gap candidate; it does not choose a mailbox implementation.
- The same historical inventory records interrupt vs cancel/close, optional context fork, descendant resource roll-up, writable workspace isolation, model/tool/context selection, GUI controls, and pending-mail recovery as review topics. For this contract freeze, any context fork and child model selection are superseded by approved D10 clean-context-only and D12 parent-model-only rules.
- The same inventory distinguishes continuable children from one-shot task runs; a mailbox-capable recipient needs the former's durable Session.
- `docs/eino-capability-verify.md` is historical Eino checkpoint evidence; it does not establish this composite workflow path.
- `docs/superpowers/specs/2026-09-21-mask-subsystem-design.md` defines run-stable mask snapshots and excludes child catalog inheritance.
- Source anchors: `internal/app/agenttool.go`, `internal/app/worker.go`, `internal/worker/`, `internal/runtime/{service,engine,checkpoint,checkpointadapter}.go`, `internal/rpc/control.go`, `internal/storage/contracts.go`, `ui/src/components/chat/RunInspector.tsx`.
