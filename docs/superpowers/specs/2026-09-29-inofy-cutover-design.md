# INOFY Cutover in VIVY — Detailed Architecture

Date: 2026-09-29  
State: detailed design for review; no code or integration acceptance  
Decision record: [agent-vivy #23](https://github.com/ProjectViVy/agent-vivy/issues/23)

## 1. Outcome and boundaries

VIVY will use the embedded INOFY Go library as its only task-graph execution engine. The graph invokes native VIVY child Runs. VIVY remains authoritative for the parent/child Run tree, continuable child sessions and mailbox, policy and tools, budget, Journal, storage, UI authorization and approval. This cutover replaces the newly introduced DAG contract without preserving old graph definitions, tool payloads or workflow checkpoints. It does not migrate existing chats, delete historical evidence, or change direct child delegation.

| ID | Requirement | Observable result |
| --- | --- | --- |
| R1 | One graph engine | New task graphs execute through INOFY; no production path compiles a second VIVY workflow graph or chooses an engine. |
| R2 | Native agent ownership | Each Agent call is a VIVY child Run with existing lineage, scoped context, policy, budget, cancellation and result rules; child sessions and mailbox remain outside graph ownership. |
| R3 | One executable schema | New task graphs use inofy.workflow/v1. The host admits only its supported node catalog and bounded subset; old integer-version DAG documents fail explicitly. |
| R4 | One durable authority | Graph state, CommitID/epoch, redacted events, protected outputs and checkpoint references commit under VIVY Core Storage; no INOFY App database. |
| R5 | Honest recovery | No orphan checkpoint becomes visible, duplicate effects are reconciled against native child evidence, stale writers lose, and ambiguous outcomes report recovery_required. |
| R6 | Clean cutover | First-party tool/RPC/UI consumers move with the runtime, old graph code is removed, historical normal sessions remain usable and old workflow state never auto-runs. |
| R7 | Future conversation boundary | A workflow node denotes one child activation, not an entire child session; entering or messaging a child session cannot silently change a compiled graph. |

The reusable definition catalog, draft/publish flow, visual editor, Garden integration and new interactive-child UX have separate acceptance. A run-local task graph does not require those products. Ordinary chat and direct child delegation do not become INOFY workflows. Initial task graphs retain their current one-shot, read-only children and their existing ceilings. INOFY's additional switch, select, repeat and wait support is not automatically exposed by the VIVY task tool.

The smallest adequate design is an INOFY library call behind VIVY's current workflow admission, one host NodeExecutor and one host RunStore. An optional engine mechanism would add permanent duplicated conformance work; making INOFY's standalone App mandatory would duplicate credentials, HTTP and persistence. No second agent loop, scheduler, service, database or graph format is introduced.

## 2. Inspected implementation and exact seam

Source baselines: VIVY 3c4ed66826f5a40fd27dba0d9e208c55240a5955; INOFY dbfebcec2be4d29aadeb2b553c5786a118b98fb6. Recheck current heads before implementation.

| Existing path / symbol | Observed behavior | Design disposition |
| --- | --- | --- |
| VIVY internal/runtime/workflow.go, executeWorkflowGraph | Direct Eino compose.NewWorkflow, mapped string outputs, VIVY checkpoint adapter | Replace graph construction and invocation with INOFY Compile and Program.Run; retain useful child-task and output-boundary helpers only where used. |
| VIVY internal/runtime/workflow_service.go, StartWorkflow / runWorkflowNode | Atomic host admission, workflow Run, one-shot children, lifecycle events, recovery and inspection | Retain host admission and governed child primitives, adapt to INOFY's definition and node contract, assign terminal ownership once. |
| VIVY internal/orchestration/descriptor.go and internal/tools/workflow.go | Separate integer-version DAG schema with max 12 nodes / 24 edges and read-only child tools | Retire old executable schema; change tool validation and affected first-party callers together. |
| VIVY internal/runtime/child_oneshot.go / child_sessions.go / child_activation.go | Idempotent stable child Runs, continuable sessions and safe-point mail | Reuse. The cutover neither invents nor replaces a session runtime. |
| VIVY internal/storage/workflow_revisions.go and migrations/*/033_workflow_revisions.sql | Immutable per-Run descriptor admission, unique parent plus operation key | Reuse table and admission transaction, with additive migration for new program identity and an explicit legacy discriminator. |
| VIVY storage.Journal.Append and SnapshotStore.Put | Each has its own transaction; Journal lacks CommitID and writer epoch | Neither alone satisfies INOFY RunStore.Commit. Extend a host workflow transaction in both database backends. |
| VIVY storage.BlobStore.Put/Get | Generation-based opaque storage; Put does not return a generation reference | Stage content-addressed checkpoint/result blobs under unique workflow-prefixed IDs, verify bytes and digest, then commit references. Do not rely on a mutable current-generation key. |
| INOFY types.go / program.go | Compile and Program.Run; NodeExecutor.Execute, RunStore.Commit/Load; initial Run performs an empty-to-admitted commit even when a host has already created a Run | Map the first INOFY admission transition onto the already admitted VIVY Run without a second native run.started event. |
| INOFY internal/einoruntime/workflow.go / repeat.go | Pinned Eino v0.9.13 Workflow/Graph, checkpoint staging, branch and repeat compilation | Reuse its graph implementation; do not write another ready queue. |

Only VIVY internal/runtime imports executable INOFY. Its storage contracts expose host types and no Eino/INOFY types; the adapter translates at the runtime boundary. The INOFY root module is an embedded dependency pinned to an immutable version/commit; apps/inofy is not imported. VIVY's Eino imports remain confined to internal/runtime and internal/provider.

## 3. Executable definition and admission

The sole executable input is an INOFY Definition (schema_version inofy.workflow/v1). The existing workflow tool name can remain but its argument schema changes in the same release; no legacy descriptor conversion remains. Its task-graph catalog initially exposes one trusted node type, vivy.child-task@1. A call node's validated config carries a bounded task string and requested tool names. Its input is an object of explicitly bound predecessor outputs, and its output is an object with a bounded result string. An order-only edge creates no implicit data access. No authored config is executable code or authority.

The tool schema must represent the supported definition subset, and the server must run the canonical INOFY decoder/validator regardless of tool-side validation. Add one exported INOFY accessor, proposed as DefinitionSchema() json.RawMessage, backed by INOFY's canonical executable schema. The VIVY tool projects its supported node subset and host limits from that schema/catalog; it does not hand-maintain another topology grammar. Implement this small library enabling change before changing the model-facing tool. The host advertises actual catalog/feature limits, not every INOFY node kind. UI callers derive capability truth from the host.

Admission order:

1. Resolve parent and root Run, session, Generation, immutable policy snapshot, operation key and current tool ceiling from trusted VIVY context. Keep depth, no-nested-workflow and read-only child-tool rules.
2. Strictly decode the Definition, normalize it using INOFY, and validate topology, node type/config, binding schemas and effective limits. Default task limits are the existing 12 nodes, 24 edges, four outputs, input/output size bounds and active-child ceiling. Host settings only narrow INOFY's limits.
3. For each Agent call, intersect requested tools with the parent ceiling at admission. Recheck immediately before child admission. The graph cannot grant tool names or attach a new Session.
4. Compile with the frozen trusted catalog. Persist definition bytes, definition digest, catalog/program digests, compiler and Eino identities, effective limits, input digest and VIVY authority binding with the workflow Run and native run.started event in the existing atomic admission transaction.
5. Launch Program.Run with the admitted ref and VIVY NodeExecutor/RunStore. Authoring a changed graph or input requires a new admitted Run, not mutation of a compiled one.

VIVY's workflow_revisions row is repurposed for newly admitted Definitions. Retain its existing descriptor_json and descriptor_digest byte/digest fields for the normalized Definition bytes; the host row schema_version integer becomes a storage discriminator: 1 means old DAG and 2 means INOFY. The executable schema version remains the Definition string inofy.workflow/v1 and is checked independently. Add nullable columns in a paired append-only SQLite/PostgreSQL migration for program_digest, catalog_digest, compiler_version, eino_build, input_digest, effective_limits and host_binding_id. Old rows remain readable as historical records but fail any new execution/resume path. New rows require all new identity fields at admission; a legacy row cannot be misclassified by missing fields.

No workflow product catalog is required for run-local task graphs. Later reusable published definitions resolve an immutable revision and pass through the same host admission and execution boundary.

## 4. Node-to-child contract

VIVY's runtime adapter implements INOFY NodeExecutor.Execute(ctx, NodeCall) (NodeReply, error). It accepts only vivy.child-task@1 with the pinned trusted ImplementationID. The adapter validates Ref, graph path, config and input, confirms the active host authority, constructs the native bounded task (dependency outputs are explicitly labeled untrusted), and calls the existing one-shot child path. It returns an output object only after ChildRunDetails confirms the child completed and its bounded summary is available.

The idempotency key is INOFY NodeCall.OperationKey (currently RunID plus logical path). The VIVY child Run ID is a deterministic workflow_child_ plus SHA-256 of workflow Run ID, a separator and that key. This preserves the existing reserved ID form and distinguishes future repeat iterations. The admission record and workflow projection bind operation key, graph path, selected child ID and admitted task digest; a repeated key with different inputs/config is an idempotency conflict. The execution attempt number is not a license to create a second child for the same logical operation.

Register this node as non-replayable for the first cut. Do not enable INOFY automatic retry or error fallback around an Agent effect. If the child is already terminal, load and verify its durable result. If it is active in the same process, wait on that existing Run; if a prior process may have crossed the model boundary, reconcile native child lifecycle and checkpoint evidence. An unresolved effect yields UnknownOutcomeError / recovery_required; never blindly invoke the model again. INOFY commits the node-attempt record before Execute and the protected result before letting dependent nodes run. The adapter must not emit an independent graph-node terminal event that races this commit.

A task graph uses one-shot children for now. Directly entering a continuable child Session is already a separate VIVY capability. A later interactive workflow node may bind one specific child activation, with its own admission and result. Workflow completion/cancellation cannot erase that session or consume mailbox messages outside the native safe point.

## 5. Transaction, lifecycle and event mapping

INOFY RunStore.Commit(ctx, ExecutionRef, RunCommit) must be adapted to a new narrow VIVY Core Storage operation, proposed as CommitWorkflowStep(ctx, WorkflowStepCommit) (WorkflowStepReceipt, error), plus LoadWorkflowStep(ctx, runID). The storage package owns these host-typed records; runtime maps INOFY structs at its import boundary. WorkflowStepCommit includes RunID, writer epoch, CommitID, digest of canonical commit content, expected and target INOFY state, native redacted events, protected result references, and optional checkpoint reference. The receipt contains committed event sequence and projection revision. Exact Go field names are proposed for the implementation plan; they do not create a public plugin Port.

A single database transaction in each driver must:

- Lock the workflow Run/projection; verify immutable program/host binding, epoch, state transition and terminal invariant.
- Check the unique (run_id, commit_id) record. Same content digest returns its original receipt; different content rejects. A stale writer or conflicting resume claim rejects before an effect can start.
- Append native Journal events with contiguous seq, update the workflow projection (including wait/resume claims and references), and advance the native Run row when appropriate.
- Commit before publishing events. If it fails, no state transition or reference is visible.

New workflow_execution and workflow_commits rows are transaction-local projections/commit receipts, not a second authoritative history. The Journal and protected blob references committed alongside them remain the reconstructible evidence; no separate INOFY SQL store is introduced. Result bytes and checkpoint bytes are stored outside event payloads under content-addressed, workflow-scoped blob IDs. Stage and verify them first; a failed SQL commit leaves only unreachable blobs. Load verifies digest and compatible program/input/policy identities before exposing bytes to INOFY.

Lifecycle mapping is explicit:

| INOFY transition | VIVY effect |
| --- | --- |
| empty → admitted | The VIVY Run already exists in accepted state from CommitWorkflowAdmission. Record one workflow admission projection/event, without creating a second Run or another run.started. |
| admitted → running | Advance accepted → active and append a workflow.started event in the same transaction. |
| running → running | Append redacted node attempt/completion/failure evidence and protected result references. |
| running → waiting | Keep native Run active, mark workflow projection waiting and atomically expose checkpoint and waits. A native child approval checkpoint, if supported later, stays separately owned by the child Run. |
| waiting → running | Atomically claim the next writer epoch and resume-answer identity before continuing effects. |
| running → succeeded/failed/cancelled | Commit exactly one native run.completed/run.failed/run.cancelled terminal event and matching Run status. The old launchWorkflow terminal emitter must not commit a second terminal. |
| running → recovery_required | Keep the native Run nonterminal with an explicit workflow recovery_required projection/event; exclude it from automatic replay until reconciled. |

Native Run active plus graph waiting/recovery_required represents one host-owned nonterminal Run with a more precise workflow projection, not two competing lifecycle authorities. The workflow inspector combines these committed facts and never infers success from an in-memory return value.

Important current-contract wrinkle: INOFY Program.Run issues its own initial empty-to-admitted commit. The adapter must accept that exact transition against a VIVY Run already atomically admitted, once only, using the workflow projection's initial absence as its expected state, with initial ExecutionRef.Epoch set to 1. RunStore.Load reports admitted from the native accepted Run and absent workflow projection; the first commit initializes that projection under the same epoch. A generic Journal.Append followed by SnapshotStore.Put is insufficient. This behavior must be proven with both real storage drivers before production routing is switched.

## 6. Crash, cancellation, and legacy policy

On startup, route active INOFY workflow Runs by the row discriminator. Reload and verify the frozen Definition, ProgramMeta, host binding, and INOFY projection. A durable waiting checkpoint may resume only through the authorized host path with a writer-epoch claim and exact outstanding answers. A previously running process may have executed a child; use its deterministic operation key and native child evidence to reconcile. Where evidence cannot establish the outcome, keep recovery_required and do not replay. A damaged/missing referenced blob fails closed. Never use old VIVY Eino graph checkpoint bytes as INOFY checkpoint bytes.

For initial read-only task nodes, INOFY SupportsWait remains false. A native child approval or interactive wait is not silently converted to a workflow answer. Introducing a wait-capable child node later requires an explicit host-owned continuation mapping and a separate real-path acceptance gate. General INOFY wait/resume coverage belongs to reusable workflow integration; the migration gate proves recovery classification and does not falsely claim cross-process continuation of a child model call.

Workflow cancellation stops new node admissions, propagates context cancellation to graph-owned active one-shot child Runs, and commits a single native terminal outcome when possible. It does not delete child sessions, unrelated children, or historical run evidence. A client disconnect follows the existing host Run semantics. Event or storage failure after a child effect yields an unresolved outcome until reconciliation; it never produces a success response solely from memory.

No backward compatibility is promised for old integer-version graphs, tool requests or graph checkpoints. Old rows remain under existing retention. Exclude them from auto-recovery and respond with unsupported legacy format on inspection/resume attempts that require the new execution projection; ordinary sessions and non-workflow Runs continue normally. Do not rewrite migration 033 or reset the database. A binary rollback after new-format admissions cannot be represented as safe cross-engine resume.

## 7. Performance and economy

INOFY and the current graph implementation both use Eino v0.9.13. This design does not claim a speed gain. Compile the immutable definition at admission and rebuild only when loading a stored Run for controlled recovery; do not add a global cache in the first cut. Runtime node calls still pay the existing model and child-Run costs; new work is the required durable pre-effect and result commits. Measure compile cost, short read-only graph wall time/allocations, and SQL writes during acceptance if performance becomes a decision, separating model time from scheduler overhead.

Remove obsolete host graph-construction code and descriptor-specific schema/validation rather than maintain two engines or a translation bridge. Keep only a small runtime adapter, a storage transaction extension in both drivers, and the catalog/schema surface the model-facing tool actually consumes. No new external service, worker or runtime plugin system is justified.

## 8. Verification gates and delivery order

| Gate | Required evidence |
| --- | --- |
| G1 — Schema and authority | New tool payload and host catalog agree; legacy/unknown-node/oversized/invalid graph rejected; child tools cannot widen, depth and existing bounds hold. |
| G2 — Agent behavior | Parallel independent nodes and a dependent synthesis run produce correct outputs via native child Runs; ordinary direct delegation, continuable child sessions and mailbox still work. |
| G3 — One execution path | No old graph compiler reachable in production; no optional selector; a single terminal owner; INOFY root imports without App SQL/HTTP dependencies. |
| G4 — Storage parity | SQLite and PostgreSQL: initial host-admitted → INOFY-admitted mapping, duplicate/conflicting CommitID, stale epoch, terminal uniqueness, contiguous events and admission idempotency. |
| G5 — Effect faults | Crash/fault before and after child admission, child terminal before graph-result commit, blob stage before SQL failure, missing referenced blob, cancel race. No duplicate child/model invocation or false success. |
| G6 — Restart | New-format waiting checkpoint only when enabled by a real supported node; running state classified/reconciled; old graph state excluded without breaking normal sessions; incompatible program/policy identities reject. |
| G7 — Product path | Model calls the changed workflow tool, can inspect/cancel one actual graph; backend and selected VIVY UI report committed node/run state. No VIVY Studio shell integration is implied. |
| G8 — Repository gate | just ci, paired migration fresh-install/upgrade/reopen/fault cases, both database drivers, and required VIVY split-UI real-path smoke; record commands and skips under the iteration log. |

The handoff has four dependent increments: (A) Definition and tool/catalog contract; (B) native storage transaction and admission identity; (C) NodeExecutor plus production routing and recovery; (D) old-code deletion and end-to-end acceptance. B may be implemented with a fault-capable test node while A is stabilized, but no production switch is accepted until A–C have passed their gates. The detailed implementation plan must specify exact files, migration number, SQL transaction behavior and tests based on a fresh head. This document is the detailed architecture; it does not mark any increment Ready or authorize code execution by inference.

## 9. Relationship to current plans

Issue #23 records the approved overall decision. This document resolves the host lifecycle and storage boundary for that decision. INOFY's existing S11 plan still assumes legacy descriptor translation and references the separate VIVY Studio shell; revise S11 against this architecture before execution. Do not create a parallel plan that silently keeps those assumptions. Issue #8 supplies multi-agent behavior expectations; issue #9 supplies the later reusable-workflow product requirements; issue #21 remains the cross-line status index.

Source evidence:

- [VIVY workflow execution](https://github.com/ProjectViVy/agent-vivy/blob/3c4ed66826f5a40fd27dba0d9e208c55240a5955/internal/runtime/workflow.go) and [workflow service](https://github.com/ProjectViVy/agent-vivy/blob/3c4ed66826f5a40fd27dba0d9e208c55240a5955/internal/runtime/workflow_service.go)
- [VIVY storage contracts](https://github.com/ProjectViVy/agent-vivy/blob/3c4ed66826f5a40fd27dba0d9e208c55240a5955/internal/storage/contracts.go), [revision admission](https://github.com/ProjectViVy/agent-vivy/blob/3c4ed66826f5a40fd27dba0d9e208c55240a5955/internal/storage/workflow_revisions.go) and [one-shot child](https://github.com/ProjectViVy/agent-vivy/blob/3c4ed66826f5a40fd27dba0d9e208c55240a5955/internal/runtime/child_oneshot.go)
- [INOFY public contracts](https://github.com/ProjectViVy/INOFY/blob/dbfebcec2be4d29aadeb2b553c5786a118b98fb6/types.go), [Program.Run](https://github.com/ProjectViVy/INOFY/blob/dbfebcec2be4d29aadeb2b553c5786a118b98fb6/program.go) and [node commit path](https://github.com/ProjectViVy/INOFY/blob/dbfebcec2be4d29aadeb2b553c5786a118b98fb6/execution.go)

Evidence level: source inspection and contract reasoning. No new Go test, database fault test or browser smoke has been run for this design.
