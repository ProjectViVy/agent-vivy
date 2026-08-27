# Verification — Vivy Debugger Studio bundle

Date: 2026-08-22

## Static checks (done)

- `node --check studio/dsh-vivy-debugger/index.js` → OK
- `node --check studio/dsh-vivy-debugger/client.js` → OK
- Profile manifest lists `dsh-vivy-debugger` in `dsh.profile.bundles`
- Installed copy present at
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-debugger/`
  (index.js, client.js, cordis.patch.yml, package.json)

## Runtime checks (after Studio restart)

Studio restarted detached via `.agents/skills/vivy-studio-skin/scripts/restart-studio.ps1`
(WMI Win32_Process.Create, ReturnValue 0). The plugin row `vivy-debugger`
loads as a bundle layer at boot; the floating button is injected into
index.html.

- [ ] `Invoke-WebRequest http://127.0.0.1:3090` HTML contains
      `data-plugin="dsh-vivy-debugger"`
- [ ] `GET /vivy-debugger/api/status` returns JSON with `running:false`
- [ ] `POST /vivy-debugger/api/start` → gateway booted (log line
      `vivy starting`, `data/vivy.db` created under debugger dir)
- [ ] `GET /vivy-debugger/api/status` shows `running:true`
- [ ] `POST /vivy-debugger/api/stop` → stopped
- [ ] `POST /vivy-debugger/api/restart` → stop + start cycle works
- [ ] Browser: floating ◈ button bottom-right; panel opens; production
      `data/vivy.db` / `data/demo/` / `data/workspaces/` untouched

## Notes

- `just ci` covers kernel/UI/docs; the Studio shell skin and this bundle are
  not Go — the relevant gate is the restart + HTTP/browser smoke above
  (see `vivy-studio-skin` skill for the no-hot-reload rule).
