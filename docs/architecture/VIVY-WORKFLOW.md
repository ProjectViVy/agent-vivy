# Vivy Workflow Contract (WF-1)

> Status: **Normative**
> Decision date: 2026-09-16
> Canonical design: `docs/plans/workflow-system/README.md` and
> `DETAILED-DESIGN.md`; wire authority for the definition shape:
> `docs/plans/workflow-system/schema/workflow-definition.schema.json`
> (mirrored at `internal/workflow/schema/workflow-definition.schema.json`).
> This document records the shipped WF-1 product contract and the explicit
> WF-2/WF-3 boundary.

## 1. Scope

The workflow system is an optional, cold-pluggable product layer: typed
workflow definitions are reusable product objects — defined, stored,
validated, versioned, invoked, and inspected — through the single
`Service.Run` / Journal / policy path. WF-1 ships the definition surface
with an explicitly unavailable execution seam. It introduces no second
engine, no workflow-owned scheduler, and no parallel Journal.

- **WF-1 (shipped):** domain types, five-check validation, the
  `${{ }}` expression grammar, canonical content identity, durable
  definition revisions and run projections (SQLite + Postgres), the
  `workflow/*` RPC family, capability tokens, six generated agent tools,
  the Unavailable executor stub, and module removal semantics.
- **WF-2 (open, #39):** the real `domain.PlanExecutor` (Eino adaptation
  confined to `internal/runtime`). Until it lands, `workflow/run` returns
  RPC error `-32011` and never creates a run.
- **WF-3 (deferred):** triggers, conditional edges, node error policies,
  resume, structured (non-string) node IO, visual editor, nesting, and
  version migration.

## 2. Module membership and removal semantics

The capability enters a Generation only through the build-owned
`vivy/workflow` module (Port `core/workflow-host@v1`, provider identity
`vivy.workflow-host`, cardinality exactly one, default-on in the default
Recipe; `recipes/minimal.vivy.yml` does not select it).

- An Assembly without `vivy/workflow` has no WorkflowHost wiring, no
  `workflow/*` RPC methods (all map to `-32601 MethodNotFound`), no
  capability tokens, and no workflow agent tools. Chat and core
  orchestration are unaffected.
- Configuration cannot resurrect an omitted module: an authored
  `workflow:` config section together with an Assembly that lacks the
  module is a startup validation error.
- Data survives removal. Definitions and run history remain stored; on
  reinstall every definition re-enters the five checks at validation or
  invocation time and passes or is explicitly rejected. No definition is
  silently dropped.

## 3. Definition model and canonical identity

A definition is a JSON object with `schema_version: "1"`, a readable slug
`id`, `title`, optional `description`, optional `inputs` parameter
declarations (`string` | `number` | `boolean`, `required`), `nodes[]`
(`id`, `kind` `model` | `agent` | `io`, per-kind `config`,
`timeout_ms`), `edges[]` (`from`/`to`, carrying both control and data
flow), and optional `outputs` name/template bindings.

Every stored revision carries the **canonical JSON** of the definition:
object keys sorted recursively, insignificant whitespace removed, array
order preserved, hashed with SHA-256 as lowercase hex. Canonical bytes
are persisted exactly and identical in both backends; the hash is
content identity, not storage metadata.

Saving appends an immutable revision: monotonic `rev` plus content
`hash`. History is never mutated; a duplicate revision race maps to
`ErrVersionConflict` and first writer wins. A future run pins the
definition revision hash plus a capability-identity hash, so later
definition or capability changes never silently affect past runs.

## 4. The five checks

Validation runs at `validate`/`define` time and again at invocation
time (capabilities may have drifted); agent-authored definitions are
untrusted input. Diagnostics are structured `{check, path, message}`:

1. **Schema** — JSON Schema draft 2020-12 against the normative schema.
2. **Topology** — unique node IDs, valid edge endpoints, exactly one
   entry, acyclicity (Kahn), full reachability, unique output names,
   every `nodes.<id>.output` reference resolved by a template, and every
   such reference carried by a direct edge.
3. **Capability** — every model profile is installed, configured, and
   active in the current Generation; every node kind is known. Snapshot
   inputs are narrow host-supplied interfaces, not global registries.
4. **Budget** — node count, parallelism, node-output, and
   definition-size ceilings (`workflow:` config; defaults 32 nodes,
   4 parallelism, 64 KiB node output, 256 KiB definition).
5. **Authority** — invocation authority resolution; `readonly`
   expansion happens only at invocation time.

Expressions are limited to `${{ inputs.<name> }}` and
`${{ nodes.<node-id>.output }}`. Unknown syntax, filters, arithmetic,
and malformed braces are rejected at validation.

## 5. Storage

