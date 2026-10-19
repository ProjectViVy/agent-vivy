# CI closure verification

Status: all requested Linux gates completed successfully after the PostgreSQL fixture repair. Native Windows remains external, not inferred from Linux. No verification shell is pending.

## Lane and execution boundary

- Worktree: `/home/ubuntu/memory-write-model/agent-vivy/.worktrees/memory-ci-closure/agent-vivy`, branch `feat/memory-ci-closure`, baseline `e46d332362608062b44509810628e66bce97a712`.
- Linux amd64; Go `1.26.4`, Node `24.9.0`, Just `1.43.1`. Git configuration was process-isolated with `GIT_CONFIG_GLOBAL=/dev/null`; Go used process-local `GOTOOLCHAIN=go1.26.4` and `GOPROXY=https://proxy.golang.org,direct`.
- Fixture Git identity was process-local `大湿 <1136523089@qq.com>`. No delivery commit, push, PR, merge or deployment was performed.
- Only the integration lane and its private sibling Laputa were advanced. Original user workspaces and Laputa source files were not edited.
- PostgreSQL used the existing secret `VIVY_POSTGRES_TEST_DSN`, checked only for a loopback host and the disposable `vivy_test` database. No DSN or credential is recorded here.

## Patch integration and dependency boundary

The integration engineer inspected `face.patch`, `sdk.patch`, `toolkit.patch`, `bml.patch`, and `recap.patch` from `/home/ubuntu/memory-write-model/ci-closure-results/`, then applied them in that order using separate `git apply --check` and `git apply` operations. Changed paths were checked against each worker's authorized scope. Worker records `face.md`, `sdk.md`, `toolkit.md`, `bml.md`, and `recap.md` were read; their focused evidence does not substitute for the aggregate gates below.

After the pin patch, official `just ensure-laputa` advanced only `../laputa` in this lane. The private checkout is clean at `0d787277f521dee0d88d1beb522c34681e3ba57e`, with canonical origin `https://github.com/ProjectViVy/laputa.git`. The source lock, all three Go pseudo-versions and six checksum entries agree. Official bootstrap/source-closure checks validate module identities, cleanliness and the toolchain; no bootstrap/source/owner guard was relaxed.

## Newly exposed PostgreSQL fixture: red, fix, green

The first real `just test-postgres` and the first aggregate `just ci` failed `TestNotebookLegacyImport`. A focused run reproduced it before editing:

```sh
go test -count=1 ./internal/storage/postgres -run '^TestNotebookLegacyImport$' -v
```

The fixture appended `&search_path=...` to a DSN without an existing query component. PostgreSQL consequently interpreted the suffix as part of the database name (`vivy_test&search_path=...`). This was a test-fixture DSN construction defect, not evidence of a production storage failure.

`internal/storage/postgres/notebook_test.go:86-91` now reuses existing `openPool(dsn, schema)`, which parses the DSN and binds `search_path` on every pooled connection. Cleanup closes the pool even if a subsequent assertion fails. The fragile one-connection `SET search_path` is unnecessary. Migration, exact legacy text, oversized body, replay/import and schema-isolation assertions remain unchanged. No production file was changed for this repair.

The same focused command passed (`0.303s`), followed by `go vet ./internal/storage/postgres` and `git diff --check`, both exit 0. Official `just test-postgres` then passed (`27.578s`, explicit `JUST_TEST_POSTGRES_EXIT_CODE=0`). Only after that focused red/green repair were repaired CI and the requested race gate started.

## Required gates

| Gate | Observed result |
|---|---|
| `just doctor` | Exit 0: 26 OK, one nonfatal pnpm warning, zero failures. Installed launcher reports `10.34.7`; UI commands actually used pinned pnpm `10.33.0`. No global package/config change was made. |
| `just ensure-laputa` | Exit 0; private canonical, clean pinned source checked again after integration. |
| `bootstrap-test` through `just ci` | 20 Node tests passed, zero skipped, including real subprocess/error/signal behavior for BML and negative source-closure guards. |
| First `just ci` | Exit 1; root test phase stopped on the PostgreSQL fixture above. Later aggregate phases did not run in this attempt. Failure and explicit exit marker remain in `ci.log`. |
| Repaired `just ci` | Exit 0; explicit `JUST_CI_EXIT_CODE=0` observed after every recipe completed (shell `ce0007`). |
| `just race-ci` | Exit 0; explicit `JUST_RACE_CI_EXIT_CODE=0` observed after the full official internal-package gate (shell `b56bd4`). Runtime passed in `970.736s`, PostgreSQL in `36.873s`, SQLite in `302.555s`; no race report. |
| `just test-postgres` after repair | Exit 0 against the real disposable database; package was not skipped. |

