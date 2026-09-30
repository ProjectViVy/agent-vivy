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

S11-G covered: bridge injects session_id into every call and omits it
fail-closed; startRun adds parent_run_id + bounded operation_id;
TransportError status mapping for not_found/revision_conflict→412,
idempotency→409, validation→422, unsupported→501, unauthenticated→401;
journal-event normalization for pages and notifications, cursor dedup,
terminal close, and resubscribe-from-cursor after stream_error;
extension registration (route `/workflows`, grouped nav, `workflow`
icon); shadow-mount page drives the real editor App against a stubbed
HostBridge listing workflows with session-bound calls.

Full gate:

```
PATH=/usr/local/go/bin:$PATH VIVY_POSTGRES_TEST_DSN=… just ci
fmt-check ui-ci vet test headless-compile plugin-ci — all green
ui: 513 vitest tests pass incl. 22 under ui/src/generated/ui/vivy-workflow
```

Fixes made during the gate: `internal` source digest re-pinned to
`ab2fbb02…` (5 rows in `conformance_results.json`), baseline Generation
inventory gained `vivy/workflow-ui`, i18n gate gained the `VENDORED.md`
vendored-tree exclusion, `host-icons`/`HOST_ICON_NAMES` gained `workflow`,
`repoSourceDirs`/`generate-default`/`default-generation.expected.json`
wired the module, `source.sha256` converged at `6abd8344…`,
`inofyRunEvents` items emit journal `type` (not `kind`), gofmt repaired
`internal/storage/workflow_definitions.go`.

# Verification — S11-G smoke defect fixes

Three defect layers surfaced by the browser smoke, each fixed and
re-verified in the real product UI with zero live patches.

**Upstream (INOFY `docs/s11-vivy-cutover-plan`, pinned by go.mod):**

- `98526b8` + `586f4b5`: the shared editor now authors named exit
  outputs — checking a node as exit binds `outputs[id] =
  {source:id, pointer:""}` and the properties panel exposes a rename
  field. Vendored byte-verbatim; `edit.test.ts` (5 tests) runs from the
  staged copy.
- `4def2ae`: `Binding` tracks pointer-key presence (`HasPointer`) so an
  authored root pointer `""` survives the typed decode → marshal →
  re-validate cycle `SaveDraft` performs — previously `omitempty`
  dropped the key and the canonical re-marshal failed `missing_binding`.
  `go test ./...` green upstream, incl. new `binding_test.go`.

**Host-side fixes:**

- `internal/runtime/inofy_store.go`: `inofyRunStore.Commit` now fans each
  committed event out on `s.publish` after the journal accepts the step
  (journal stays authority; laggards re-sync from it). Wired in
  `launchINOFYWorkflow`.
- `WorkflowStepReceipt` gains `Replayed bool` on both drivers plus a
  conformance assertion, so an idempotent re-commit never re-publishes.
- `internal/rpc/control.go`: `streamRun` re-subscribes before replaying
  the journal tail after a bus drop — a laggard can no longer miss
  events committed between drop and resubscribe.
- `WorkflowPage`: StrictMode/unmount cleanup removes only the nodes its
  own effect appended (a replayed mount was being wiped).
- `TestWorkflowProductRootPointerOutput` (save → re-save → publish of a
  `pointer:""` draft) and `TestINOFYStoreCommitPublishesCommittedEvents`
  and `TestRunSubscriptionResubscribesAfterBusDrop` guard all three.

Focused suite + full gate:

```
go test ./internal/runtime ./internal/rpc ./internal/storage/... -count=1 — green (sqlite + postgres)
PATH=/usr/local/go/bin:$PATH VIVY_POSTGRES_TEST_DSN=… just ci
fmt-check ui-ci vet test headless-compile plugin-ci — all green
ui: 518 vitest tests pass; module source.sha256 ef631023…;
internal source digest re-pinned to 0bdcb5af…
```

