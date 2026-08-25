# Verification — 2026-08-26 AGENTS.md 并行与提交规则

## `just ci`（仓库根，2026-08-26）

Exit code 0，五段全绿：

| Slice | Result |
|---|---|
| `fmt-check` (gofmt over cmd/internal/sdk/ui *.go) | pass, no unformatted files |
| `go vet ./...` | pass |
| `go test ./...` | all packages ok (`internal/app` 2.7s fresh, rest cached) |
| `headless-compile` (`go test -run '^$' -tags vivy_headless`) | pass |
| `ui-ci` (pnpm install → typecheck → test → build) | pass: tsc clean; vitest 12 files / 57 tests passed; vite build OK (only a >500 kB chunk warning) |

注意：本次 ci 运行在**含其他 lane 未提交 UI 改动的整体脏树**上（evolution
页 / welcome wizard / chat message actions），该组合树全绿；本交付自身仅改
`AGENTS.md` / `docs/TODO.md` / 本日志，不影响任何被测路径。

## Smoke

纯治理文档改动，无用户可见或可执行行为变化，`smoke-for-user-visible-change`
不适用（理由如上，按规则记录）。

## Commit staging note

`docs/TODO.md` 的本次改动（`PROC-COMMIT` 条目）**故意不入本交付的 commit**：
该文件还带着其他 lane 的两条未提交条目（`UI-EVO`、`UI-CHAT-ACT`），整文件
stage 会把无关改动卷进来，违反本次新增的 `commit-one-concern-per-deliverable`。
该条目随 TODO 看板下次提交携带。
