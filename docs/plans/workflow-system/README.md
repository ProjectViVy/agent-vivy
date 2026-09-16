# Vivy Workflow System Design (issue #40)

> **Status:** Design proposal for
> [#40 — Optional cold-pluggable workflow system with Agent authoring](https://github.com/ProjectViVy/agent-vivy/issues/40).
> Design approved in session on 2026-09-16. Not yet an implementation
> authorization; WF-1 scheduling is a separate decision.
>
> Related: [#39 — core Agent DAG / bounded multi-agent orchestration](https://github.com/ProjectViVy/agent-vivy/issues/39)
> (this design defines the seam #39 will implement), #38 (evolution
> pipelines, independent), #35 (AutoDream, independent).

## 1. Summary

An optional, cold-pluggable workflow product layer: typed workflow
definitions as reusable product objects — defined, stored, validated,
versioned, invoked, and inspected — executed through a single explicit
orchestration seam supplied by the #39 core track. No second engine, no
feature-owned scheduler, no parallel Journal.

Approved posture (2026-09-16):

1. **Contracts first, seam for #39.** This design fixes the workflow
   product contracts plus one narrow Vivy-domain interface
   (`PlanExecutor`). The #39 track supplies the implementation later.
2. **First-slice node catalog = `model` + `agent` + `io`.** Control and
   data flow live on edges; no mode markers. Further node kinds arrive
   later through the normal Module/Generation path.
3. **No triggers in the first slice.** Invocation is by user (RPC) and
   authorized Agent (Tool surface) only; a single invocation entry is
   reserved for future trigger owners.

## 2. Reference findings (OpenFang)

The inspected reference (`morediva/.workspace/openfang`, Rust,
Apache-2.0/MIT) informed the product intent and the listed pitfalls:

- Workflow = ordered `steps: Vec<WorkflowStep>` with `StepMode`
  markers (`FanOut`/`Collect`/`Conditional`/`Loop`); not an
  edge-driven DAG (`crates/openfang-kernel/src/workflow.rs`).
- No versioning: `PUT` overwrites in place and the on-disk JSON goes
  stale (update) or resurrects the workflow on restart (delete).
- Runs live only in memory (cap 200, eviction); no checkpoint, no
  durable run store; the run endpoint blocks synchronously up to 1h.
- The visual builder holds real graph state but `saveWorkflow`
  flattens it to a steps array, dropping edges, conditions, and
  `output_var`; its TOML export is parsed by nothing.
- No agent-authored workflow facility (only agent manifests via a
  setup wizard).

Vivy borrows the product shape (definitions, runs, history, invocation
by users and agents) and rejects the execution model.

## 3. Architecture overview and composition path

Four new surfaces, all on existing seams:

| Surface | Location | Role |
|---|---|---|
| Definition domain + validation + compiler | `internal/workflow` | `Definition`, `Node`, `Edge`, `NodeKind`, five-check validator, definition→`WorkflowPlan` compiler. Pure Go, no Eino. |
| Host wiring | `internal/app/assembly_workflow.go` | `WorkflowHost` wiring, sibling of `assembly_tools.go`. |
| Persistence | `internal/storage` (`WorkflowStore` in `contracts.go`, SQLite + Postgres) | Immutable definition versions, workflow run index/projection. |
| Orchestration seam | `internal/domain` | `WorkflowPlan` types + `PlanExecutor` interface. Implemented by #39 (Eino adaptation confined to `internal/runtime`). Until then wired to an explicit Unavailable stub. |

Composition follows the v1 path. First-party `model`/`agent`/`io`
providers ship with the module; the port `std/workflow-node@v1`
(cardinality `0..n`, Consumer `WorkflowHost`) enters the catalog as
`PLANNED` and only becomes `SUPPORTED` with its seven artifacts
(Definition, SDK Contract, Host Consumer, real Provider, Failure
Model, Conformance Suite, Inspect Projection). External node Modules
enter only through explicit Recipe source pins and a new sealed
Generation.

Cold-pluggability is exercised at Recipe level: a Recipe without the
workflow module produces an Assembly with no `WorkflowHost` wiring, no
`workflow.*` RPC methods, and no Agent tools, while chat and core
orchestration remain intact.

Explicitly not built: a second engine, a workflow-owned scheduler, a
parallel Journal, any v0 compatibility layer. The single `Service.Run`
/ Journal / Policy path and protected-tool reservations are unchanged.

## 4. Definition model and the five checks

`WorkflowDefinition` (JSON, `schema_version: 1`):

- `id` (readable slug), `title`, `description`;
- `inputs`: parameter declarations (name, type, required);
- `nodes[]`: `id`, `kind` (`model` | `agent` | `io`), `config`
  (per-kind: model = model profile / system prompt / sampling; agent =
  task prompt / tool grants / context scope; io = mapping/template),
  `inputs` (upstream references and template expressions),
  `timeout_ms`;
- `edges[]`: `from`/`to` — both control and data flow live on edges;
  no implicit mode markers;
- `outputs`: terminal output mapping.

Versioning: saving a definition appends an **immutable snapshot**
(monotonic `rev` + content `hash`); updates never mutate history. A
run pins definition hash + capability identity + Generation manifest
hash; later definition changes never affect past runs.

The five checks run at save and again at invoke (capabilities may have
drifted); agent-authored definitions are untrusted input:

1. **Schema** — JSON Schema validation.
2. **Topology** — acyclic, single entry, all nodes reachable, edge and
   mapping references resolve, no cyclic io references.
3. **Capability** — every node kind has an installed provider in the
   current Generation; model nodes reference configured profiles.
4. **Budget** — node count, parallelism, total token/time ceilings;
   excess is rejected.
5. **Authority** — agent-node tool grants are a subset of the
   invoker's authority; narrowing only, never widening. Prompts are
   not a permission mechanism.

## 5. Execution model, seam, durability

Invocation sequence (owned by `WorkflowHost`):

1. Load the definition — explicit version or latest, then pin.
2. Re-run the five checks.
3. Compile to `WorkflowPlan` — frozen topology, per-node resolved
   capability identity, mappings, budget envelope.
4. Create a root run on the existing run lifecycle and Journal;
   `agent` nodes execute as existing `RunKindChild` child runs
   (`ListRunTree` traces the full tree); `model`/`io` nodes are
   step events inside the root run, not independent runs.
5. Hand the plan to the `PlanExecutor` seam.

Seam contract (pure domain interface in `internal/domain`):

```go
type PlanExecutor interface {
    ExecutePlan(ctx context.Context, run RunID, plan WorkflowPlan,
        events PlanEventSink) (PlanResult, error)
}
```

Minimal dependency constraints recorded for #39:

- Execute nodes honoring edge dependencies with bounded parallelism.
- Node dispatch calls back into kernel-owned executors: `model` via
  the ModelHost path, `agent` via the child-run path (authority may
  narrow only; parent budget inherited), `io` as pure functions.
- All effects flow through ToolHost/Policy/Approval; no new authority
  path.
- Node outcomes are synchronously observable (event sink) and durably
  recorded (Journal).
- Context cancellation stops initiating new nodes; completed-node
  evidence is preserved.
- Eino adaptation stays inside `internal/runtime`
  (`compose.NewGraph`/`NewWorkflow` are the inspected candidates on
  the v0.9.13 pin). `compose.WithCheckPointStore` is **not adopted**
  in the first slice: durability is defined by Vivy Run/Journal
  semantics; the checkpoint mapping is unsettled and is not forced in.

Until #39 lands, the default Generation wires an **Unavailable stub**:
invocation fails with an explicit `workflow.execution.unavailable`
error, no run is created, and nothing pretends success.

Failure and interruption semantics (first slice, explicit):

- Node failure → terminal run state `failed`; per-node outcomes
  (node id, error, partial outputs of completed nodes) recorded.
- No per-node retry in v1 (later: node error policies via schema
  evolution).
- Process restart → existing E2 recovery via `ListActiveRuns`: active
  workflow runs settle to `failed` (interrupted); no automatic
  resume in v1 (resume is a listed open item).
- Cancellation → `cancelled` terminal state with completed-node
  evidence preserved.

## 6. Invocation surfaces and Agent authoring

RPC (following `internal/rpc` conventions):
`workflow.list / get / validate / define / run / runs`.

Agent tools (contributed by the workflow module through ToolHost,
namespace-qualified IDs, disjoint from the reserved T1 protected
tools — no shadowing possible):

- `workflow.list` — definitions with latest version;
- `workflow.get` — definition by id/version;
- `workflow.validate` — dry-run of the five checks returning
  structured diagnostics; the authoring-loop tool
  (draft → validate → fix → validate);
- `workflow.run` — parameterized invocation under normal
  Policy/approval governance;
- `workflow.runs` — run history.

Agent authoring path: the Agent produces definition JSON, iterates via
`workflow.validate`, and saves through the existing proposal/approval
machinery (`governedProposalTool.PrepareProposal` path) — an
Agent-initiated save is an effectful proposal requiring human
approval before a version is appended. Direct human saves via RPC
follow ordinary classification. Authority: agent-node tool grants
must be a subset of the invoking run's authority, checked at validate
and invoke time; workflows invoked from an agent run inherit the
parent budget envelope.

## 7. Storage and Inspect

`WorkflowStore` (added to `storage/contracts.go`, both backends):

- `SaveDefinition(ctx, def) (rev, hash)` — append immutable version;
- `GetDefinition(ctx, id, rev)` / `LatestDefinition(ctx, id)`;
- `ListDefinitions(ctx)` — id, latest rev, title, status summary;
- `SaveWorkflowRun / GetWorkflowRun / ListWorkflowRunsByDefinition` —
  run record with runID, definition id+rev+hash, capability identity
  (node kind → provider module + Generation manifest hash), node
  outcomes, terminal state and reason.

Definitions are data stored as rows carrying the JSON payload; hashes
are content-derived. Run evidence itself stays in Journal/RunStore;
`WorkflowStore` holds only the workflow-specific index and projection —
it is not a second Journal.

Inspect projection: definitions, versions, and run history appear in
the Inspect surface. Until the seven artifacts exist, the catalog
entry for `std/workflow-node@v1` remains `PLANNED`.

## 8. Removal and unavailability semantics

- Recipe without the workflow module → no WorkflowHost wiring, no
  `workflow.*` RPC, no Agent tools; chat and core orchestration
  intact, proven by a removal test.
- **Data survives module removal**: stored definitions and run
  history remain; on reinstall, definitions re-enter the five checks
  (capabilities may have drifted) — pass or explicit rejection.
- Generation switches are deploy-time events; in-flight runs follow
  the interruption semantics.
- Capability drift: a definition referencing an uninstalled node kind
  fails the capability check at invoke with an explicit outcome; the
  definition is retained, never silently dropped.
- Reinstall compatibility: under `schema_version: 1`, readers fail
  explicitly on unknown node kinds; no speculative execution.

## 9. Testing, acceptance, phasing

Tests (deterministic, no live network): five-check matrices with
positive and representative negative cases per check; compiler tests;
dual-backend storage tests; stub-executor unavailable behavior; e2e
define → validate → run (unavailable) → runs; removal test (a
Generation without the module passes the core suite); the
proposal/approval path for agent-authored saves; authority-narrowing
test (agent-node grants must not exceed the invoker's).

Acceptance mapping against issue #40:

| Acceptance direction | First-slice state |
|---|---|
| Definition validated/saved/versioned/invoked | Met by WF-1 |
| Execution respects dependencies and mappings | Via seam; explicit unavailable until WF-2 |
| Runs retain definition version and capability identity | Met by WF-1 |
| Invalid definitions / unavailable capabilities / failed nodes / interrupted runs have explicit outcomes | Met by WF-1 |
| Generation without the module keeps chat + core DAG | Met (removal test) |
| No public Module bypasses protected tools, Host authority, single path | Met by construction |

Phasing:

- **WF-1** (this design, implementable now): domain types, five
  checks, storage, RPC, tools, stub executor, removal semantics,
  Inspect projection.
- **WF-2** (= #39 supply): real `PlanExecutor` (Eino adaptation inside
  `internal/runtime`); tracked under #39.
- **WF-3** (open, later): trigger Port, node error policies/retry,
  resume, UI editor, workflow nesting, version migration.

Eino capability check record: repository pin v0.9.13 inspected;
`compose.NewGraph` / `compose.NewWorkflow` available for the seam
implementer; `compose.WithCheckPointStore` deliberately not adopted in
the first slice (mapping to Vivy durability semantics unsettled).

Document placement: this design lives under `docs/plans/workflow-system/`;
the product-contract document `docs/architecture/VIVY-WORKFLOW.md` is
created when WF-1 ships.

## 10. Open items (recorded, not decided)

- Trigger ownership and the single invocation-entry contract shape.
- Node error policies (retry/skip) and resume-after-interruption.
- Visual editor scope (a possible UI module surface, not part of the
  system definition).
- Workflow nesting and version migration beyond `schema_version: 1`.
- Checkpoint-store mapping if long-running resumable workflows become
  a requirement.