Browser smoke rounds (real split pair, frozen-env mock provider,
fresh `VIVY_USER_HOME`):

- rec-2 (`…/rec-2c44f612-…-edited.mp4`): mount-wipe defect found+fixed;
  ledger stall root-caused to the missing bus publish.
- rec-3 (`/home/ubuntu/screencasts/rec-7e73555b-…/…-edited.mp4`): mount
  fix verified (`shadowRoot.children===2`); open detail streamed live
  through seq 10 incl. `node_failed` + `recovery_required` on cancel;
  save still dead-ended at `missing_binding` (defect 4).
- rec-4 (`/home/ubuntu/screencasts/rec-e444d004-…/…-edited.mp4`): the
  UI-authored graph (2 `vivy.child-task@1` nodes, edge, exit checked,
  output renamed `answer`) **saves** (`草稿已保存`), **validates**
  (`校验通过`), **publishes** (`修订 r1`), and the published revision ran
  `workflow_27152173e4ee3c0e` to `engine_status:"succeeded"` with
  `outputs.answer` resolving node2's whole result packet via the empty
  root pointer. Cancel on hanging r2 streamed `node_failed` +
  `recovery_required` live to seq 10. Screenshots
  `ss_7e0bacce.png` (published r1 + output field), `ss_edca5794.png`
  (ledger to `run.succeeded`), `ss_4811561d.png` (cancel tail live).
  Logs `/home/ubuntu/s11e-smoke/backend-g4.log`, `vite2.log`.

Known product gap (pre-existing, unchanged by S11-G): the editor's
运行 button binds `state.currentRun` as `parent_run_id`, which is often
stale → `-32603`; runs started via `inofy.startRun` with an active
parent work correctly. Needs a product decision (bind the session's
active run or resolve it backend-side).

# Verification — S11-G revision (VIVY-native editor)

- `pnpm vitest run src/generated/ui/vivy-workflow`: 28/28 (4 files —
  editor, face-bridge, page, studio edit/graph) on the staged tree.
- `pnpm typecheck`: clean.
- Browser smoke on the split pair at `127.0.0.1:3015` (fresh VIVY_USER_HOME,
  marker-routed mock on :11434):
  - rec-5 (`/home/ubuntu/screencasts/rec-10f81b43-…/…-edited.mp4`):
    golden path on the VIVY-native editor — seed `begin` node, add
    second `vivy.child-task@1`, task edit, edge, exit + output `answer`,
    save → validate → publish r1 → run `succeeded` with output bound →
    cancel streamed `node_failed` + `recovery_required` live. Two
    selection defects surfaced (below). Screenshots
    `ss_99f56759.png` (succeeded run + outputs), `ss_7c3113e3.png`
    (cancel live stream), `ss_f0e67462.png` (crash evidence),
    `ss_f74e529e.png` (published), `ss_256b6f29.png` (workflows tab),
    `ss_2460492c.png` (canvas + panel).
  - rec-6 (delta, post-fix): save-while-selected → "Draft saved." 2×
    clean (was 100% `Maximum update depth exceeded`); panel follows
    node1→node2 clicks; pane click + Escape deselect; drag + edge +
    save stable; validate diagnostics render; zero console errors.
    `ss_cf47ba4d.png`.
- `just ci` (PATH incl. /usr/local/go/bin, VIVY_POSTGRES_TEST_DSN set):
  green — all Go suites on both drivers, UI vitest 519, plugin-ci
  incl. source-digest re-pin (`a0fcd1ea…`).

Defects found and fixed this round:

- Render-time `selected:true` injection into the controlled `nodes`
  prop → infinite setState on save + permanently stuck first selection.
  Fixed by carrying selection in canvas state via `reproject()` and
  letting `applyNodeChanges`/`onSelectionChange` own it.
- `applyArtifact` cleared selection on every save — kept via a
  `preserveSelection` path used only by save/validate/publish/run.
