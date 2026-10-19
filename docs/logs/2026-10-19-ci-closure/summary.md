# CI closure integration summary

## Outcome

**Linux integration and all requested local gates passed. Native Windows remains external.** S1-S5 reviewed repairs are integrated into `feat/memory-ci-closure` in the isolated `memory-ci-closure/agent-vivy` worktree, based on `e46d332362608062b44509810628e66bce97a712`. The parent prepares the reviewed deliverable as a local human-attributed commit. No push, PR, merge or deployment was performed; publication requires the owner's authorization.

The first aggregate CI attempt exposed an additional PostgreSQL test-fixture DSN defect. It was reproduced with a focused test, repaired without production changes, and verified green before the successful aggregate runs. The initial failure is retained in the CI log, not overwritten or relabeled as a pass. There are no outstanding failing tests or pending verification shells in this handoff.

## Integrated changes

- **S1:** `internal/app/facehost_selected_test.go` explicitly closes the first App before composing another owner. `internal/faceprocess/selected_test.go` joins provider completion before restoring the global seam and retains shutdown-deadline assertions. Production owner/recovery guards were not changed.
- **S2:** `sdk/internal/frontend_v1.go` separates stdout data from stderr diagnostics at all five affected Go parsing/hashing sites. `sdk/internal/go_output_test.go` covers stderr noise, invalid binding output and retained wrapped command failures.
- **S3:** `internal/runtime/bundle_hooks_matrix_test.go` uses the real Go test binary as a portable Hook fixture through the existing platform shell, retaining stdin, resolved binding, denial, rewrite, exit and timeout evidence. No Windows skip or production shell fallback was added.
- **S4:** `scripts/bml-ci.mjs` replaces the PowerShell-specific `bml-ci` body; its tests are included in `bootstrap-test`. Vet precedes bounded uncached BML tests, with inherited streams and nonzero/signal/spawn failure propagation.
- **S5:** `laputa-source.lock.json`, three Go pseudo-versions and six checksum entries adopt published Laputa `0d787277f521dee0d88d1beb522c34681e3ba57e`. Official `just ensure-laputa` advanced only this lane's clean canonical private sibling. Laputa source was not edited.
- **Integration follow-up:** `internal/storage/postgres/notebook_test.go:86-91` reuses `openPool(dsn, schema)` instead of appending `&search_path` to an arbitrary DSN, and closes the pool on early failures. Exact legacy-import/migration/data assertions remain intact.

The worker evidence files `face.md`, `sdk.md`, `toolkit.md`, `bml.md`, and `recap.md` were inspected and retained. The execution index now reflects observed integration and platform boundaries.

## Verification result

| Gate | Final result |
|---|---|
| `just doctor` | Passed: 26 OK, one nonfatal pnpm-launcher warning, zero failures |
| `just ensure-laputa` | Passed; canonical clean private source at the exact pin |
| `just ci` | Passed, explicit `JUST_CI_EXIT_CODE=0` after the complete recipe |
| `just race-ci` | Passed, explicit `JUST_RACE_CI_EXIT_CODE=0` after the complete official internal-package race gate |
| `just test-postgres` | Passed, explicit exit 0 against the disposable `vivy_test` database; no skip |
| Isolated split development smoke | Backend health, Vite RPC discovery and authenticated WebSocket proxy upgrade passed; both dev processes stopped |

The successful CI includes 20 bootstrap Node tests, UI typecheck and 768 tests in 101 files, UI build/i18n, Go vet/root tests, the DIVA memory-loop overlay, six generated-assembly recall checks, headless compilation, 23 independent Face/plugin modules and BML vet/tests. The official race gate covers `./internal/...`, not every module or the DIVA memory-loop overlay.

Additional integration-owned checks passed: focused SDK race tests, ten Face race repetitions, 15 Python model unit tests, four real-Go race-instrumented BML/Recap traces (834 operations), curated cancellation replay, an 11-operation BML lifecycle/replay trace, the formerly failing 213-operation Recap receipt, native cancellation/replay regressions, ACTMEM race tests and four selected Garden compatibility packages. The worker's larger 40-trace/8,340-operation sweep was reviewed rather than unnecessarily repeated.

Raw aggregate logs:

- `/home/ubuntu/memory-write-model/ci-closure-results/ci.log` (initial exit 1 and repaired exit 0 retained).
- `/home/ubuntu/memory-write-model/ci-closure-results/race.log` (final exit 0).

Detailed commands, red/green evidence, gate scopes and the separate integration review pass are in [verification.md](verification.md). Final tracked/new-file whitespace checks, the supplemental fixture patch reverse-check and private Laputa cleanliness check passed. Integration workers left HEAD at the baseline; the parent independently reviewed the final diff and reran focused checks before preparing the local commit. Smoke ports `18788` and `13016` were confirmed closed after shutdown. Planning artifacts stay local and are excluded from delivery commits.

The additional fixture-only patch is `/home/ubuntu/memory-write-model/ci-closure-results/postgres-fixture.patch`; it supplements the five supplied worker patches and is already applied in this integration worktree. Closure records and index changes are also present directly in the worktree.

## Acceptance boundary

[acceptance.md](acceptance.md) records Linux acceptance separately from unexecuted native Windows work. No native Windows, manual browser, full upstream product-suite, complete all-module race, exhaustive state-space, production-data, external-provider or all-autonomy/evolution coverage is claimed. No secrets, daily Vivy home, original workspaces, Git configuration, remotes or production journals were changed.
