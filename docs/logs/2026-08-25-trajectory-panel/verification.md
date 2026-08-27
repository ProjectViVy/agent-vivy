# Verification

## Commands (run in `../agent-vivy-trajectory` worktree, branch `feat/trajectory-panel`)

| Check | Result |
|---|---|
| `pnpm install --frozen-lockfile` (ui) | pass (19s, shared pnpm store; esbuild postinstall ignored but build works) |
| `pnpm typecheck` (ui) | pass (0 errors) |
| `pnpm test` (ui) | pass — 16 files / 122 tests, incl. new `trajectory-utils.test.ts` (17 tests) |
| `pnpm build` (ui) | pass (2207 modules, dist emitted) |
| `just ci` (worktree root) | **pass, exit 0** — fmt-check · go vet ./... · go test ./... · headless-compile · ui-ci |

> Fresh-checkout note: a clean tree has no `ui/dist`, so the first `just ci`
> fails at `go vet ./...` on `ui/embed.go`'s `go:embed all:dist` before the
> first `pnpm build` runs. Normal dev trees carry a prior build's `ui/dist`.
> This run built the UI first, then `just ci` passed end to end. Tracked as
> `docs/TODO.md` §0.1 `UI-CI-BOOTSTRAP`.

## Static checks

- `git status` in worktree: only this deliverable's paths changed.
- No audit leftovers in `ui/src`:
  `grep -ri "audit|审计" ui/src` → 3 benign hits only: 进化页「可审计的进化
  治理」subtitle (zh/en) and `diva-preview-data.test.ts`'s
  `not.toContain('audit')` assertion.
- `ui/src/components/audit/` no longer exists; `DIVA_AUDIT_EVENTS` removed.
- i18n zh/en parity enforced by `src/i18n/index.test.ts` (passes) after
  removing `audit`/`settings.preview.audit` and adding `trajectory`.
- `TrajectoryPanel` imports no `src/lib/demo-api.ts` and no RPC/api layers:
  data comes from `trajectory-demo-data.ts` static constants only.

## User-visible smoke (browser, `http://127.0.0.1:3016/dashboard`)

Headless Chromium smoke against the worktree's Vite dev server (port 3016,
same `ui/` sources; the always-on :3015 split pair serves the root tree).
Storage pre-seeded to dismiss the first-run welcome wizard and demo banner. All
checks passed, `pageerror` count 0:

1. `/dashboard` renders `中控台`, tabs = **概览 / Token / 轨迹**, no 审计 tab.
2. 轨迹 tab: toolbar present (轨迹工具栏), one timeline (24 spans, 3 lanes),
   ledger 24 rows, 8 `Request #N` chips, spy spans match record count.
3. Click row `rec-4` opens the detail panel (tabs 输入/输出/思考); close works.
4. Click `Request 1` chip opens request details — header `请求 1 · #1 · Step 1`,
   Summary shows 状态=完成 / Provider=deepseek / Model=deepseek-chat /
   工具调用=1 / Result text; 摘要/用量/时序 tabs present.
5. 「实际时长」toggle re-projects spans: first span width 34.4px → 51.4px.
6. 折叠全部回合 → 摘要行 `…#1 · 已折叠 · 7 条记录` / `#2 · 7` / `#3 · 8`,
   record rows drop 24 → 2; expand restores 24.
7. 错误行存在（`data-error`, `bash · pnpm build` → `exit 1 · 产物检查失败`）.
8. 搜索「build」→ 2 行且全部命中；清空恢复.
9. 时间轴拖选区间 → 区间外账本行淡化（opacity 0.35, 12 行）；轨道聚焦后
   Escape 清除选区（0 行淡化）。
10. 回合标签 `#1/#2/#3` 与会话起始 `Session` 标签渲染正常。
11. 三泳道标签 Input/Model/Tools 按 7/21/35px 渲染（DOM 断点确认）；
    zh/en 词典结构对等由 `src/i18n/index.test.ts` 断言兜底（本次未做浏览器内
    语言切换冒烟）。