`storage.WorkflowStore` (definitions) and `storage.WorkflowRunStore`
(runs) live on the single `storage.Engine` in both backends — SQLite
migration 024 and Postgres schema v22 — with latest-definition indexing,
workflow-run foreign keys into existing runs/sessions, bounded
node-outcome JSON, and CN-28 conformance (immutable version race,
terminal first-writer-wins, canonical byte fidelity, backend parity).
Run evidence itself stays in RunStore/Journal; the workflow-run row is a
workflow-specific index and projection, not a second Journal.

## 6. Execution seam and the Unavailable stub

Execution is the domain seam `domain.PlanExecutor`
(`ExecutePlan(ctx, run, WorkflowPlan, PlanEventSink) (PlanResult, error)`).
WF-1 wires `workflowhost.UnavailableExecutor`, which reports
`Available() == false`. The invocation path is ordered load/pin →
re-validate → compile to a topologically ordered plan → availability
check. The availability check fires **before** any session, run, or
workflow-run row is created: `workflow/run` returns
`workflow.ErrExecutionUnavailable` and the durable state stays empty.
WF-1 leaves recovery to the existing `Service.Recover` active-run
settlement path; no WF-2 executor behavior is simulated.

The run/event vocabulary adds `RunKindWorkflow` and three non-terminal
event types with strict, secret-free payload schemas:
`workflow.invoked` (definition id/rev/hash, capability hash, input
digest), `workflow.node.started` (node id, kind), and
`workflow.node.completed` (node id, kind, status, error only). Node
output text is never journaled.

## 7. RPC contract

Methods, registered only when `ControlDeps.Workflow` is non-nil:

| Method | Params | Result |
|---|---|---|
| `workflow/list` | — | `{workflows: [{id, latest_rev, hash, title, created_at}]}` |
| `workflow/get` | `{id, rev?}` | `{workflow: {id, rev, hash, definition, created_at}}` |
| `workflow/validate` | `{definition}` | `{valid, diagnostics: [{check, path, message}]}` |
| `workflow/define` | `{definition}` | `{id, rev, hash}` |
| `workflow/run` | `{id, rev?, inputs, session_id?}` | `{run_id, status}` |
| `workflow/runs` | `{id, limit?}` | `{runs: [run summary]}` |

Error mapping (at the RPC boundary only; storage causes are preserved
internally): validation diagnostics and invalid inputs → `-32602`
with diagnostics in `Error.Data`; unknown workflow → `-32004`;
revision conflict → `-32009`; capability/authority drift → `-32009`;
`ErrExecutionUnavailable` → `-32011` with message
`workflow execution is not configured in this generation`.

Capability tokens `workflow.definitions` and `workflow.run` are
advertised by `capabilities` only when the seam is present; the UI gates
any workflow affordance on `capabilities.includes('workflow.definitions')`.

## 8. Agent tools

The module contributes six generated `std/tool@v1` providers through the
sealed Assembly and the normal ToolHost → Policy → Approval path:
`workflow_list`, `workflow_get`, `workflow_validate`, `workflow_define`,
`workflow_run`, `workflow_runs`. They are generated providers — not
protected T1 IDs — and cannot shadow or bypass the registry.

- Read-only: `workflow_list`, `workflow_get`, `workflow_validate`,
  `workflow_runs`.
- Effectful proposals: `workflow_define` (bounded summary: id, next
  revision, hash, node count) and `workflow_run` (bounded summary: id,
  revision, hash, input digest). Proposal preparation never persists or
  executes; a human approval is required before a revision is appended
  or an invocation is attempted.
- `plan` and `read_only` policy profiles deny the two effectful tools
  via the standard effect default; read-only members stay usable.
- Results are bounded by `MaxToolResultBytes`; responses never contain
  full definitions of unrelated workflows, raw journals, or node output
  text.

## 9. Public Port status

`std/workflow-node@v1` is cataloged with consumer
`core/workflow-host@v1` and remains **`SPECIFIED`/`PLANNED`**: its
support evidence (real node Provider, Failure Model, Conformance Suite,
Inspect Projection) arrives with WF-2. It must not be claimed
`SUPPORTED` before the seven-artifact standard completes. External node
Modules enter only through explicit Recipe source pins and a new sealed
Generation.

## 10. Configuration

```yaml
workflow:
  max_nodes: 32
  max_parallelism: 4
  max_node_output_bytes: 65536   # 64 KiB
  definition_max_bytes: 262144   # 256 KiB
```

Omitting the section uses the built-in defaults and — decisively — does
not make a minimal Generation appear to request the workflow capability;
membership is decided by the sealed Assembly alone. Authored sections
merge omitted fields over safe defaults; non-positive values for limits
that require positivity are rejected.
