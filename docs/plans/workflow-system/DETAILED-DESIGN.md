# Vivy Workflow System — Detailed Architecture Design (WF-1)

> **Status:** Detailed design for the WF-1 slice of
> [#40](https://github.com/ProjectViVy/agent-vivy/issues/40), deepening
> `README.md` (approved 2026-09-16) to implementation-ready contracts.
> Produced in the `feat/workflow-system-design` worktree lane. Not an
> implementation authorization; WF-1 scheduling is separate.
>
> Scope: everything implementable **now** (domain types, validation,
> storage, RPC, agent tools, stub executor, removal semantics, Inspect
> entries) plus the precise seam contract WF-2 (#39) must satisfy.
> Node executors and real orchestration are WF-2.

## 0. Corrections to README.md

Grounded against the actual codebase during this pass:

1. **RPC wire methods use slash namespaces** (`skills/list`, `cron/create`;
   `internal/rpc/control.go` method switch), not dots. Wire surface is
   `workflow/list` etc.; capability tokens stay dotted (`workflow.definitions`).
2. **Agent tool IDs use snake_case** matching the T1 precedent
   (`skills_list`, `list_dir`): `workflow_list`, `workflow_get`,
   `workflow_validate`, `workflow_define`, `workflow_run`, `workflow_runs`.
3. **Template references replace a per-node `inputs` field.** Data flow is
   expressed with `${{ ... }}` expressions inside config templates; edges
   declare dependencies and must cover every cross-node reference (§4).
4. **Conditional edges are out of scope for v1** (unconditional DAG only);
   moved to open items.

## 1. Module identity and composition

- New first-party internal module `vivy/workflow`
  (`internal/modules/workflow/module.go`), Descriptor following
  `internal/modules/loop/module.go`:
  `Provides: [core/workflow-host@v1 "vivy.workflow-host"]`,
  `Source: {Ref: "file:internal"}`, `Lifecycle: ScopeGeneration`.
  `core/*` ownership follows the `vivy/tool-host` precedent
  (`internal/moduleport/ports.go` rejects non-build providers of core ports).
- Registered in `internal/modules/optional/catalog.go` as
  `Definition{ModuleID: "vivy/workflow", Port: "core/workflow-host@v1",
  DefaultOn: true, RequiredBy: []string{"std/workflow-node@v1"}}`.
- `recipes/default.vivy.yml` gains `vivy/workflow`; `minimal.vivy.yml`
  does not. The omission is proven by extending the
  `sdk/internal/assembly/minimal_internal_test.go` pattern
  (`TestMinimalRecipeOmitsOptionalCapabilities` already fails if any module
  ending in `-host` other than `vivy/tool-host` survives the minimal recipe).
- Composition root (`internal/app/app.go`): when
  `assemblyHasModule(runtimeAssembly.Manifest.Modules, "vivy/workflow")` is
  false, no `WorkflowHost` is constructed, `ControlDeps.Workflow` stays nil,
  no `workflow.*` capability tokens are advertised, and the six agent tools
  are not registered. This mirrors the observer/skill-source nil-host
  pattern (`internal/app/assembly_observers.go`, `assembly_sources.go`).
- `validateRuntimeAssemblyConfig` additionally rejects any `workflow:`
  config section when the module is absent (same rule as MCP servers
  requiring `vivy/mcp-host`): config cannot resurrect an omitted host.

## 2. Package layout

```
internal/workflow/
  definition.go     // Definition, Node, Edge, NodeConfig kinds, canonical JSON + hash
  validate.go       // five-check validator, diagnostics
  expr.go           // ${{ }} template grammar: parse, resolve, coverage vs edges
  plan.go           // Compile(definition, resolvedCaps, budget) -> WorkflowPlan
  errors.go         // sentinel taxonomy (§8)
internal/workflowhost/
  host.go           // Host: invoke, validate, store ops; journals via storage.Engine
  executor_stub.go  // PlanExecutor returning ErrExecutionUnavailable
internal/modules/workflow/
  module.go         // Descriptor + Construct (lifecycle only; no providers in WF-1)
```

`internal/domain/workflow.go` holds the seam types (`WorkflowPlan`,
`PlanNode`, `PlanExecutor`, `PlanEventSink`, `NodeOutcome`, `PlanResult`)
so WF-2 can implement against domain contracts without importing
`internal/workflow`. No Eino imports anywhere in these packages.

## 3. Domain types

```go
// internal/workflow/definition.go
type Definition struct {
    APIVersion string  `json:"schema_version"` // always "1"
    ID         string  `json:"id"`              // slug ^[a-z0-9][a-z0-9-]{0,63}$
    Title      string  `json:"title"`
    Nodes      []Node  `json:"nodes"`
    Edges      []Edge  `json:"edges"`
    Outputs    []OutputBinding `json:"outputs"`
}

type Node struct {
    ID       string          `json:"id"`   // ^[a-z0-9][a-z0-9_-]{0,63}$
    Kind     NodeKind        `json:"kind"` // "model" | "agent" | "io"
    Config   json.RawMessage `json:"config"`
    TimeoutMS int64          `json:"timeout_ms"` // [1000, 3600000]
}

type Edge struct {
    From string `json:"from"`
    To   string `json:"to"`
}

type OutputBinding struct {
    Name     string `json:"name"`
    Template string `json:"template"`
}

// Configs (validated by schema, §4):
//   model: {profile, user_template, system?, sampling?:{temperature?, max_tokens?}}
//   agent: {task_template, tools: [name], max_turns? <= 8}
//   io:    {template}
```

Canonical form: `Definition` is marshaled with sorted object keys and no
insignificant whitespace; `hash = hex(sha256(canonical))`. Saving computes
`rev = latest + 1` inside one transaction (`UNIQUE(id, rev)` races yield
`storage.ErrVersionConflict`), following the `file_versions` precedent
(`internal/storage/sqlite/fileversions.go`).

## 4. Expression grammar (v1)

Templates are literal text containing zero or more references:

```
reference := "${{" path "}}"
path      := "inputs." name | "nodes." node-id ".output"
```

- Values are strings (node output text; input values stringified).
- Unknown grammar inside `${{ }}` is a validation error — no filters,
  no arithmetic, no code.
- Resolution rule: every `nodes.<id>.output` reference must be covered by
  an edge `<id> -> <this-node>` (direct predecessor). A reference without
  a covering edge fails topology validation — data flow may never be
  wider than the declared dependency graph.
- `inputs.<name>` must match a declared `inputs` property in the schema
  (top-level `inputs` object is part of the document; §4 of the JSON
  Schema). Missing reference at invoke time (input omitted despite being
  required) is an `InvalidParams` RPC error / tool argument error.

## 5. Validation — five checks, precise

Run at save (`workflow/define`, `workflow_define` tool) and again at
invoke (`workflow/run`, `workflow_run` tool). Diagnostics are structured:
`{check, path, message}` with `check ∈ schema|topology|capability|budget|authority`.

1. **Schema** — validate the document against
   `docs/plans/workflow-system/schema/workflow-definition.schema.json`
   using the repository's existing validator
   (`santhosh-tekuri/jsonschema/v6`, same as `tools.ValidateSchema`).
2. **Topology** — unique node ids; unique workflow outputs; every edge
   endpoint exists; exactly one entry (in-degree 0); Kahn's algorithm
   proves acyclicity; all nodes reachable from entry; template reference
   coverage per §4; `inputs`/`outputs` references resolve.
3. **Capability** — `kind` is in the installed node-kind set (WF-1:
   `model|agent|io`, fixed and always present with the module);
   `model.profile` must resolve to a configured, active provider profile
   in the ModelHost inventory (unconfigured = rejection, per the
   "missing network configuration means inactive" rule).
4. **Budget** — `len(nodes) <= workflow.max_nodes` (default 32);
   graph width (max antichain of ready nodes) `<= workflow.max_parallelism`
   (default 4, matching the child-run per-parent cap); per-node
   `timeout_ms` bounds checked by schema; definition document size
   `<= workflow.definition_max_bytes` (default 256 KiB).
5. **Authority** — at save: `agent.tools[]` entries must be names that
   exist in the tool catalog (unknown name rejected); at invoke: each
   name must be available to the invoking principal (agent run tool set
   or the user context); the reserved literal `"readonly"` expands to
   the current readonly catalog subset. Grants may only narrow.

Config knobs (`internal/config`, new `workflow:` section):
`max_nodes`, `max_parallelism`, `max_node_output_bytes` (default 64 KiB),
`definition_max_bytes`. Parse/validate tests required (AGENTS.md rule).

## 6. Storage

`internal/storage/contracts.go` gains two interfaces; both embedded in
`Engine`; implemented as methods on both `*Backend`s in
`sqlite/workflows.go` / `postgres/workflows.go`:

```go
type WorkflowDefinitionRecord struct {
    ID         string
    Rev        int64
    Hash       string
    Definition []byte // canonical JSON
    CreatedAt  int64  // unix millis
}

type WorkflowStore interface {
    SaveDefinition(ctx context.Context, rec WorkflowDefinitionRecord) error
    GetDefinition(ctx context.Context, id string, rev int64) (WorkflowDefinitionRecord, error)
    LatestDefinition(ctx context.Context, id string) (WorkflowDefinitionRecord, error)
    ListDefinitions(ctx context.Context) ([]WorkflowDefinitionRecord, error) // latest rev per id
}

type WorkflowRunRecord struct {
    RunID          domain.RunID
    WorkflowID     string
    Rev            int64
    Hash           string
    CapabilityJSON []byte // node kind -> provider module + generation manifest hash
    SessionID      domain.SessionID
    Status         domain.RunStatus
    Reason         string
    NodeOutcomes   []byte // bounded JSON; updated once at terminal
    CreatedAt      int64
    UpdatedAt      int64
}

type WorkflowRunStore interface {
    SaveWorkflowRun(ctx context.Context, rec WorkflowRunRecord) error
    GetWorkflowRun(ctx context.Context, runID domain.RunID) (WorkflowRunRecord, error)
    ListWorkflowRunsByDefinition(ctx context.Context, id string, limit int) ([]WorkflowRunRecord, error)
    SetWorkflowRunTerminal(ctx context.Context, runID domain.RunID, status domain.RunStatus, reason string, outcomes []byte) error // guarded, first-writer-wins
}
```

SQLite: append `migration024` (new const + `migrations` slice entry in
`internal/storage/sqlite/sqlite.go`):

```sql
CREATE TABLE workflow_definitions (
    id TEXT NOT NULL,
    rev INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    definition BLOB NOT NULL,
    created_at_ms INTEGER NOT NULL,
    PRIMARY KEY (id, rev)
);
CREATE INDEX workflow_definitions_latest_idx ON workflow_definitions(id, rev DESC);

CREATE TABLE workflow_runs (
    run_id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    workflow_rev INTEGER NOT NULL,
    workflow_hash TEXT NOT NULL,
    capability_json BLOB NOT NULL,
    session_id TEXT NOT NULL,
    status TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    node_outcomes_json BLOB NOT NULL DEFAULT '',
    created_at_ms INTEGER NOT NULL,
    updated_at_ms INTEGER NOT NULL,
    FOREIGN KEY(run_id) REFERENCES runs(id),
    FOREIGN KEY(session_id) REFERENCES sessions(id)
);
CREATE INDEX workflow_runs_definition_idx ON workflow_runs(workflow_id, created_at_ms);
```

Postgres: bump `schemaVersion` 21 → 22, add `schemaV22Upgrade`
(`internal/storage/postgres/schema.go`) with `BYTEA`/`BIGINT` type
mapping, `apply(22, ...)` in `migrate()` (`postgres.go`). Fresh
databases reach v22 through the bootstrap + upgrade chain.

Semantics: definitions are append-only (no update/delete in WF-1);
`SetWorkflowRunTerminal` uses the guarded-UPDATE + `RowsAffected`
first-writer-wins pattern (`SetSkillRevisionStatus` precedent), mapping
zero rows to `storage.ErrNotFound`. Errors use the shared sentinels
(`storage.ErrNotFound`, `storage.ErrVersionConflict`); JSON is
marshaled in Go, never by DB functions.

Tests: per-backend tests on temp-file SQLite /
`VIVY_POSTGRES_TEST_DSN`-gated Postgres (skip when unset); one new
conformance case `CN-28` in `internal/storage/conformance/suite.go`
(append-immutable, version race, terminal first-writer-wins,
byte-fidelity of `Definition`), bumping the `len(cases) != 27`
assertion to 28.

## 7. Run integration

**A workflow run is a root run driven by `WorkflowHost`, not a chat turn
through `Service.Run`.** Precedents: `internal/runtime/shell.go` creates
governed runs directly; `internal/app/worker.go` drives child runs.
The single-path rule is preserved because all state goes through the
same `RunStore` / `Journal` / policy surfaces — there is no second
engine or journal.

- New `domain.RunKind` value `RunKindWorkflow = "workflow"` (extend
  `Valid()`; audit `internal/app/worker.go` depth logic — workflow roots
  are depth 0 like primaries, children spawned underneath are ordinary
  `RunKindChild`).
- Session strategy: `workflow/run` accepts optional `session_id`;
  absent → the host creates a dedicated session titled
  `workflow <id>@<rev>` via `SessionStore`. This satisfies the
  `runs.session_id` foreign key and keeps history groupable.
- Lifecycle: `CreateRun(accepted)` → `SetRunStatus(active)` → executor
  drives → terminal `run.completed|failed|cancelled` event, exactly-once
  (D-008 state machine in `internal/domain/run.go` unchanged).
- Budget: root workflow run gets a `BudgetLedger` from the configured
  `BudgetPolicy` (same knobs as chat runs); workflows invoked from an
  agent run execute inside the parent's ledger chain via the existing
  `Ledger.Child`/`tighterPolicy` machinery. The plan's budget envelope
  is validated against config ceilings at compile time; the WF-2
  executor must `Reserve` event/model/tool budget per node exactly as
  the chat loop does.
- Recovery (restart, E2): workflow root runs have no pending
  approval/question checkpoints, so the existing `Service.recover`
  no-pending path settles them to `run.failed`
  (`failUnrecoverable`, `internal/runtime/service.go`) — the documented
  "interrupted ⇒ terminal failed, no auto-resume" semantics come from
  the existing mechanism, not new code.

**New Journal event types** (`internal/domain/event.go` vocabulary +
payload schemas under `schemas/events/payloads/`, all
`additionalProperties: false`):

| Type | Payload | Notes |
|---|---|---|
| `workflow.invoked` | `{definition_id, rev, hash, capability_hash, inputs_sha256}` | non-terminal, first event after `run.started` |
| `workflow.node.started` | `{node_id, kind}` | |
| `workflow.node.completed` | `{node_id, kind, status, error?}` | no output text (size); output goes to the run record |
| `run.completed/failed/cancelled` | existing payloads | terminal stays in the `run.*` family |

Node outputs are **not** journaled (C4 event cap); bounded output text
(`workflow.max_node_output_bytes`, truncate with explicit marker)
lives in `WorkflowRunRecord.NodeOutcomes`, mirroring the
content-addressed `model.completed` precedent.

## 8. Error taxonomy (`internal/workflow/errors.go`)

| Sentinel | Raised by | RPC mapping |
|---|---|---|
| `ErrSchemaInvalid` + diagnostics | validator | `-32602` InvalidParams with `data.diagnostics` |
| `ErrTopologyInvalid` + diagnostics | validator | `-32602` |
| `ErrCapabilityUnavailable` + diagnostics | validator | `-32602` at save; `-32004`-style explicit at invoke if drift |
| `ErrBudgetInvalid` + diagnostics | validator | `-32602` |
| `ErrAuthorityDenied` + diagnostics | validator/invoke | `-32602` / `403`-equivalent via `CodeConflict` per existing conventions |
| `ErrExecutionUnavailable` | stub executor | new `CodeUnavailable = -32011`, message `"workflow execution is not configured in this generation"` |
| `storage.ErrNotFound` | stores | `-32004` |
| `storage.ErrVersionConflict` | concurrent save | `-32009` |

`CodeUnavailable` is a new app code in `internal/rpc/protocol.go`
(next free: `-32011`); `internalError` continues to collapse store
strings — raw errors never cross the wire.

## 9. RPC surface (`internal/rpc/control.go`)

Optional `ControlDeps.Workflow` field (nil disables the family,
mirroring the `Skills`/`Cron` fields). Capability tokens appended in
`initialize` only when non-nil: `workflow.definitions`,
`workflow.run`.

| Method | Params | Result |
|---|---|---|
| `workflow/list` | `{}` | `{workflows: [{id, latest_rev, hash, title, created_at}]}` |
| `workflow/get` | `{id, rev?}` | `{workflow: {id, rev, hash, definition, created_at}}` |
| `workflow/validate` | `{definition}` | `{valid, diagnostics: [{check, path, message}]}` |
| `workflow/define` | `{definition}` | `{id, rev, hash}` |
| `workflow/run` | `{id, rev?, inputs, session_id?}` | `{run_id, status: "accepted"}` |
| `workflow/runs` | `{id, limit?}` | `{runs: [WorkflowRunRecord summary]}` |

DTOs use snake_case tags; `title` is parsed from the stored definition
in the list handler (no denormalized column). UI: add wrappers to
`ui/src/lib/api.ts` `RPC_METHODS` + arrow functions; capability-gated
rendering via `capabilities.includes('workflow.definitions')`.

## 10. Agent tools

Registered as generated `std/tool@v1` providers from the
`vivy/workflow` module (v1-correct path: Descriptor `Provides` entries
→ collected in `zz_default.go` → `bindGeneratedTools`; **not** the
legacy registry). `Trust: TrustPublic` (they are not protected tools);
duplicate-ID rejection and the T1 marker mechanism make shadowing
structurally impossible (`toolhost.ErrProtectedToolID`).

| Tool | Readonly | Governance |
|---|---|---|
| `workflow_list` | yes | plain |
| `workflow_get` | yes | plain |
| `workflow_validate` | yes | plain (pure function) |
| `workflow_define` | no | `PrepareProposal` (preview: id, rev, hash, changed nodes); approval via the standard `tool.approval_required` Interrupt path |
| `workflow_run` | no | `PrepareProposal` (preview: id, rev, hash, inputs digest); standard policy/approval |
| `workflow_runs` | yes | plain |

All six are subject to the normal `PolicyEngine` profile rules
(`plan`/`read_only` profiles deny the mutating two), argument
validation via `Schema`, and `MaxToolResultBytes` bounding. Tool
schemas declare `additionalProperties: false`; results are bounded
summaries, never full run journals.

## 11. The seam — WF-2 contract (#39)

`internal/domain/workflow.go`:

```go
type WorkflowPlan struct {
    DefinitionID string
    Rev          int64
    Hash         string
    Nodes        []PlanNode // topologically ordered input
    Edges        []Edge
    Outputs      []OutputBinding
    Budget       BudgetEnvelope // ceilings already validated <= config
}

type PlanNode struct {
    ID        string
    Kind      NodeKind
    Spec      json.RawMessage // resolved config: profile -> concrete provider identity
    TimeoutMS int64
}

type NodeOutcome struct {
    NodeID  string
    Kind    NodeKind
    Status  string // "completed" | "failed" | "skipped_not_started" | "cancelled"
    Output  string // bounded text
    Error   string
}

type PlanResult struct {
    Outcomes []NodeOutcome
    Outputs  map[string]string
}

type PlanEventSink interface {
    NodeStarted(ctx context.Context, nodeID string, kind NodeKind) error
    NodeCompleted(ctx context.Context, o NodeOutcome) error
}

type PlanExecutor interface {
    ExecutePlan(ctx context.Context, run RunID, plan WorkflowPlan,
        sink PlanEventSink) (PlanResult, error)
}
```

WF-2 acceptance criteria (recorded for #39):

1. Honors edge order with parallelism bounded by `Budget.Parallelism`;
   ready-set scheduling, never out-of-order execution of an unmet dependency.
2. Dispatch: `model` via the ModelHost provider path (concrete profile
   resolved at compile time); `agent` via the existing child-run
   machinery (`StartAgentTask(ctx, task, mask)` — tool-mask narrowing is
   the authority mechanism, `internal/app/agenttool.go`); `io` as pure
   template evaluation using `internal/workflow/expr.go`.
3. Every effect flows through ToolHost/Policy/Approval; the executor
   never invents an authority path. Child policy-hash equality rule
   (`worker.go`) applies unchanged.
4. Emits sink events AND journals them (`workflow.node.*`), reserving
   budget per event/model/tool call through the run's ledger chain.
5. Per-node `TimeoutMS` enforced with context deadlines; ctx cancellation
   stops initiating nodes and returns partial `PlanResult`.
6. Eino adaptation confined to `internal/runtime`
   (`compose.NewGraph`/`NewWorkflow` are the inspected v0.9.13
   candidates; `WithCheckPointStore` remains unadopted — durability is
   Vivy Run/Journal semantics, restart settles to `run.failed`).
7. Idempotence of evidence: re-delivery of a sink event must not create
   duplicate Journal rows (Journal seq already guarantees this).

WF-1 wires `workflowhost.executor_stub.go` implementing `PlanExecutor`
by returning `ErrExecutionUnavailable`; `workflow/run` fails with
`-32011` **before** `CreateRun` (no orphan runs).

## 12. Inspect and port catalog

- `docs/architecture/VIVY-PORT-CATALOG.md` gains the internal-port row
  `core/workflow-host@v1` (owner `vivy/workflow`, `CardinalityExactlyOne`)
  and the public-port row `std/workflow-node@v1` with v1 state
  `PLANNED` — no `SUPPORTED` claim until the seven evidence artifacts
  exist (`sdk/internal/assembly/evidence.go` ledger).
- `internal/moduleport/ports.go` `PublicCatalog` gains the
  `std/workflow-node@v1` entry so Recipe validation accepts (and
  tracks) future providers; validation of an actual provider stays
  closed until evidence lands (WF-3+).
- Studio `Inspect` (`internal/studio/inspect.go`) needs no change: the
  sealed manifest gains `vivy/workflow` and the six tool names
  automatically.

## 13. Test plan (WF-1)

| Area | Tests |
|---|---|
| Schema/validator | matrix per check: positive + ≥2 negatives each (bad slug, cycle, uncovered template ref, missing profile, over-width graph, unknown tool name) |
| expr | reference parsing, coverage, stringification, missing-input error |
| Canonicalization | stable hash across key order; rev race → `ErrVersionConflict` |
| Storage | sqlite temp-file; postgres gated by `VIVY_POSTGRES_TEST_DSN`; CN-28 parity case |
| RPC | `NewControlHandler` with fake deps: nil-dep → MethodNotFound; validate/define/run error mappings incl. `-32011` before run creation |
| Tools | spec schemas; proposal preview content; readonly denials under `read_only` profile |
| Removal | minimal recipe test extension; default-recipe build without `vivy/workflow` → no capability token, no tools, chat suite green |
| Config | `workflow:` parse/validate; refusal when module absent |
| Determinism | no live network anywhere |

## 14. Open items (unchanged from README plus additions)

- Conditional edges / branch routing (new — explicitly out of v1).
- Trigger Port and the single invocation-entry contract.
- Node error policies (retry/skip) and resume-after-interruption.
- Structured (non-string) node inputs/outputs.
- Visual editor scope; workflow nesting; version migration beyond v1.
- Checkpoint-store mapping if resumable workflows become required.

## 15. Evidence base

Grounded against (worktree `feat/workflow-system-design` @ `9385e1f`):
`internal/rpc/{protocol,control}.go` (method switch, ControlDeps,
capability advertisement, error codes); `internal/storage/contracts.go`
+ `sqlite/sqlite.go` (numbered migrations) + `postgres/{schema,postgres}.go`
(schemaVersion chain) + `sqlite/fileversions.go` (immutable version
precedent) + `conformance/suite.go`; `internal/domain/{tool,run,runmode,event,generation}.go`;
`internal/tools/tools.go` (protected list, registry); `sdk/port/tool`;
`internal/app/{assembly_tools,agenttool,worker}.go`; `internal/modules/{defaults,optional}`;
`sdk/internal/assembly/{compiler,evidence,minimal_internal_test}.go`;
`internal/runtime/{service,budget}.go`; `recipes/*.yml`;
`schemas/events/payloads/`.
