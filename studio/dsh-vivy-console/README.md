# dsh-vivy-console

First-party **Vivy Studio console** bundle: the Vivy development loop from
inside the Studio UI. Supersedes `dsh-vivy-debugger` (the floating
bottom-right ◈ button is retired; the console lives in the conversation view
ring beside Trajectory / Context as a **「Vivy 控制台」** tab).

Scope (2026-08-27): the console is the **development loop only** — gateway
lifecycle + logs + VIVY WEB frontend debugging. Studio distribution
(`pack` / `eval` / `release` / `install` / `rollback`) is deliberately **not**
in the Studio UI; it stays on the `vivy-sdk` / `vivy-studio.exe` command
line.

## What it does

| Area | Contents |
| --- | --- |
| 网关 Gateway | `vivy.exe` status / logs / start / stop / restart / EXE override (mock mode, data isolated under `data/studio-home/vivy-console/`) |
| VIVY WEB | Same-origin facade of the VIVY WEB UI at `/vivy-web/` with a frontend-debug bridge: console capture, JSON-RPC (WebSocket) capture, page-error capture, and an evaluate box that runs expressions in the VIVY WEB page |
| 日志 Logs | Tail of the managed gateway stdout+stderr |

## Host half (`index.js`)

Real Node plugin (no vm sandbox) that registers on the Studio `webServer`:

| Route | Purpose |
| --- | --- |
| `GET /vivy-config.json` | `{"controlPlaneUrl":"http://127.0.0.1:<gateway-port>"}` — points the embedded VIVY WEB app at the managed gateway's RPC plane |
| `GET /vivy-console/hook.js` | The frontend-debug bridge script injected into the proxied VIVY WEB HTML |
| `/vivy-console/api/status` `logs` `resolve` `start` `stop` `restart` `setExe` | Gateway lifecycle (ported from the retired debugger) |
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

- `vivy.exe` at the repo root (or `VIVY_INSTALL_DIR`, or set in the panel).
  The gateway must be a current build: the generated mock config uses
  `server.allowed_origins` (added 2026-08-25), which an older `vivy.exe`
  rejects on startup (`field allowed_origins not found in type
  config.Server`). Rebuild with `go build -o vivy.exe ./cmd/vivy`.
- `data/studio-home/` is Studio scratch (ST-2).
