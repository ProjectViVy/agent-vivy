# Verification

- RED: `go test ./internal/runtime -run '^TestGoalAdmissionEventExposesCurrentRunToSubscriber$' -count=1` failed: the published admission contained a RunID while the immediate subscriber projection returned an empty current RunID.
- RED: `pnpm exec vitest run src/components/chat/WorkControlBar.test.tsx` failed both new edit behavior cases because the edit form was absent.
- RED: `pnpm exec vitest run src/lib/store.test.ts -t 'submits the Goal reference captured'` failed because `goal/edit` sent revision 3 from refreshed work rather than captured revision 2.
- GREEN: `pnpm exec vitest run src/components/chat/WorkControlBar.test.tsx src/lib/store.test.ts src/lib/ui-build-provenance.test.ts src/lib/ui-sdk-face-compat.test.ts` passed, 4 files / 57 tests.
- GREEN: `pnpm typecheck` passed after the Face store signature was updated. Its first run exposed that signature mismatch in two compile-time Face compatibility tests.
- GREEN: `go test ./internal/runtime ./internal/rpc -run 'TestGoal|TestWorkControl|TestWorkSubscription' -count=1 -timeout=5m` passed both packages.
- GREEN: `go test -race ./internal/runtime ./internal/rpc -run 'TestGoalAdmissionEventExposesCurrentRunToSubscriber|TestGoal|TestWorkControl|TestWorkSubscription' -count=1 -timeout=5m` passed both packages.
- `git diff --check` passed. Go files were formatted with `gofmt`.

## Integrated repository gate

- Tested source tree: `1eb5cf8` plus the i18n-contract and conformance-metadata changes committed as `33d1faa`.
- Final `just ci` exited 0. The command-local PATH included `C:\Program Files\Go\bin` because Go is not on the default shell PATH.
- The gate passed UI typecheck, all 51 UI test files / 424 tests, UI production build, i18n completeness and cross-face contract checks (8 script tests), `go vet ./...`, `go test -timeout 20m ./...`, headless compile, and plugin/face vet and tests.
- Slow packages in the final run: `internal/app` 59.2s, `sdk/internal` 848.9s, and `sdk/internal/conformance` 187.9s. The UI build emitted the existing >500 kB chunk-size advisory; build succeeded.
- The first integrated attempt stopped at `check-i18n-cross-face.js`: five new Goal-edit Web keys lacked `faceSpecific.webKeys` entries. Added the five classifications; the focused cross-face script and all 8 classifier tests passed.
- The next integrated attempt stopped at `TestCheckedInProviderConformanceMatchesExecutedSuites`: changes under `internal/` made the checked-in digest stale. `go run ./sdk/internal/cmd/source-hash internal ""` computed `ba293c1a2698960a4e32f27464fd012d402b8394a0c558df780af5d913b75a73`; that value now appears in all five internal-rooted rows. The final full gate passed the producer test and conformance suites.

On this host `go` is not on the default shell PATH; Go commands used `C:\Program Files\Go\bin\go.exe`, and `pnpm typecheck` temporarily prepended that bin directory to PATH so its SDK staging step could run. The worktree's UI dependencies were installed with `pnpm install --frozen-lockfile` (no lockfile change).

## Not run / remaining gates

- `just test-postgres` was not run because `VIVY_POSTGRES_TEST_DSN` is unset and `psql` / a local PostgreSQL service were not found. Docker-required validations remain deferred at the user's request. Live PostgreSQL evidence is still required by PG-0/PG-1, so that gate remains open.
- `just dev` and browser acceptance were not run: ports 8787 and 3015 were already occupied by processes 8596 (`vivy.exe`) and 4620 (`node.exe`) from the root checkout. They were left running. PG-5/PG-6 browser evidence remains open.
- The PG-6 real coding walkthrough was not run; no evidence is claimed for live-model effectiveness, actual changed files, or command-exit records.
- No push, PR-state change, merge, or issue closure was made. The PR remains Draft.

The existing Eino v0.9.13 / EinoExt capability check in the PG-3 ledger remains applicable: this change adds no Eino API or orchestration. UI staging touched the working copy of `ui/src/generated/assembly.ts` and `ui/src/routeTree.gen.ts` due to line endings; both have no content diff and were excluded from the commits.
