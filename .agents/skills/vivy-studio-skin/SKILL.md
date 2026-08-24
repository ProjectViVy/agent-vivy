---
name: vivy-studio-skin
description: Edit the first-party Vivy Studio shell skin — top-left wordmark/title, theme.css, brand.js, index.js — and make the change visible in the running Studio at http://127.0.0.1:3090. Use when the user asks to rename or retitle Studio or its top-left text (改标题 / 改名字 / 左上角 / 改名 / 换皮 / rename / wordmark / title), change visible product copy or theme, or reports a change "didn't take effect" (没生效 / 还是没变). Covers the no-hot-reload trap, syncing the installed profile copy, and the detached safe restart that does not kill the agent's own host server.
---

# Vivy Studio skin

The first-party skin is a **server-side plugin** under
`studio/dsh-vivy-studio/` (`name = "vivy-studio-skin"`, `inject = ["webServer"]`).
`index.js` taps the served index HTML; `theme.css` + `brand.js` are read into
memory **at server boot** — there is no hot reload, and `just ci` does not cover
these files (Go fmt/vet/test only).

## Air gap

- Never read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`
  (tenant Journal).
- `data/studio-home/` is Studio's own scratch — the installed skin copy and
  restart logs live there and are writable.

## Where each visible string lives (theme.css unless noted)

| What you see | Where |
|---|---|
| Top-left wordmark in the sidebar | `theme.css` → `button:has(> svg[viewBox="0 0 182 24"])::after { content: ... }` (`::before` draws the letter tile) |
| Browser tab title | `brand.js` (`PRODUCT` const) and `index.js` (static `<title>` replace) |
| Welcome dialog title / copy | `theme.css` `[class*="dialog"] h2[class*="title"]::after`, `[class*="copy"] p:first-of-type::after` |
| Preview badge | `theme.css` `[class*="previewBadge"]::after` |

Change only the string the user asked for. Do not cascade every
"Vivy Studio" occurrence to the new name unless asked.

## Procedure

1. **Edit the source** under `studio/dsh-vivy-studio/`.
2. **Sync the installed copy.** The profile install is a pnpm `file:` **copy**,
   not a symlink:
   `data/studio-home/profiles/<profile>/node_modules/dsh-vivy-studio/`
   Apply the identical edit there (re-sealing with `dsh plugin add file:...` also
   refreshes it, but a direct sync is surgical). Then search for stale copies:
   `Get-ChildItem data/studio-home -Recurse -Filter <file>` and check each.
3. **Restart the Studio server.** The running server
   (`dsh --profile vivy-studio --port 3090`, launched by
   `launch-vivy-studio.ps1`) caches the skin in memory; only a restart makes the
   change visible.
   - **Never kill the listener from your own process tree.** Agent tool
     processes (pwsh) run as children of the Studio server; an in-tree kill
     kills your own turn and the restart never happens.
   - Launch the bundled `scripts/restart-studio.ps1` **detached**, then finish
     your turn:
     ```powershell
     $r = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{
       CommandLine = 'powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "C:\...\restart-studio.ps1" -WaitSeconds 15' }
     # ReturnValue must be 0. Alternative: one-shot schtasks /Create /IT + /Run + /Delete.
     ```
   - The script: aborts if spawned inside the server's tree, waits
     `-WaitSeconds` (let the turn settle), `taskkill /T /F` the port-3090
     listener, relaunches via a generated `boot-studio.cmd` with an explicit env
     (`DSH_HOME`, `GOPROXY`, node dir on `PATH` — detached processes inherit
     none of the harness env), and logs to `data/studio-home/studio-restart.log`
     + `studio-boot.*.log`.
   - **Always confirm the log appeared** after spawning. A silent exit means the
     script never ran (see pitfalls).
4. **Verify.** After the port is back up (the GUI drops and the user must
   refresh):
   ```powershell
   $html = (Invoke-WebRequest http://127.0.0.1:3090 -UseBasicParsing).Content
   $html.Contains("Nierland")   # or your string — checks the injected <style data-plugin="dsh-vivy-studio">
   ```
   On failure, read `studio-restart.log` and the `studio-boot.*.log` tails.

## Pitfalls (learned the hard way)

- **`$home` is a read-only automatic variable** in PowerShell (collides with
  `$HOME`, case-insensitive). A script assigning `$home = ...` under
  `$ErrorActionPreference = "Stop"` exits instantly with no output. Name it
  `$studioHome`.
- **Detached processes lose the harness env** (`DSH_HOME`, `GOPROXY`, node
  `PATH`); without an explicit boot wrapper `dsh.cmd` cannot find `node`.
- **`dsh plugin` has no reload** — it only forwards to pnpm. Restart is the only
  path; there is no plugin hot-reload.
- The **browser may cache the page**; tell the user to hard-refresh after the
  restart.

## Forbidden

- Killing the Studio listener from an in-tree process (kills your own turn).
- Editing the tenant Journal (`data/vivy.db`, `data/demo/`, `data/workspaces/`).
- Renaming strings the user did not ask for.
