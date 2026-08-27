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
| 后端 Backend | **Pure-API** vivy binary (`go build -tags vivy_headless ./cmd/vivy`), auto-compiled from workspace source into Studio scratch; serves only the `/rpc` control plane — **no embedded frontend** | mock mode, data isolated under `data/studio-home/vivy-console/`; status / start / stop / restart / EXE override |
| 前端 Frontend | **The DEV dev server**: `pnpm dev` in `ui/`, `http://127.0.0.1:3015` — the single user-facing app | its `/rpc` proxy is pointed at the managed backend's live port (`VIVY_BACKEND_ADDR`); status / start / stop / restart / open |
| 日志 Logs | **One unified timeline** of backend (`gateway.out.log`/`gateway.err.log`) and frontend (`frontend.out.log`/`frontend.err.log`) lines, tagged with source (后端/前端) | filter by source, pause, clear |

The console does **not** open or debug a "VIVY WEB page" of its own: you use
the dev server at `http://127.0.0.1:3015` in the normal browser. The
retired `/vivy-web/` same-origin proxy, `hook.js` frontend-debug bridge, and
Studio-origin `/vivy-config.json` route are deleted — there is no embedded UI
to proxy (the `vivy_headless` backend carries none), and nothing else needs
the bridge.

Scope (2026-08-27): the console is the **development loop only**. Studio
distribution (`pack` / `eval` / `release` / `install` / `rollback`) is
deliberately **not** in the Studio UI; it stays on the `vivy-sdk` /
`vivy-studio.exe` command line.

## Host half (`index.js`)

Real Node plugin (no vm sandbox) that registers on the Studio `webServer`:

| Route | Purpose |
| --- | --- |
| `/vivy-console/api/status` `logs` `resolve` | Status (backend + frontend) / unified log timeline / path resolution |
| `/vivy-console/api/start` `stop` `restart` `setExe` | Backend lifecycle. **Start on the managed path first runs `go build -tags vivy_headless -o <scratch>/vivy-backend.exe ./cmd/vivy`** (incremental, output streamed into the backend log), then spawns it with the generated mock config (`VIVY_CONFIG`) — every start reflects current workspace source |
| `/vivy-console/api/frontend/status` `start` `stop` `restart` | Vite dev server lifecycle; the child gets `VIVY_BACKEND_ADDR=http://127.0.0.1:<gateway-port>` so `ui/vite.config.ts` proxies `/rpc` to the live backend port |

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
`vivy-console` order 30 label 「Vivy 控制台」 with three sections:
后端 / 前端 / 日志. The 「前端」 section includes an「打开」button that opens
`http://127.0.0.1:3015` in a new tab (the dev server is the app).

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