# dsh-vivy-console

First-party **Vivy Studio console** bundle: full gateway + distribution
lifecycle management from inside the Studio UI. Supersedes
`dsh-vivy-debugger` (the floating bottom-right ◈ button is retired; the
console lives in the conversation view ring beside Trajectory / Context as
a **「Vivy 控制台」** tab).

## What it does

| Area | Contents |
| --- | --- |
| 网关 Gateway | `vivy.exe` status / logs / start / stop / restart / EXE override (mock mode, data isolated under `data/studio-home/vivy-console/`) |
| VIVY WEB | Same-origin facade of the VIVY WEB UI at `/vivy-web/` with a frontend-debug bridge: console capture, JSON-RPC (WebSocket) capture, page-error capture, and an evaluate box that runs expressions in the VIVY WEB page |
| 日志 Logs | Tail of the managed gateway stdout+stderr |
| 生命周期 Lifecycle | Studio distribution ledger (generations / evals / releases / installs / events / worktrees) and actions — `pack` / `eval` / `release` (human-confirmed, NG-25) / `reject` / `install` / `rollback` / `inspect` — all driven through `vivy-studio.exe` (`cmd/vivy-studio`) |

## Host half (`index.js`)

Real Node plugin (no vm sandbox) that registers on the Studio `webServer`:

| Route | Purpose |
| --- | --- |
| `GET /vivy-config.json` | `{"controlPlaneUrl":"http://127.0.0.1:<gateway-port>"}` — points the embedded VIVY WEB app at the managed gateway's RPC plane |
| `GET /vivy-console/hook.js` | The frontend-debug bridge script injected into the proxied VIVY WEB HTML |
| `/vivy-console/api/status` `logs` `resolve` `start` `stop` `restart` `setExe` | Gateway lifecycle (ported from the retired debugger) |
| `/vivy-console/api/lifecycle/list?kind=…` | Ledger read via `vivy-studio.exe --worktree <root> list <kind>` |
| `/vivy-console/api/lifecycle/run` | Spawn `vivy-studio.exe` for `pack/eval/release/reject/install/rollback/inspect`; one job at a time; **release is only forwarded `--actor human --yes` after explicit UI confirmation** |
| `/vivy-console/api/lifecycle/jobs/<id>` | Job progress (status, exit code, output tail) |
| `/vivy-web/*` | Same-origin proxy of the gateway's embedded UI; `text/html` responses are rewritten (absolute asset paths → `/vivy-web/…` + hook injection). RPC stays **direct** to the gateway (the generated mock config adds `server.allowed_origins` for the Studio origin) |

Air gap: gateway data lives only under `data/studio-home/vivy-console/`
(Studio's own scratch); the production Journal (`data/vivy.db`,
`data/demo/`, `data/workspaces/`) is never touched.

## Client half (`client.js`) and bridge (`hook.js`)

- `client.js` is a hand-authored client module in the DSH client-modules
  handoff format (`window.__ModuleLoader__.load({id, factory})`, React via
  the injected `require`) — no build step. It registers
  `conversation.view` id `vivy-console` order 30 label 「Vivy 控制台」.
- `hook.js` is injected by the `/vivy-web` proxy before the app bundle
  loads. It captures `console.*`, page errors, unhandled rejections, and
  WebSocket (JSON-RPC) frames, relaying them to the console tab via
  `postMessage`; it also answers `ping` / `evaluate` commands. URLs have
  their RPC token redacted and frame bodies are capped + secret-looking
  keys masked (D-010).

## Install

Composed into the `vivy-studio` profile by `launch-vivy-studio.ps1`
(dependency + `dsh.profile.bundles` entry), so a normal
`.\launch-vivy-studio.ps1` boots with the console enabled. To apply an
edited source to a running profile: sync the installed copy at
`data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/` and
restart Studio (skin-style injection — no hot reload).

## Requirements

- `vivy.exe` at the repo root (or `VIVY_INSTALL_DIR`, or set in the panel)
- `vivy-studio.exe` at the repo root (build with `just studio`) for the
  lifecycle section; absent → the ledger endpoints report a hint.
- `data/studio-home/` is Studio scratch (ST-2); never point install/rollback
  targets at the source tree or `data/`.