Full stdout/stderr logs are outside the Git checkout:

- `/home/ubuntu/memory-write-model/ci-closure-results/ci.log` retains the failed first attempt and appends the repaired attempt.
- `/home/ubuntu/memory-write-model/ci-closure-results/race.log` records the requested aggregate race gate.

Both aggregate shell wrappers use `set -o pipefail`, capture `${PIPESTATUS[0]}` immediately after `just ... | tee ...`, print an explicit gate exit-code marker and exit with that code. A running shell is not a pass. The repaired CI and race commands overlapped after the fixture fix; no code edits occurred while they ran. Each completed its own official UI prerequisite and isolated temporary test fixtures; both full recipe exit codes were observed. This is not a claim of serialized aggregate execution.

The repaired CI completed UI typecheck/tests/build (101 files, 768 tests) and i18n checks, Go vet, root tests, the DIVA memory-loop overlay, all six generated-assembly recall checks, headless compilation, vet/tests for 23 independent Face/plugin modules, and BML vet/uncached tests. The repository's existing `TestMemoryLoop` exclusion is followed by its owning DIVA-overlay runner; it was not introduced or expanded by these repairs. Ordinary root package successes may be cached from the first attempt; the previously failing PostgreSQL package reran and passed.

`race-ci` is the official `./internal/...` race gate, not all Go modules and not the DIVA-overlay memory-loop phase. Additional focused SDK, real-memory-model and dependency race checks below extend that evidence without changing the gate's scope.

## Focused and real-path memory evidence

The integration engineer independently reran these checks; they are not claims copied from worker sweeps:

- SDK stdout/stderr and retained-cause tests under `-race`, selecting `TestSourceGoBindingOutput`, `TestGoModulePathOutput`, `TestMergeSourceModulePinsOutput`, `TestCanonicalResolvedModfileOutput`, `TestSnapshotDependencyLocksOutput`, and `TestParsedGoCommandFailuresRetainDiagnostics`: passed (`16.057s`). Cases include successful stderr noise, invalid/main/multiple package output and `errors.As(*exec.ExitError)` with diagnostics.
- `go test -race -count=10 ./internal/faceprocess`: passed (`1.709s`). The test joins the provider before restoring the global seam; no sleep-based coordination or production owner change was added.
- `python3 -m unittest discover -s scripts -p test_memory_model.py -v`: all 15 tests passed, covering strict transport/receipt rejection and both cancellation boundaries. Python ran with `PYTHONDONTWRITEBYTECODE=1`.
- `python3 scripts/memory_model.py --target all --seed 17 --cases 2 --steps 100 --race`: all four real-Go differential traces passed. BML seeds 17 and 18 each executed 204 operations; Recap seeds 17 and 18 each executed 213 operations (834 operations total).
- `python3 scripts/memory_model.py --replay scripts/testdata/memory-model/recap-cancel.json --race`: all seven operations passed.
- `python3 scripts/memory_model.py --target all --go-regressions --race`: native BML replay/tombstone-after-reopen and Recap cancellation regressions passed. Recap explicitly passed `recovery_before_replace` and `recovery_after_replace`.
- A separate 11-operation BML replay passed with `--race`: original create, update, original-operation replay, delete, restart, snapshot, replay after deletion, crash, snapshot, changed-payload replay and foreign-owner snapshot.
- The S5 worker's previously failing seed-17 Recap receipt was read from its isolated `.workspace/memory-model/recap-17.json` and replayed here on a fresh root with `--race`: 213 operations passed. The worker workspace was not edited.
- In the private pinned dependency, `go test -race -mod=readonly -p=1 -count=1 ./actmem` passed (`3.006s`). In private Garden, the same options passed `./agentapi`, `./internal/activity`, `./internal/recall`, and `./internal/runtimecore` (`14.846s`, `1.174s`, `1.103s`, `15.671s`). These are selected compatibility packages, not the entire Laputa/Garden product suite.

