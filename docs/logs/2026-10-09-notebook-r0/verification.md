# R0 Verification

Postgres backend used: `postgres://postgres:vivytest@127.0.0.1:55432/vivy_test`
(`vivy-pg` container, `VIVY_POSTGRES_TEST_DSN` set for every storage run below).

## R0-focused suites

- `go test ./internal/storage/sqlite ./internal/storage/postgres -run 'TestReport.*Admission|Test.*WorkflowAdmission' -count=1` — PASS both dialects.
  Covers `TestReportRootAdmission` (commit, replay/Rejoined, same-key
  different-digest idempotency conflict, busy target, other-target
  unaffected, validation rejects, child-under-parent-run namespace) and
  `TestReportRootAdmissionParallel` (8 goroutines, exactly one winner).
- `go test ./internal/runtime -run 'TestReport|TestINOFY|Test.*Workflow' -count=1` — PASS.
  `TestReportServiceAdmission` exercises `Service.StartReport`
  (create/rejoin/conflict/busy + control session reuse);
  `TestReportExecutorRejectsEscalation` proves sealed nodes fail closed
  with `ErrAuthorityDenied` when the report effect capability is unbound.
- `go test ./internal/app -run 'Report' -count=1` — PASS.
  `TestReportPurposePrecedesObserverDelivery` proves `purpose=report` Runs
  are excluded from memory/cognitive ingest predicates while cursors advance.
- `go test ./internal/modules/reports ./internal/moduleport -count=1` — PASS.
- `go test ./sdk/internal/assembly -run 'Report' -count=1` — PASS (new
  selected/omitted/requires-port/requires-factory/duplicate cases).

## Full package suites (both dialects)

- `VIVY_POSTGRES_TEST_DSN=… go test ./internal/storage/sqlite ./internal/storage/postgres -count=1` — PASS
  (sqlite 13.6s, postgres 22.7s; includes CN-19 runs listed by session,
  CN-33 goal admission, CN-40 child admission, tree queries, concurrent
  goal admissions — real Postgres, not skips).
- `go test ./internal/runtime -count=1` — PASS (64s).
- `go test ./internal/app -count=1` — PASS (11.9s).

## Plugin pressure matrix

- `TestCheckedInProviderConformanceMatchesExecutedSuites` — PASS (54s;
  internal source digest re-pinned to `d28e7280…` on the 5 internal rows
  after the `internal/` changes).
- `TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity)` — PASS.
- `Test(GenerationFailureMatrixExecutesEveryCase|MinimalArtifactPhysicallyOmitsOptionalModules)` — PASS.
- `Test(CompilePluginV1GraphFixtures|StartFailureRollsBackEveryConstructedOwner)` — PASS.
- `TestMiddleware(TimeoutFailsClosed|PanicAndInvalidDecisionFailClosed)` — PASS.

## SDK pack + Inspect (fresh disposable paths, `.workspace/r0-pack/`)

- `recipes/default.vivy.yml` — packed; manifest contains no `vivy/reports`
  (absent by default); `zz_default.go` accessors return `false`/`nil`.
- `recipes/no-notebook.vivy.yml` — packed; `vivy/reports` physically absent
  (0 references in manifest and generated sources).
- `recipes/reports-backend.vivy.yml` — packed; manifest contains
  `vivy/reports` module entry, its `core/report-service@v1` provides edge,
  and the requires edges on `core/action-host@v1` +
  `core/notebook-service@v1`; generated assembly emits
  `reports.NewModule().Construct(hosts.ForModule("vivy/reports"))` and the
  `ReportFactory: reports.Open` factory binding.

## `go generate ./internal/generated/assembly`

Regenerated via the checked-in tool; `zz_default.go` reports
`HasReportFactory() == false` / `ReportFactoryValue() == nil` for the
default recipe.

## `just ci`

End-to-end pipeline run (see acceptance). `ui/src/generated/*` regeneration
was reverted before commit per repo convention.

## Defects found and fixed during verification

1. `scan tree run: sql: expected 9 destination arguments in Scan, not 10` —
   `listRunsWhere` and `LatestPrimaryRunBySession` gained the `purpose`
   column in SELECT but the scans were updated in only one of the two
   places; fixed by selecting `purpose` and assigning `r.Purpose` in both
   readers on both dialects, then re-running the full storage + runtime
   suites green.
2. Postgres `mask: unavailable` on report admission — migration 038 was
   missing `DROP CONSTRAINT workflow_revisions_parent_run_id_fkey` (the FK
   rejected `parent_run_id=''`); added and re-verified on the real backend.
3. `TestReportPurposePrecedesObserverDelivery` returned no exclusion —
   `CreateRun` never wrote `purpose`; INSERT widened to 10 columns on both
   dialects.
4. `TestCoreCatalogDefinesCanonicalClosedPorts` — canonical closed-port
   list updated with `core/report-service@v1`.

## Honest-execution note

R0 advertises no generation output: sealed node types (collect, narrate,
validate-render, persist) are admitted and executed by `reportNodeExecutor`,
which fails closed with `ErrAuthorityDenied` "capability is not configured"
until R1 binds the report effect. No placeholder success is reported.
