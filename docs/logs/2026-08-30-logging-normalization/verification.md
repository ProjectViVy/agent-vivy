# Verification — logging normalization

Environment: Windows, worktree `../agent-vivy-logging` (branch
`feat/logging-normalization`), Go 1.26.4.

## 1. Gate: `just ci` — PASS

Fresh worktree note: `just ci` order puts `go vet ./...` before
`ui-ci`, and `ui/embed.go` needs `ui/dist`; in a fresh checkout
`ui/dist` must exist first. Sequence actually run:

1. `just ... ui-ci` — pnpm install --frozen-lockfile, typecheck, test
   (21 files / 175 tests passed), build → `ui/dist` created. PASS.
2. `just -f justfile -d <worktree> ci` — full gate:
   - `fmt-check` (gofmt over cmd/internal/sdk/ui) — clean
   - `go vet ./...` — clean
   - `go test ./...` — all packages pass (incl. new
     `internal/logging`, extended `internal/config`)
   - `headless-compile` — pass
   - `ui-ci` — pass (175 tests, vite build)
   Exit code 0.

Targeted earlier runs: `go test ./internal/logging ./internal/config
./internal/runtime` — ok (runtime 29.9s).

## 2. Unit coverage added

- `internal/logging`: level/format parse tables, env override
  (`VIVY_LOG_LEVEL`/`VIVY_LOG_FORMAT`), invalid env/config values
  rejected, empty dir rejected, JSON file sink content
  (msg/run/level/source), text format content, daily rollover across
  two local days (injected clock), retention sweep (old mtime removed,
  fresh + non-matching names kept, `0` keeps all).
- `internal/config`: `logging:` defaults (info/json/30/true), explicit
  section load including `retention_days: 0` overriding the default,
  `LogDirectory()` derivation, invalid level/format/retention rejected
  under the strict decoder.

## 3. Real-path smoke — PASS

Built `vivy.exe` (`go build -o data/smoke-logs/vivy.exe ./cmd/vivy`),
ran with `VIVY_CONFIG`/`VIVY_USER_HOME` pointing at a scratch dir,
`server.addr: 127.0.0.1:18787`, `runtime.mock: true`.

| Scenario | Result |
|---|---|
| Default start | stdout and `data/logs/vivy.log.2026-08-30` both carry JSON lines; milestone `logging initialized {"level":"info","format":"json",...,"stdout":true}` then `vivy starting` (app.go:625). Source file:line present. |
| `VIVY_LOG_LEVEL=debug` | milestone reports `"level":"debug"`; process starts normally. |
| `VIVY_LOG_FORMAT=text` | file/console lines switch to `time=... level=INFO source=... msg="logging initialized" ... format=text`. |
| Retention sweep | Seeded `vivy.log.2020-01-01` with 2020 mtime → deleted on next start; today's file kept. |
| `VIVY_LOG_LEVEL=loud` | startup aborts via the bootstrap logger: `startup aborted err="logging: invalid level \"loud\" (want debug|info|warn|error)"`. |
| Error path lands in file | an overlapped smoke start failed composition ("organism lease held" — expected scratch contention between two smoke processes); the ERROR line with source was written to the file sink. |

Smoke artifacts (exe, config, logs) live in the worktree's ignored
`data/smoke-logs/` scratch; nothing outside the worktree was touched
(no writes to `data/vivy.db`, `data/demo/`, `data/workspaces/` of the
root checkout).

## 4. Not run

- Browser UI smoke — no UI-facing behavior changed; the server-side
  real-path runs above are the observable surface for this iteration.
- `vivy worker` file logging — explicitly out of scope (TODO LOG-1).