S5 already recorded a 40-trace, 8,340-operation race-instrumented real-Go sweep in `recap.md`. Integration used representative fresh traces/replays rather than repeating that costly sweep. This is bounded memory consistency evidence, not exhaustive exploration or all autonomy/evolution feature coverage.

## Development smoke

`just dev --no-browser` started the real split pair using integration-local `.workspace/dev/home`, backend `127.0.0.1:18788` and Vite `127.0.0.1:13016`. Backend `/healthz` returned HTTP 200 with `{"status":"ok","stage":"e2-recovery"}`. Via Vite, `/rpc/bootstrap` returned HTTP 200 and valid `/rpc` discovery with a nonempty token (not printed). An authenticated WebSocket upgrade through the Vite `/rpc` proxy succeeded.

An initial probe mistakenly requested `/api/config`, which is not this application's discovery endpoint; it returned Vite HTML and JSON parsing failed. Source inspection corrected the probe to `/rpc/bootstrap`; no product change was needed. The dev pair was then intentionally stopped with Ctrl+C (shell `eced76`, exit 130), with backend shutdown observed. No browser/UI manual acceptance or native Windows execution is claimed.

## Separate integration review pass

The integration engineer reviewed the final changed contracts independently of the S1-S5 patch authors:

1. Face changes are test-only: first App close is checked before composing a second owner; provider completion precedes seam restoration; shutdown status/error assertions remain strict.
2. The five SDK sites parse/hash only stdout, retain stderr on command failure and wrap subprocess causes with `%w`. Existing importability guards remain; no generic command framework or runtime registry was introduced.
3. Toolkit changes are test-only and still pass through the production platform shell. The Go helper validates real stdin and resolved environment binding. Deny, nonzero exit, rewrite, invalid JSON shape, missing executable and timeout remain fail-closed. The Hook deadline remains one second; the larger enclosing test budget lets the helper's deliberate two-second timeout stimulus distinguish a missing Hook deadline from a genuine timeout.
4. The BML Node entry runs vet before uncached bounded tests in the independent module, inherits both streams, preserves spawn-error causes and rejects signals/nonzero exits. Bootstrap tests cover those paths.
5. Dependency changes are limited to the published pin and matching records. Upstream cancellation handling still marks post-replacement uncertainty unknown and preserves I/O causes; the new pin permits only the documented safe pre-replacement cancellation retry.
6. The PostgreSQL follow-up reuses the owning helper, leaves all data assertions intact, configures every pooled connection and closes resources on failed assertions. It has focused red/green evidence; aggregate reruns include this final test file.

No new public Port, alternate Service/Journal/policy path, Eino import, LLM orchestration, production ownership bypass, source-bootstrap bypass or disabled assertion was introduced. An Eino capability extension is not applicable to these fixture/command-output/dependency repairs. This is a distinct integration-engineer review pass, not a claim that a new external reviewer or native Windows runner was used. Final diff/whitespace and private-source checks are recorded in the closure summary.

## Parent final review

After the integration handoff, the parent inspected all production/test/recipe diffs and the new SDK and BML test helpers. No further code defect was substantiated. Fresh checks on the unchanged code tree completed with exit 0:

- `go test -race -count=3 ./internal/faceprocess` (`1.259s`).
- `go test -race -count=1 -run 'Test(SourceGoBindingOutput|GoModulePathOutput|MergeSourceModulePinsOutput|CanonicalResolvedModfileOutput|SnapshotDependencyLocksOutput|ParsedGoCommandFailuresRetainDiagnostics)' ./sdk/internal` (`16.099s`).
- `go test -count=1 -run '^TestSelectedFaceUsesOneApp$' ./internal/app` (`0.316s`).
- `node --test scripts/bml-ci.test.mjs scripts/ensure-laputa.test.mjs scripts/dependency-closure.test.mjs`: 20 passed, zero failed or skipped.
- `go vet ./sdk/internal ./internal/faceprocess ./internal/storage/postgres` and `git diff --check`: no diagnostics.

The private Laputa checkout was independently confirmed clean at the exact pinned SHA. Raw aggregate logs retain both the initial CI exit 1 and repaired exit 0, with final race exit 0. No code changed after those aggregate runs; only delivery records were clarified. Worker no-commit instructions applied to their handoffs, not to the parent's subsequent local human-attributed delivery commit. Planning artifacts are not staged. No push or PR is authorized by this local preparation.
