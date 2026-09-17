# WF-1 Workflow System Implementation Plan

> For the implementation session, use `superpowers:subagent-driven-development` when independent lanes are available; otherwise use `superpowers:executing-plans` task by task with review checkpoints.

**Goal:** Ship the scheduled WF-1 workflow product slice as an optional, cold-pluggable Vivy capability: validated immutable workflow definitions, SQLite/Postgres persistence, the sealed `vivy/workflow` Assembly module, control-plane RPC and capability tokens, six governed workflow tools, and an explicit unavailable execution seam.

**Architecture:** Keep workflow definitions, expression parsing, validation, and plan compilation in pure Go under `internal/workflow`. Keep the future execution contract in `internal/domain` and host/storage/policy wiring in `internal/workflowhost` and `internal/app`. The WF-1 executor always returns `ErrExecutionUnavailable`; it must not create a run or introduce a second scheduler, Journal, policy path, or runtime. The default Recipe selects the workflow module; the minimal Recipe does not, and an omitted module removes its host, RPC family, capability tokens, and generated tools.

**Tech Stack:** Go, existing `storage.Engine` SQLite/Postgres backends, JSON Schema draft 2020-12 through `github.com/santhosh-tekuri/jsonschema/v6`, existing JSON-RPC control plane, generated v1 Assembly, existing ToolHost/Policy/Approval path, and the existing Vite/TypeScript API layer.

**Spec:** `docs/plans/workflow-system/README.md`, `docs/plans/workflow-system/DETAILED-DESIGN.md`, and `docs/plans/workflow-system/schema/workflow-definition.schema.json`.

## Scope and non-goals

