# dsh-vivy-console

First-party **Vivy Studio console** bundle: the Vivy development loop from
inside the Studio UI. Supersedes `dsh-vivy-debugger` (the floating
bottom-right ◈ button is retired; the console lives in the conversation view
ring beside Trajectory / Context as a **「Vivy 控制台」** tab).

## One dev model only

The console manages exactly the split pair from AGENTS.md — there is **no
embedded-UI / standalone-UI split** and **no separate VIVY WEB debugging**:

| Process | What it is | Notes |
| --- | --- | --- |
| 后端 Backend | **Pure-API** vivy binary (`go build -tags vivy_headless ./cmd/vivy`), auto-compiled from workspace source into Studio scratch; serves only the `/rpc` control plane — **no embedded frontend** | real OpenAI-compatible provider path (no product mock); data isolated under `data/studio-home/vivy-console/` via `VIVY_CONFIG` + `VIVY_USER_HOME`; status / start / stop / restart / EXE override |
| 前端 Frontend | **The DEV dev server**: `pnpm dev` in `ui/`, `http://127.0.0.1:3015` — the single user-facing app | its `/rpc` proxy is pointed at the managed backend's live port (`VIVY_BACKEND_ADDR`); status / start / stop / restart / open |
| 日志 Logs | **One unified timeline** of backend (`gateway.out.log`/`gateway.err.log`) and frontend (`frontend.out.log`/`frontend.err.log`) lines, tagged with source (后端/前端) | filter by source, pause, clear |

The console does **not** open or debug a "VIVY WEB page" of its own: you use
the dev server at `http://127.0.0.1:3015` in the normal browser. The
retired `/vivy-web/` same-origin proxy, `hook.js` frontend-debug bridge, and
Studio-origin `/vivy-config.json` route are deleted — there is no embedded UI
to proxy (the `vivy_headless` backend carries none), and nothing else needs
the bridge.

Scope (2026-08-27): the console is two clearly separated concerns —
see the 「打包与版本」 page below for the distribution half. Distribution
(`pack` / `eval` / `release` / `install` / `rollback`) is a **separate page**,
never mixed into the dev loop; release keeps the human gate (NG-25).

**Vivy Code (2026-08-29)** is a **third** operator surface on the 总控台 card
grid: it opens a dedicated OS console for `vivy tui --demo` (Crush-style TUI
skeleton) or `--plain`. It is **not** part of 一键启动/停止/重启, does not
share the backend/frontend process tree, and never writes gateway or Vite
logs. Source root: `VIVY_CODE_ROOT` → this workspace → sibling
`../agent-vivy-tui-crush` (when that worktree has `internal/tui`).

## Host half (`index.js`)

Real Node plugin (no vm sandbox) that registers on the Studio `webServer`:

| Route | Purpose |
| --- | --- |
| `/vivy-console/api/status` `logs` `resolve` | Status (backend + frontend) / unified log timeline / path resolution |
| `/vivy-console/api/start` `stop` `restart` `setExe` | Backend lifecycle. **Start on the managed path first runs `go build -tags vivy_headless -o <scratch>/vivy-backend.exe ./cmd/vivy`** (incremental, output streamed into the backend log), then spawns it with the generated real-provider overlay (`VIVY_CONFIG` + `VIVY_USER_HOME` under Studio scratch) — every start reflects current workspace source; `runtime.mock` is not written (removed from product Config 2026-08-29) |
| `/vivy-console/api/frontend/status` `start` `stop` `restart` | Vite dev server lifecycle; the child gets `VIVY_BACKEND_ADDR=http://127.0.0.1:<gateway-port>` so `ui/vite.config.ts` proxies `/rpc` to the live backend port |
| `/vivy-console/api/code/status` `open` | **Vivy Code** TUI panel: status + open a dedicated console window (`go run ./cmd/vivy tui --demo` / `--plain`). Not part of backend/frontend lifecycle |
| `/vivy-console/api/lifecycle/list?kind=` `run` `jobs/<id>` | **Packaging & version management** (separate from the dev loop): reads the ledger (`generations/evals/releases/installs/events/worktrees`) and runs `pack/eval/release/reject/install/rollback/inspect` through `vivy-studio.exe` on the pinned worktree, one concurrent job with streamed output. Release forwards `--actor human --yes` only after the UI confirmation (NG-25); this surface never starts/stops the dev processes |

