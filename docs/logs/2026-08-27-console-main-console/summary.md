# Vivy Console — 总控台 (main console) interaction

Date: 2026-08-27
Scope: Vivy Studio overlay (`studio/dsh-vivy-console` client), not the Vivy
kernel. No host-half change.

## What changed

Per user request (the unified dev model "works now, but the interaction feels
fragmented"), the console's three sections 后端/前端/日志 are reorganized into
a **总控台** so backend and frontend are not split across separate tabs:

1. **总控台 section** (was 后端 + 前端 separate tabs):
   - one overall state line: 全部运行中 / 后端运行中 · 前端未运行 /
     前端运行中 · 后端未运行 / 未运行;
   - **▶ 一键启动 / ■ 一键停止 / ⟳ 一键重启** — the client orchestrates the
     existing host routes sequentially (start: backend → frontend; stop:
     frontend → backend; restart: stop both → start both), reporting a
     combined result message; "已在运行" from one side is treated as a
     no-op and the other side still runs;
   - two side-by-side status cards (backend + frontend): 状态 / PID /
     监听 / 启动于 / EXE / 形态（纯 API · vivy_headless）/ 编译方式, and
     入口 / 命令 / 代理（/rpc → 后端）respectively;
   - each card keeps its own individual 启动/停止/重启 (and the backend EXE
     override input) so fine control is not lost.
2. **日志 section** unchanged (unified 后端+前端 timeline, source chips,
   pause, clear).
3. No host API change — the one-click actions reuse
   `/vivy-console/api/start|stop|restart` and `/frontend/*`.

### Files

- `studio/dsh-vivy-console/client.js` — two sections 总控台/日志; OverviewPane
  (one-click orchestration + backend/frontend status cards) replaces the
  separate BackendPane/FrontendPane tabs; small `vc-card-title` style added.
- `studio/dsh-vivy-console/README.md` — client-half section rewritten for the
  总控台 layout.
- `studio/dsh-vivy-console/package.json` — description updated.
- Installed profile copy synced:
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`.

## What was explicitly not done

- No host-half (`index.js`) change, no kernel / `internal/` / `cmd/` / `ui/`
  change — so no Studio restart is required: the client bundle is served
  fresh per request and appears after a browser refresh.
- No change to the unified model (pure-API backend + Vite dev server only);
  the 内嵌/独立 dual-mode debugging stays removed.
- No new host routes (one-click is client-side orchestration of the existing
  API).

## Notes

- One-click 启动 starts the backend first (which incrementally compiles
  `vivy_headless` when not overridden) and only then the frontend; a backend
  start failure aborts the sequence with the backend message.
- Status polling and the rest of the console behaviour are unchanged from the
  previous iteration.