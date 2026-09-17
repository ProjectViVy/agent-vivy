# 2026-09-16 — Workflow WF-1 slice

## What changed

Shipped the scheduled WF-1 workflow product slice (issue #40) in the
`feat/workflow-wf1` worktree lane, per
`docs/superpowers/plans/2026-09-16-workflow-wf1.md` and the approved design in
`docs/plans/workflow-system/`:

- **Pure domain (`internal/workflow`, `internal/domain/workflow.go`):** the
  `WorkflowPlan`/`PlanNode`/`PlanOutcome`/`PlanExecutor` seam; the v1
  definition document; recursive canonical JSON + SHA-256 content identity;
  the `${{ inputs.* }}` / `${{ nodes.*.output }}` grammar; the five-check
  validator (schema, topology, capability, budget, authority) with structured
  `{check, path, message}` diagnostics; the topological plan compiler; and
  the six sentinel errors. No Eino imports.
- **Storage (`internal/storage`):** `WorkflowDefinitionRecord`,
  `WorkflowStore`, `WorkflowRunRecord`, `WorkflowRunStore` on
  `storage.Engine`; SQLite migration 024 and Postgres schema v22;
  append-immutable revisions with `ErrVersionConflict` on revision races;
  guarded first-writer-wins terminal updates; CN-28 (suite grew 27 → 28)
  covering version race, terminal projection, canonical byte fidelity, and
  SQLite/Postgres parity.
- **Module & Assembly:** build-owned `vivy/workflow` module
  (`core/workflow-host@v1`, provider `vivy.workflow-host`) registered
  default-on in the optional catalog with `std/workflow-node@v1` as its
  future activation trigger; the six generated `std/tool@v1` provider
  identities bound through the Assembly (`bindGeneratedTools` path, no
  parallel legacy registry); `recipes/default.vivy.yml` selects the module,
  `recipes/minimal.vivy.yml` provably does not; `zz_default.go` regenerated
  by the generator (not hand-edited).
- **Host (`internal/workflowhost`, `internal/app`):** `WorkflowHost` over
  narrow storage/validation/capability/executor seams; capability snapshots
  carry declarative model-profile and tool-catalog state only (no secrets);
  `UnavailableExecutor` reports unavailable and the `Run` path refuses
  **before** any session, run, or workflow-run row exists; app wiring lives
  in `internal/app/assembly_workflow.go` and arms only when the sealed
  manifest contains `vivy/workflow`.
- **RPC (`internal/rpc`):** `workflow/list|get|validate|define|run|runs`
  registered only when the optional `ControlDeps.Workflow` seam is non-nil
  (nil ⇒ `-32601` for the whole family); `CodeUnavailable = -32011` with the
  exact user-facing message; diagnostics in `Error.Data`; storage
  not-found/conflict mapped at the boundary only; capability tokens
  `workflow.definitions` / `workflow.run`.
- **Tools (`internal/tools`):** six governed tools with strict
  `additionalProperties: false` schemas; read-only list/get/validate/runs;
  `workflow_define`/`workflow_run` implement `ProposalProvider` with bounded
  previews and no persistence/execution during proposal preparation;
  `plan`/`read_only` policy profiles deny them via the standard effect
  default (asserted in `internal/runtime/policy_test.go`).
- **Events & schemas:** `RunKindWorkflow` plus `workflow.invoked`,
  `workflow.node.started`, `workflow.node.completed` in canonical order with
  strict, secret-free payload schemas (input digest only; node output text
  never journaled).
- **Config (`internal/config`):** top-level `workflow:` section
  (`max_nodes`/`max_parallelism`/`max_node_output_bytes`/`definition_max_bytes`,
  defaults 32 / 4 / 64 KiB / 256 KiB) with authored-section presence
  tracking; `validateRuntimeAssemblyConfig` rejects an authored `workflow:`
  section when the sealed Assembly omits `vivy/workflow`;
  `config.example.yaml` documents the section.
- **UI (`ui/src/lib/api.ts`):** six RPC method names, typed snake_case DTOs,
  wrappers, and `-32011 → 503 unavailable` mapping; capability accessors are
  expected to gate on `capabilities.includes('workflow.definitions')` (no
  editor UI — WF-3).
- **Conformance evidence:** `sdk/internal/conformance/reproduction_test.go`
  and `sdk/internal/assembly/conformance_results.json` re-pinned to the new
  `internal/` source digest after the slice landed (first-party providers
  share the `internal/` tree digest).
- **Support gate (`sdk/internal/support_state_test.go`):** the
  every-public-Port-is-`SUPPORTED` release gate gained a commented
  `plannedPublicPorts` exemption for `std/workflow-node@v1`, whose
  seven-artifact evidence arrives with WF-2
  (`docs/architecture/VIVY-WORKFLOW.md` §9). The exemption asserts the
  opposite: a planned Port must never evaluate `SUPPORTED`, and it is
  skipped by the evidence-removal mutation loop. The catalog entry, its
  `SPECIFIED`/`PLANNED` state, and all product code are unchanged; the
  test-only edit is confined to `sdk/`, so the `internal/` source digest
  is untouched.
- **Docs:** new product contract `docs/architecture/VIVY-WORKFLOW.md`;
  `docs/architecture/VIVY-PORT-CATALOG.md` lists `core/workflow-host@v1`
  (internal, default-on) and `std/workflow-node@v1`
  (`SPECIFIED`/`PLANNED`); `docs/TODO.md` WF-1 row moved to
  `docs/COMPLETE.MD`, WF-2/WF-3 remain open. The iteration log
  (`docs/logs/2026-09-16-workflow-wf1/`) records the green `just ci` run
  and the split-pair real-path smoke.

## Explicitly not done

- **No executor.** WF-2's real `PlanExecutor` (Eino `compose.NewGraph` /
  `NewWorkflow` adapted inside `internal/runtime`) is untouched; `workflow/run`
  returns `-32011` and creates nothing.
- **No `std/workflow-node@v1` support claims** and no external node provider.
- **No WF-3 scope:** triggers, conditional edges, retry/skip, resume,
  structured values, visual editor, nesting, version migration.
- **No denormalized title column** — `workflow/list` parses titles from
  stored canonical definitions.
- The unrelated `ui/src/routeTree.gen.ts` working-tree entry (CRLF-only
  normalization noise, no content diff) was deliberately left unstaged.
