# dsh-vivy-debugger

First-party Vivy Studio **gateway debugger** bundle: view the Vivy gateway
(`vivy.exe`) status and logs, and start / stop / restart it — directly from
the Studio UI.

- Host half (`index.js`): registers `/vivy-debugger/api/*` JSON routes on the
  Studio's `webServer` service and manages the gateway child process with
  `node:child_process` + file-backed logs. Real Node — no vm sandbox, no
  `pwsh` wrapper, no shell redirection.
- Browser half (`client.js`): injected into `index.html` via
  `webServer.tapIndex` (same mechanism as the skin plugin). Renders a
  floating **◈** button pinned to the bottom-right corner of the main page;
  clicking opens a status/log panel with ▶ 启动 / ■ 停止 / ⟳ 重启.
- Data is fully isolated under
  `<root>/data/studio-home/vivy-debugger/` (Studio's own scratch): the
  generated `config.yaml` runs the gateway in **mock mode** on a free
  loopback port with its own SQLite file. The production Journal
  (`data/vivy.db`, `data/demo/`, `data/workspaces/`) is never touched
  (ST-2 air gap).

## API

| Route | Method | Purpose |
| --- | --- | --- |
| `/vivy-debugger/api/status` | GET | running state, PID, addr, listening, exe, config/log paths |
| `/vivy-debugger/api/logs` | GET | tail of the gateway stdout+stderr files (max 300 lines) |
| `/vivy-debugger/api/resolve` | GET | resolved `vivy.exe`, config path, workspace root |
| `/vivy-debugger/api/start` | POST | write isolated config, spawn the gateway |
| `/vivy-debugger/api/stop` | POST | stop the managed child (or an external vivy via taskkill) |
| `/vivy-debugger/api/restart` | POST | stop then start |
| `/vivy-debugger/api/setExe` | POST | override the `vivy.exe` path (`{ "path": "..." }`) |

## Install

The bundle is composed into the `vivy-studio` profile by
`launch-vivy-studio.ps1` (dependency + `dsh.profile.bundles` entry), so a
normal `.\launch-vivy-studio.ps1` boots with the debugger enabled. To apply
it to an already-running profile:

```powershell
# 1. edit sources under studio/dsh-vivy-debugger/
# 2. sync the installed copy:
#    data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-debugger/
# 3. add to the profile manifest (dependencies + bundles), then restart Studio.
```

The floating button appears at the bottom-right of the main page after the
Studio server restarts (skin-style injection — no hot reload).
