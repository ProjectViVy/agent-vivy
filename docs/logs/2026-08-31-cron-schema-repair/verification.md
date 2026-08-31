# Verification

- `C:\PROGRA~1\Go\bin\go.exe test ./internal/storage/sqlite -run TestReopenRepairsCronTableAfterMigration016WasRecorded -count=1` — passed.
- `C:\PROGRA~1\Go\bin\go.exe test ./internal/storage/sqlite -count=1` — passed.
- `C:\PROGRA~1\Go\bin\go.exe test ./internal/storage/postgres -count=1` — passed; the optional live Postgres upgrade test is skipped when `VIVY_POSTGRES_TEST_DSN` is unset.
- `C:\Windows\System32\cmd.exe /c pnpm install --frozen-lockfile` from `ui/` — passed; generated local ignored dependencies.
- `C:\Windows\System32\cmd.exe /c pnpm build` from `ui/` — passed; generated the local ignored `ui/dist` required by Go embed.
- `C:\Windows\System32\cmd.exe /c just ci` — passed after the UI embed prerequisite was generated: Go fmt-check, vet, all Go tests, headless compile, UI typecheck, 173 Vitest tests, and Vite build.
- `git diff --check` — passed.

The first `just ci` attempt stopped before vet because this fresh worktree had no
ignored `ui/dist`; no source failure was reported. The prerequisite was built
and the complete command was rerun successfully.