Backend binary resolution order: explicit EXE override (panel `setExe`) →
`VIVY_HEADLESS_EXE` env → auto-compiled `<scratch>/vivy-backend.exe`. The
first two skip the auto-compile. An override is expected to be a headless
(pure-API) build — the console no longer runs the embedded-UI binary.

Air gap: backend data lives only under `data/studio-home/vivy-console/`
(Studio's own scratch); the production Journal (`data/vivy.db`,
`data/demo/`, `data/workspaces/`) is never touched.

## Client half (`client.js`)

Hand-authored client module in the DSH client-modules handoff format
(`window.__ModuleLoader__.load({id, factory})`, React via the injected
`require`) — no build step. It registers `conversation.view` id
`vivy-console` order 30 label 「Vivy 控制台」 with three sections, in order:
**总控台 / 日志 / 打包与版本**:

- **总控台** — one consolidated screen: an overall state line (全部运行中 /
  后端运行中 · 前端未运行 / …), and **▶ 一键启动 / ■ 一键停止 / ⟳ 一键重启**
  (orchestrates backend + frontend sequentially), and two side-by-side
  status cards:
  - **后端状态**: 状态 / PID / 监听 / 启动于 / EXE / 形态（纯 API …
    vivy_headless）/ 编译方式, plus individual 启动/停止/重启 and the EXE
    override input (留空自动编译).
  - **前端状态**: 状态 / PID / 入口（127.0.0.1:3015 Vite DEV 开发服务器，
    唯一前端）/ 启动于 / 命令 / 代理（/rpc → 后端）, plus individual
    启动/停止/重启 and a「打开」button that opens
    `http://127.0.0.1:3015` in a new tab (the dev server is the app).
  - **Vivy Code（TUI 开发面板）**: independent third card — opens
    `vivy tui --demo` (or `--plain`) in a dedicated OS console; **not**
    driven by 一键启动/停止/重启. Source via `VIVY_CODE_ROOT` or sibling
    TUI worktree.
- **打包与版本** — packaging & version management, **deliberately separate
  from the dev loop** (it never starts/stops the backend/frontend): page
  header states the separation; ledger chips (generations / evals /
  releases / installs / events / worktrees) with `vivy-studio.exe` reads, a
  JSON table per ledger, the action form (pack/eval/release/reject/install/
  rollback/inspect inputs), the release human-confirmation checkbox, and a
  live job-output viewer (one concurrent job).
- **日志** — one unified timeline of 后端 + 前端 lines with source chips,
  pause, clear.

## Install

Composed into the `vivy-studio` profile by `launch-vivy-studio.ps1`
(dependency + `dsh.profile.bundles` entry), so a normal
`.\launch-vivy-studio.ps1` boots with the console enabled. To apply an
edited source to a running profile: sync the installed copy at
`data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/` and
restart Studio (skin-style injection — no hot reload). The client bundle is
served fresh per request (tab changes appear after a browser refresh); the
host-half routes need the restart.

## Requirements

- **Go toolchain** reachable from the Studio process (it compiles the
  headless backend on start; `launch-vivy-studio.ps1` already puts Go on
  `PATH`). Alternatively pin a prebuilt headless binary via the panel EXE
  override or `VIVY_HEADLESS_EXE`.
- `ui/` with `package.json` and installed `node_modules` for the frontend
  dev server (`pnpm dev`).
- `data/studio-home/` is Studio scratch (ST-2).