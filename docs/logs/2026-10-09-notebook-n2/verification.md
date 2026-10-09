# N2 — verification

## Optional module (Task 1)

- `sdk/internal/assembly` notebook suite: selected Provider emits `NotebookFactory` + accessors + typed binding; omission emits closed accessors with no `internal/modules/notebook` or `notebookcontract` import; factory-without-port, provider-without-factory, and duplicate providers each rejected with named errors. `TestGenerateRuntimeAssembly*` notebook cases green.
- `internal/app/assembly_notebook_test.go`: default assembly exposes the factory and serves a real `CreateEntry` through the sqlite backend; omitted manifest → nil bundle; `ForSession` gates empty/unknown/known sessions (`ws.v1:<id>`); mistyped `NotebookFactoryValue` → construction error; closed bundle → mutation rejected.
- `internal/moduleport`: `core/notebook-service@v1` registered in the canonical closed catalog (AtMostOne, Conditional, owner `vivy/notebook-core`).

## Trusted actions (Task 2)

- `internal/actionhost/notebook_test.go`: human → Home + `peer:` actor; run-bound → `ws.v1:` + `run:` actor; unarmed fails closed; unauthenticated writes nothing; foreign session denied.
- `internal/app/notebook_authority_test.go` (real RPC + sqlite, no UI):
  - `TestNotebookOriginCannotBeForged` — JSON claiming `origin`/`actor`/`scope_id`, and nested claims inside `request`, rejected by strict schema; list afterward proves nothing persisted.
  - `TestNotebookHumanEditAdmittedUnderDefaultProfile` — direct authenticated write admitted; same `operation_key` replay reconciles to `"replayed":true` (response-loss path).
  - `TestNotebookExplicitDenyStillDenies` / `TestNotebookExplicitPromptRuleStaysDecisive` — operator rules beat the exemption both directions.
  - `TestNotebookAgentInvocationKeepsFrozenPolicy` — `turn/start`-bound invocation is denied under the default profile (no human exemption for in-run callers).
  - `TestNotebookHeadlessActionExercise` — sections create/list, entry create, CAS save, stale-CAS error, get, revisions list, comments create/list, export — full §7 surface over `module.action.invoke`.
- `internal/modules/notebook/module_test.go`: factory requires store+scopes; scope/actor bound by facade; operation key required; closed bundle fails; tool actor `tool:<id>`; tools fail closed when bundle inactive; forged authority fields rejected; descriptor sealed.

## Tool rebinding + provenance (Task 3)

- `internal/tools`: `Builtin(nil)` leaves note names unregistered (no dead stubs); `Builtin(backend)` resolves them — engine surface test uses a real sqlite backend.
- `internal/runtime/notebook_provenance_test.go`: `TestNotebookProvenanceSurvivesCompaction` — admission stamps notebook provenance durably, replayed admit returns the stamped row, projected `tool.finished` message inherits the mark, plain rows untouched, folded compaction summary is conservatively tainted.
- `internal/observerhost`: `TestRunObserverExcludedRunSkipsDeliveryButAdvancesCursor` — excluded run delivers zero events while the durable cursor still advances.
- `internal/storage/conformance` CN-45 (sqlite + postgres): flagged op persists both markers, `HasExcludedToolOperations` false→true, messages round-trip, survives `Reopen`, no retroactive taint.
- `internal/storage/migrations`: manifest/runner counts bumped to 37; migration 37 metadata pinned (`notebook_provenance`).

## Generations (Task 4)

- `go run ./sdk verify plugins/vivy-notebook` → `ok vivy/notebook`.
- `go run ./sdk pack` + `inspect-artifact` on three recipes into disposable dirs:
  - `default` → modules `vivy/notebook`, `vivy/notebook-core`, `vivy/notebook-tools`; tools `list_notes`/`read_note`/`write_note`.
  - `notebook-backend` → `vivy/notebook-core` + `vivy/notebook-tools` + tools; no UI module.
  - `no-notebook` → zero notebook modules, zero note tools (physically absent, not merely inactive).
- `go generate ./internal/generated/assembly` regenerated `zz_default.go` (never hand-edited); `internal` sourceSha256 re-pinned to `9789af36…` (5 rows) and `TestCheckedInProviderConformanceMatchesExecutedSuites` green.
- `go test ./internal/app ./internal/actionhost ./internal/modules/notebook ./internal/runtime ./internal/observerhost ./internal/tools ./internal/storage/...` — all green incl. Postgres (vivy-pg:55432).
- N0 marker `TestServiceDoesNotInjectNotebook` still green.
- `just ci` — full suite green (see acceptance).
- Known flake (pre-existing, not N2): `TestAppShutdownBounded` intermittently fails `TempDir RemoveAll` on a leftover `shutdown.db-journal`/persona `garden.db` WAL file — a shutdown-drain timing race in the test environment. Evidence: 9/10 standalone passes; full `internal/app` package green on rerun; N2 added no writer on that path (the new observer hook is a read-only `SELECT` and the notebook bundle opens no files).
