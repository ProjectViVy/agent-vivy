# Verification

- RED: `go test ./internal/runtime -run '^TestGoalAdmissionEventExposesCurrentRunToSubscriber$' -count=1` failed: the published admission contained a RunID while the immediate subscriber projection returned an empty current RunID.
- RED: `pnpm exec vitest run src/components/chat/WorkControlBar.test.tsx` failed both new edit behavior cases because the edit form was absent.
- RED: `pnpm exec vitest run src/lib/store.test.ts -t 'submits the Goal reference captured'` failed because `goal/edit` sent revision 3 from refreshed work rather than captured revision 2.
- GREEN: `pnpm exec vitest run src/components/chat/WorkControlBar.test.tsx src/lib/store.test.ts src/lib/ui-build-provenance.test.ts src/lib/ui-sdk-face-compat.test.ts` passed, 4 files / 57 tests.
- GREEN: `pnpm typecheck` passed after the Face store signature was updated. Its first run exposed that signature mismatch in two compile-time Face compatibility tests.
- GREEN: `go test ./internal/runtime ./internal/rpc -run 'TestGoal|TestWorkControl|TestWorkSubscription' -count=1 -timeout=5m` passed both packages.
- GREEN: `go test -race ./internal/runtime ./internal/rpc -run 'TestGoalAdmissionEventExposesCurrentRunToSubscriber|TestGoal|TestWorkControl|TestWorkSubscription' -count=1 -timeout=5m` passed both packages.
- `git diff --check` passed. Go files were formatted with `gofmt`.

On this host `go` is not on the default shell PATH; Go commands used `C:\Program Files\Go\bin\go.exe`, and `pnpm typecheck` temporarily prepended that bin directory to PATH so its SDK staging step could run. The worktree's UI dependencies were installed with `pnpm install --frozen-lockfile` (no lockfile change).

No full `just ci`, Docker/PostgreSQL check, browser smoke, or dev server was run, per the fix brief. The parent lane owns the final integrated gate. The existing Eino v0.9.13 / EinoExt capability check in the PG-3 ledger remains applicable: this change adds no Eino API or orchestration. UI staging touched the working copy of `ui/src/generated/assembly.ts` but the tracked content has no diff and is excluded from the commit.