- Implement WF-1 only. WF-2's real `PlanExecutor` and WF-3 triggers, branching, retry/skip, resume, structured values, editor, nesting, and migration remain deferred.
- Keep `std/workflow-node@v1` in the public catalog as `PLANNED`; do not claim it is supported or add an external provider.
- Do not edit generated Assembly files by hand. Regenerate them from the default Recipe and source catalog.
- Do not import Eino from `internal/workflow`, `internal/workflowhost`, `internal/domain`, `internal/storage`, `internal/rpc`, tools, or UI. Only the existing `internal/runtime`/`internal/provider` quarantine may import Eino.
- Do not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/` during implementation or verification.

## Eino capability check

The repository is pinned to `github.com/cloudwego/eino v0.9.13` and the relevant EinoExt modules in `go.mod`. The inspected future orchestration candidates are `github.com/cloudwego/eino/compose.NewGraph` and `compose.NewWorkflow`; the existing runtime also has the ADK runner/checkpoint path. Those APIs belong to WF-2 and will be adapted only behind the `internal/runtime` boundary when real execution is scheduled. WF-1 has no missing Eino capability to replace: its validator, immutable definition store, policy/authority checks, and unavailable stub are Vivy product contracts, so no custom Eino substitute is introduced now. The WF-2 checkpoint mapping remains deferred because WF-1 explicitly settles unavailable execution before a run exists.

## Global implementation constraints

- Follow the order Module -> typed Port -> Provider/Consumer -> Recipe -> generated Assembly -> evidence.
- Use `storage.ErrNotFound` and `storage.ErrVersionConflict` without replacing their cause; map errors at the RPC boundary only.
- Canonicalize definition JSON before hashing and preserve canonical bytes exactly in both backends.
- Keep all workflow limits and outputs bounded. Never put workflow inputs, node output text, tool arguments, provider keys, or raw Journal blobs in logs or event payloads unless the design explicitly allows a bounded digest.
- Mutating definition/run tools must implement the existing `tools.ProposalProvider` path. They must never call storage or execution while preparing a proposal.
- Every task below is test-first when implementation resumes: add the focused failing test, make the smallest implementation pass, then run only the relevant focused checks. No tests are run during this preparation turn at the user's request.

## Task 1: Add the pure workflow/domain contracts and expression grammar

**Files:**

- Create `internal/domain/workflow.go`.
- Create `internal/workflow/definition.go`, `expr.go`, `errors.go`, `validate.go`, and `plan.go`.
- Add focused tests under `internal/workflow/*_test.go` and `internal/domain/workflow_test.go`.

**Work:**

1. Define the domain seam exactly as the detailed design requires: `WorkflowPlan`, `PlanNode`, `NodeOutcome`, `PlanResult`, `PlanEventSink`, and `PlanExecutor`. Keep `NodeKind` and `Edge` data-only and keep `BudgetEnvelope` data-only; do not expose Eino types.
2. Define the v1 workflow document, node, edge, output binding, and node-config types. Use the schema file as the wire authority, including its `inputs` and `description` fields where the README and the abbreviated design snippet differ; do not silently omit schema fields.
3. Implement canonical JSON recursively: object keys sorted, insignificant whitespace removed, array order preserved, `json.RawMessage` normalized as JSON, and SHA-256 returned as lowercase hex. Reject malformed or non-object definitions before hashing.
4. Parse only `${{ inputs.<name> }}` and `${{ nodes.<node-id>.output }}` references. Reject unknown syntax, filters, arithmetic, code, and malformed braces. Provide deterministic template resolution for string values and a missing-required-input error at invocation time.
5. Implement the five checks with structured `{check, path, message}` diagnostics: schema, topology, capability, budget, and authority. Topology must enforce unique IDs, valid endpoints, exactly one entry, Kahn acyclicity, reachability, output uniqueness, reference resolution, and direct-edge coverage for every node-output reference. Capability and authority inputs must be narrow interfaces supplied by the host, not global registries.
6. Implement `Compile` to return a topologically ordered `domain.WorkflowPlan`, resolve model profile identities through the supplied capability snapshot, expand `readonly` only at invocation authority resolution, and copy all JSON input rather than retaining caller-owned buffers.
7. Add sentinels `ErrSchemaInvalid`, `ErrTopologyInvalid`, `ErrCapabilityUnavailable`, `ErrBudgetInvalid`, `ErrAuthorityDenied`, and `ErrExecutionUnavailable` with wrapped diagnostics.

**Tests to add before implementation:** positive definitions; at least two failures per check; bad workflow/node slugs; duplicate and disconnected nodes; cycle and multiple entries; uncovered template references; missing profile and inactive profile; over-node and over-width budgets; oversized canonical bytes; unknown and unavailable tool names; canonical hash stability across object-key order; expression parse/stringify/coverage/missing-input cases.

## Task 2: Add explicit workflow configuration and sealed-module removal behavior

**Files:**

- Modify `internal/config/config.go` and `internal/config/config_test.go`.
- Modify `config.example.yaml` if the example exposes the new operator section.
- Modify `internal/app/assembly_validate.go` and its focused tests.

**Work:**

1. Add a top-level `workflow:` configuration section with `max_nodes`, `max_parallelism`, `max_node_output_bytes`, and `definition_max_bytes`, using defaults 32, 4, 64 KiB, and 256 KiB.
2. Preserve explicit-section presence separately from defaults (a pointer or equivalent presence bit is acceptable) so an omitted section does not make a minimal generation appear to request a workflow capability. Merge omitted fields in an authored section with safe defaults, while still rejecting negative/zero values where the contract requires positive limits.
3. Make `validateRuntimeAssemblyConfig` reject an authored `workflow:` section when `vivy/workflow` is absent. Configuration must not resurrect a capability omitted by the sealed Assembly.
4. Add parse/default/invalid-value tests and tests proving the absent-module refusal without changing unrelated config validation.

## Task 3: Register the typed Port, first-party Module, Recipe, and generated Assembly

**Files:**

- Create `internal/modules/workflow/module.go` and focused module tests.
- Modify `internal/modules/optional/catalog.go` and `catalog_test.go`.
- Modify `internal/modules/defaults/catalog.go` and `catalog_test.go` to bind the separate workflow package and its generated provider constructor.
- Modify `internal/moduleport/ports.go` and tests for `core/workflow-host@v1`.
- Modify `sdk/port/catalog.go` and catalog tests for `std/workflow-node@v1`.
- Modify `recipes/default.vivy.yml`; leave `recipes/minimal.vivy.yml` without workflow.
- Modify `sdk/internal/assembly/minimal_internal_test.go` and generator tests as needed.
- Regenerate `internal/generated/assembly/zz_default.go` through the existing generator after source metadata is correct.

**Work:**

1. Add the build-owned `vivy/workflow` module with `core/workflow-host@v1` provider identity `vivy.workflow-host`, generation-scoped lifecycle, and no runtime discovery.
2. Register it as a default-on optional host with `std/workflow-node@v1` as its future activation trigger. Keep the core Port closed to non-T1 providers and cardinality exactly one.
3. Add the public `std/workflow-node@v1` definition with `core/workflow-host@v1` consumer and leave support evidence absent, so its derived state remains `SPECIFIED`/`PLANNED` rather than `SUPPORTED`.
4. Add `vivy/workflow` only to the default Recipe. Extend the minimal Recipe proof so it asserts no workflow module, workflow provider inventory, or workflow source import survives.
5. Resolve the design's lifecycle-only module wording together with the explicit §10 requirement for generated workflow tools: keep lifecycle construction in `internal/modules/workflow`, but declare the six `std/tool@v1` identities and their build-owned provider collection through the Assembly binding needed by `bindGeneratedTools`; do not hand-wire a parallel legacy registry path.
6. Regenerate the Assembly and verify the manifest contains the module and six tool identities, while the minimal generated Assembly contains neither.

## Task 4: Add durable workflow definition/run storage and CN-28 parity

**Files:**

- Modify `internal/storage/contracts.go`.
- Create `internal/storage/sqlite/workflows.go` and `internal/storage/postgres/workflows.go`.
- Modify `internal/storage/sqlite/sqlite.go` with migration 024.
- Modify `internal/storage/postgres/schema.go` and `internal/storage/postgres/postgres.go` for schema v22.
- Modify `internal/storage/conformance/suite.go` and backend conformance tests.
- Add focused SQLite tests and the existing environment-gated Postgres parity case.

**Work:**

1. Add `WorkflowDefinitionRecord`, `WorkflowStore`, `WorkflowRunRecord`, and `WorkflowRunStore` to the storage contracts and embed both in `storage.Engine`.
2. Implement append-immutable definition revisions: latest revision plus one in a transaction, canonical byte/hash fidelity, duplicate revision race mapped to `ErrVersionConflict`, deterministic list ordering, and `GetDefinition`/`LatestDefinition`/`ListDefinitions` behavior.
3. Add migration 024 and Postgres schema v22 exactly as designed, including the latest-definition index, workflow-run foreign keys to existing runs/sessions, bounded node-outcome JSON storage, and workflow/run indexes.
4. Implement workflow-run save/list/get and guarded terminal update with first-writer-wins semantics. A zero-row terminal update returns `ErrNotFound` as specified; do not overwrite an already-terminal record.
5. Add CN-28 for immutable version race, terminal first-writer-wins, canonical bytes, and SQLite/Postgres parity. Update the suite's case count and comments from 27 to 28.

## Task 5: Add run/event vocabulary and the unavailable WorkflowHost

**Files:**

- Modify `internal/domain/run.go` and `internal/domain/event.go`.
- Create `schemas/events/payloads/workflow.invoked.json`, `workflow.node.started.json`, and `workflow.node.completed.json`; update `schemas/events/run-event.schema.json` if its enum requires it.
- Create `internal/workflowhost/host.go` and `executor_stub.go` with focused tests.
- Create `internal/app/assembly_workflow.go` and focused composition tests.

**Work:**

1. Add `RunKindWorkflow` and the three non-terminal workflow event types to the domain vocabulary in canonical order. Keep existing terminal `run.*` semantics unchanged.
2. Add strict, secret-free payload schemas: `workflow.invoked` carries definition/revision/hash/capability hash/input digest; node start carries node ID/kind; node completion carries node ID/kind/status/error only. Never journal node output text.
3. Define `workflowhost.Host` over narrow storage, validation, capability, tool-catalog, policy, and executor interfaces. It owns validate/list/get/define/run/runs operations but delegates durable state to `storage.Engine` and future execution to `domain.PlanExecutor`.
4. Implement `executor_stub.go` as a `domain.PlanExecutor` that returns `ErrExecutionUnavailable` without creating a run, session, Journal row, or workflow-run row.
5. Assemble the host only when the generated manifest contains `vivy/workflow`; otherwise return a nil optional host. Use the same run/session/policy seams that the normal service uses, and leave recovery to the existing `Service.Recover` active-run settlement path.
6. Keep the WF-1 `workflow/run` path ordered as load/pin -> validate/compile -> invoke the stub -> return unavailable. The future path may create a workflow root run only after the executor is available; no WF-1 orphan row is allowed.

## Task 6: Add RPC methods, capability tokens, and error mapping

**Files:**

- Modify `internal/rpc/protocol.go` with `CodeUnavailable = -32011`.
- Modify `internal/rpc/control.go` with the optional `ControlDeps.Workflow` seam, DTOs, dispatch cases, capability advertisement, handlers, and mappings.
- Add/update RPC focused tests using the existing `newControlTestEnv` style.

**Work:**

1. Add `workflow.definitions` and `workflow.run` only when `ControlDeps.Workflow` is non-nil. Nil must make every `workflow/*` method return `MethodNotFound`, matching other optional families.
2. Implement `workflow/list`, `workflow/get`, `workflow/validate`, `workflow/define`, `workflow/run`, and `workflow/runs` with the exact snake_case wire shape. Parse list titles from stored canonical definitions; do not add a denormalized title column.
3. Return diagnostics in `Error.Data` for schema/topology/capability/budget/authority failures. Map storage not-found to `CodeNotFound`, version races to `CodeConflict`, and `ErrExecutionUnavailable` to `CodeUnavailable` with the exact user-facing message.
4. Verify `workflow/run` with the stub returns `-32011` before `CreateRun` is observable, and that invalid inputs fail before any store mutation.

## Task 7: Add the six generated, governed Agent tools

**Files:**

- Create `internal/tools/workflow.go` and focused tool tests.
- Add the build-owned provider definitions/factory in `internal/modules/workflow/module.go` or its package-local provider file.
- Modify `internal/tools/tools.go`, `internal/app/app.go`, and `internal/app/assembly_tools.go` only where needed to conditionally inject workflow operations and preserve generated-tool binding.
- Add/update app/toolhost governance tests.

**Work:**

1. Implement `workflow_list`, `workflow_get`, `workflow_validate`, `workflow_define`, `workflow_run`, and `workflow_runs` with strict object schemas and `additionalProperties: false`.
2. Keep list/get/validate/runs read-only. Define a bounded proposal containing ID, revision, hash, and changed-node summary; run a bounded proposal containing ID, revision, hash, and input digest. Proposal generation must not persist or execute.
3. Route actual calls through `WorkflowHost` operations and the existing `bindGeneratedTools` -> ToolHost -> Policy/Approval path. Mark the generated providers `TrustPublic`; they are not protected IDs and must not bypass the normal registry or governance path.
4. Ensure `plan`/`read_only` policy profiles deny `workflow_define` and `workflow_run`, while readonly tools remain usable. Bound result summaries by `MaxToolResultBytes`; never return full journals.
5. Inject these implementations only when the sealed workflow module exists. The default Assembly exposes them through generated providers; a minimal Assembly must not accidentally register them as legacy tools.

## Task 8: Add the typed UI API surface without starting the editor

**Files:**

- Modify `ui/src/lib/api.ts` with method names, wire interfaces, and request wrappers.
- Modify `ui/src/lib/api.test.ts` with method/argument mapping checks.

**Work:**

1. Add all six RPC method names to `RPC_METHODS` without duplicates.
2. Add typed wrappers for list/get/validate/define/run/runs using the exact snake_case DTOs.
3. Gate any workflow capability rendering/accessor on `capabilities.includes('workflow.definitions')`. Do not add the visual workflow editor or node interaction in WF-1; that remains WF-3.

## Task 9: Contract documentation, iteration evidence, and TODO closure

**Files:**

- Create `docs/architecture/VIVY-WORKFLOW.md` from the shipped contract.
- Update `docs/architecture/VIVY-PORT-CATALOG.md` with the core host and planned public Port.
- Update `docs/TODO.md` only after all WF-1 acceptance criteria are complete, moving WF-1 to the completion log while leaving WF-2/WF-3 open.
- Create `docs/logs/2026-09-16-workflow-wf1/summary.md`, `verification.md`, and `acceptance.md` for the delivered slice.

**Work:**

1. Document the optional module/removal behavior, definition schema/revisions, five checks, RPC/tool contracts, storage semantics, unavailable stub, and explicit WF-2/WF-3 boundary.
2. Record every verification command and result. In the current preparation turn, record no test result because the user explicitly deferred testing; the delivery log must be updated with the actual focused checks and product gate when implementation is complete.
3. Include human acceptance steps: default generation advertises and lists workflow tools, definitions round-trip canonically, invalid definitions show structured diagnostics, minimal generation has no workflow capability, mutating tools produce approval proposals, and `workflow/run` returns `-32011` without creating a run.

## Verification sequence after implementation resumes

1. Run focused pure-domain/config/storage/RPC/tool tests for the task just completed; keep Postgres gated by `VIVY_POSTGRES_TEST_DSN`.
2. Run the Assembly generator and inspect the generated diff; confirm no generated file was hand-edited and the minimal Recipe remains free of workflow imports.
3. Run `just ci` from the repository root as the product gate. This is intentionally not run in the current preparation turn per the user's instruction.
4. For the user-visible API, start the split pair (`just run` and `cd ui; pnpm dev`) and exercise `http://127.0.0.1:3015`; do not use the embedded UI or Studio as the inner-loop verification path.
5. Review `git diff --check`, inspect the iteration log, stage only WF-1 paths plus its docs/evidence, and make a focused human-attributed commit. Leave the unrelated `ui/src/routeTree.gen.ts` working-tree state untouched unless its owner explicitly resolves it.
