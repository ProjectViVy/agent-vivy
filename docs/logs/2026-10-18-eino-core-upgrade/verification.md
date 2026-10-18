# Verification — Eino core upgrade

Linux/amd64 cloud sandbox, Go 1.26.4, running as root; sibling
`../laputa` checked out at the pinned `4b2bec2cc2ab`. `just` is not installed
and every `just ci` recipe is PowerShell, so each step was run as its Linux
equivalent.

## Bug reproduction (before the fix)

The four new tests were run against the original `service.go`:
all failed — run.failed carried the generic restart message, and with a
missing checkpoint the approval stayed `pending`. A probe before the fix also
showed the approval and question left `pending` after an engine-version
restart. With the fix all four pass.

## `just ci` steps

| Step | Command run | Result |
|---|---|---|
| ensure-laputa | `git clone ProjectViVy/laputa ../laputa && git checkout 4b2bec2cc2ab` | ok |
| bootstrap-test | `node --test scripts/ensure-laputa.test.mjs scripts/dependency-closure.test.mjs` | 2 pass / 9 fail — **identical on untouched `main`** (worktree at `e7dcf85`); the scripts assume Windows paths/PowerShell. Not caused by this change |
| fmt-check | `gofmt -l` over `git ls-files --cached --others --exclude-standard -- '*.go'` | clean |
| ui-ci | `pnpm install --frozen-lockfile` (official npm registry; the lockfile's `registry.npmmirror.com` is blocked by the sandbox proxy, integrity unchanged), `pnpm typecheck`, `pnpm test`, `pnpm build`, the three i18n checks | all pass; 93 files / 700 tests |
| vet | `go vet ./...` | clean |
| test | `go test -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...` | all packages ok except `TestDeliverableReadSnapshotWriteFailure` (expects an unwritable directory; root defeats it — fails identically without the bump, see `docs/logs/2026-10-17-provider-agentic-spike/`). `sdk/...` was first run before the UI toolchain was installed (failures were missing `ui/node_modules`); re-run after `ui-ci`: all ok |
| test (memory loop) | Linux port of `scripts/test-memory-loop.ps1` (same commands: `-run ^TestMemoryLoop` on all packages but `internal/app`, `sdk pack` + `inspect-artifact` of `recipes/diva.vivy.yml`, `sdk/internal` memory-loop, `internal/app` with the overlay) | ok |
| diva-recall-test | Linux port of `scripts/test-diva-recall.ps1` (generate DIVA assembly, overlay, `-tags vivy_diva_integration`, six selected tests) | ok, 0 skips, none missing |
| headless-compile | `go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui` | ok |
| plugin-ci | `go vet ./... && go test ./...` in each of the 22 `plugins/**` and `faces/**` modules | all ok |

## Targeted checks

| Check | Result |
|---|---|
| `go test -race -timeout 20m ./internal/runtime -run 'TestNudge\|TestToolFailure' -count=1` on untouched `main` (v0.9.13) | FAIL, 1 `DATA RACE` at `eino@v0.9.13/compose/tool_node.go:1253` |
| same command on this branch (v0.9.21) | ok, 0 races |
| `GOFLAGS=-mod=readonly go build ./internal/... ./cmd/...` | ok (`go.sum` complete) |
| `go version -m` on built `cmd/vivy` | `github.com/cloudwego/eino v0.9.21` |
| Two-binary smoke: old binary (v0.9.13, from `e7dcf85`) then new binary on the same scratch SQLite database, `VIVY_CONFIG` + `VIVY_ADDR=127.0.0.1:18787` | both boot, UI 200, `/rpc` 401 (auth required, expected); log shows no WARN/ERROR besides the address override |

Downgrade direction: the checkpoint version check is string equality
(`internal/runtime/checkpoint.go:107`), so a rollback reaches the same
recovery branch as the tested upgrade. No separate rollback test was added;
the tests use two arbitrary version strings, so they cover either direction.

## Not run

- `just ci` itself (no `just`, recipes need PowerShell).
- Split-Vite browser smoke on `:3015`: no UI change.
- Live-provider paused-run upgrade (no credentials).
