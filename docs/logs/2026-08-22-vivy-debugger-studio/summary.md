# Vivy Debugger — Studio first-party bundle

Date: 2026-08-22
Scope: Vivy Studio overlay (Studio bundle), not the Vivy kernel.

## What changed

The Vivy gateway debugger, previously only a session-local dynamic Cordis
plugin (`vdbg-1`, lost on restart), is now **first-party Studio code**:

- New bundle package `studio/dsh-vivy-debugger/` (name `dsh-vivy-debugger`),
  composed into the `vivy-studio` profile as its own patch layer.
  - `index.js` — Host half: registers `/vivy-debugger/api/*` JSON routes on
    the Studio `webServer` service; manages the `vivy.exe` gateway child via
    `node:child_process` (real Node — no vm sandbox, no pwsh wrapper, no shell
    redirection), with stdout/stderr appended to files under
    `data/studio-home/vivy-debugger/`.
  - `client.js` — Browser half: injected into `index.html` via
    `webServer.tapIndex` (same mechanism as the skin). Floating **◈** button
    pinned bottom-right of the main page; click opens a status/log panel with
    ▶ 启动 / ■ 停止 / ⟳ 重启 and an exe-path override.
  - `cordis.patch.yml` — one insert row `vivy-debugger`.
  - `package.json` / `README.md`.
- `launch-vivy-studio.ps1` now composes `dsh-vivy-debugger` into the profile
  (dependency + `dsh.profile.bundles`), so a fresh launch boots with the
  debugger enabled; the seal check re-runs when the debugger is missing.
- Installed profile (`data/studio-home/profiles/vivy-studio/`) synced:
  manifest + `node_modules/dsh-vivy-debugger` copy.

## What was explicitly not done

- No change to the Vivy kernel, `internal/`, `cmd/`, or `ui/` (kernel scope
  stays separate).
- No production Journal access: gateway data is isolated under
  `data/studio-home/vivy-debugger/` with mock mode (ST-2 air gap).
- The dynamic `vdbg-1` plugin was left running as-is; this bundle is the
  durable replacement. It is NOT a kernel plugin (no `vivy-sdk` pack).
- `studio/dsh-plugin-hub/` (third-party, untracked) was not touched and is
  not part of this deliverable.

## Files

- `studio/dsh-vivy-debugger/index.js`, `client.js`, `cordis.patch.yml`,
  `package.json`, `README.md`
- `launch-vivy-studio.ps1`
- Installed profile copy: `data/studio-home/profiles/vivy-studio/`
