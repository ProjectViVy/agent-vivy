# Verification — S11-E cutover

Focused suite (new real-path tests, all GREEN):

```
go test ./internal/runtime -run 'INOFY|Workflow' -count=1
ok  agent-vivy/internal/runtime  2.281s
```

Covered: admit→run→inspect end-to-end (2-node dependent graph, `a` parallel
edge into `join`, `outputs.answer` bound from `join`'s committed result blob,
child run ids `workflow_child_…`, exactly one terminal event); dedup join by
operation identity and conflict on changed definition; invalid/legacy payloads
rejected with `ErrINOFYInvalidDefinition` and nothing persisted; cancel
propagation (child cancelled, engine classifies `recovery_required`, run stays
non-terminal, duplicate start fenced); restart recovery classification
(running→`recovery_required`, no fabricated native terminal); admitted-crash
resume completing with 2 governed children; schema-1 rows skipped by recovery
and rejected by inspection.

Full gate:

```
PATH=/usr/local/go/bin:$PATH VIVY_POSTGRES_TEST_DSN=postgres://postgres:vivy@127.0.0.1:5432/vivy?sslmode=disable just ci
fmt-check ui-ci vet test headless-compile plugin-ci  — all green
go test -timeout 20m ./... — every package ok (runtime 51s, sdk/internal 199s)
VIVY_POSTGRES_TEST_DSN=… just test-postgres — ok internal/storage/postgres 11.7s
```

Conformance re-pin: internal source digest `ee268a…` → `7b210587…` in
`sdk/internal/assembly/conformance_results.json` (5 internal-provider rows);
`sdk/internal/conformance` green after re-pin.

Storage upgrade/reopen/fault coverage: sqlite + postgres `workflow_steps`
suites exercise admission→commit→result→checkpoint round trips, epoch/stale
writer fences, dedup, expected-status conflicts, and reopen persistence; both
drivers green inside `just test` / `just test-postgres`.

Source audit (no old graph compiler production-reachable):
`rg 'compose\.New'` → only `internal/runtime/orchestration.go` (native
orchestration proof). Its producers (`nativeOrchestrationDescriptor`,
`executeNativeOrchestrationWorkflow`) are referenced solely from `*_test.go`;
the sole live entry point `isNativeOrchestrationResumeTarget` can only fire on
an approval resume target of form `native-orchestration:<id>`, which no
production path ever persists. Remaining `compose.*` usages are chat-loop
primitives (ToolsNodeConfig, GetToolCallID, IsInterruptRerunError).
Old workflow route files `internal/runtime/workflow.go`,
`workflow_test.go`, `internal/orchestration/validation_test.go` deleted;
`internal/orchestration/descriptor.go` reduced to limit constants.

Split-UI real-path smoke (AGENTS gate) — PASSED on the real dev pair
(`go run ./cmd/vivy` :8787 + `pnpm dev` :3015, fresh `VIVY_USER_HOME` temp
dir honoring ST-2 air gap, frozen-env mock provider):
`workflow/start` over `/rpc` WS ran a 2-node `a→join` workflow through
governed child runs to `engine_status:"succeeded"`; `workflow/get` returns
`definition` + `engine_status` + `nodes` + `outputs`, no `descriptor` key,
legacy descriptor params rejected `-32602`; Run Inspector renders the
committed projection (nodes, revision digest, outputs) with no JS errors.
`engine_status` display line added to the inspector afterward (i18n en/zh).

# Verification — S11-F definitions + host actions

Focused suite (storage + runtime + rpc, GREEN on sqlite and postgres):

```
go test ./internal/storage/... ./internal/runtime ./internal/modules/... ./internal/actionhost/... ./internal/rpc/... -run 'Definition|WorkflowProduct|INOFY' -count=1
VIVY_POSTGRES_TEST_DSN=… go test ./internal/storage/postgres/ -run 'Definition' -count=1
```

Covered: draft CAS (create-only sentinel, stale-etag conflict,
create-or-overwrite empty etag), author isolation (foreign read/write/publish
rejected), publish dedup + monotone revision allocation + immutable stored
artifact, unknown node type rejected at validate/publish, revision→run
identity binding (definition_id/revision + canonical input on the admitted
revision), draft-etag snapshot starts, operation-key dedup/conflict,
event paging cursor, node output from committed blobs, session-scoped
getRun/events/cancel (foreign → not-found), cancel through the governed
path, honest capabilities (wait/resume unsupported), disabled product
surface keeping `workflow/start` on the core path, RPC session binding and
error-code surface.

Full gate:

```
PATH=/usr/local/go/bin:$PATH VIVY_POSTGRES_TEST_DSN=… just ci
fmt-check ui-ci vet test headless-compile plugin-ci — all green
```

Fixes made during the gate: migration count assertions 34→35 with
`workflow_definitions` metadata + new tables; i18n cross-face contract
classified `runInspector.workflowEngineStatus`; conformance re-pin to the
new internal source digest (5 internal-provider rows).
