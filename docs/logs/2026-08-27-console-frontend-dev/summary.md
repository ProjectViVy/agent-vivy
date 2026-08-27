# Vivy Console — frontend dev management, unified logs, standalone WEB tab

Date: 2026-08-27
Scope: Vivy Studio overlay (`studio/dsh-vivy-console`), not the Vivy kernel.

## What changed

Per user request, the console's development loop now manages **both sides at
once** and opens VIVY WEB as a **standalone page**:

1. **Frontend dev server management.** The console now owns a second child
   process: the Vite dev server (`pnpm dev` in `ui/`, `127.0.0.1:3015`,
   strictPort; its `/rpc` proxy targets the managed gateway). New
   `/vivy-console/api/frontend/{status,start,stop,restart}` routes; the
   client gains a「前端」section beside「网关」. Stop uses `taskkill /T` on
   Windows (the shell wrapper is `cmd.exe`, so a plain `kill()` could
   orphan Vite).

2. **Unified log timeline.** `/vivy-console/api/logs` now returns one feed
   of `{src: "backend"|"frontend", text}` lines (gateway
   `gateway.out.log`/`gateway.err.log` + Vite `frontend.out.log`/
   `frontend.err.log`), plus per-source running/logPath. The「日志」section
   renders a single timeline with source chips 全部/后端/前端, pause, and
   clear.

3. **VIVY WEB as a standalone tab.** The iframe is gone. The「VIVY WEB」
   section opens the target in its own browser tab via `window.open`
   (`/vivy-web/` proxy facade with debug bridge, or direct Vite `:3015`),
   keeps a handle for evaluate, and drops/rebinds capture with a
   「断开捕获」button. `hook.js` now relays to **both** `window.parent`
   (iframe embedding, retained for compatibility) **and** `window.opener`
   (standalone tab), so the debug bridge works in the new-tab layout.

### Files

- `studio/dsh-vivy-console/index.js` — frontend child state/management,
  unified `readLogs()`, `frontendStatus()` nested in `status()`, new
  `/frontend/*` routes, tree-kill stop, `dispose()` kills both children.
- `studio/dsh-vivy-console/client.js` — FrontendPane, WebPane standalone
  tab (no iframe, window.open + opener relay + evaluate + disconnect),
  LogsPane unified timeline with source filter, 4-section ring
  (网关/前端/VIVY WEB/日志); removed dead `vc-frame` CSS.
- `studio/dsh-vivy-console/hook.js` — `relayTargets()` posts to
  `window.parent` and/or `window.opener`.
- `studio/dsh-vivy-console/package.json` / `README.md` — scope updated.
- Installed profile copy synced:
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`.

## What was explicitly not done

- No Vivy kernel / `internal/` / `cmd/` / `ui/` / `sdk/` changes.
- No other bundles touched (`dsh-context`, `dsh-better-sidebar`,
  `dsh-cost-meter`, `dsh-plugin-hub`, `dsh-vivy-studio`).
- Distribution (pack/eval/release/install/rollback) stays CLI-only, per the
  earlier scope decision.
- The dev-target (`:3015`) standalone tab is a direct cross-origin embed:
  display and evaluate-over-opener work, but no debug-bridge injection there
  (no HTML control) — the UI states this honestly.

## Notes

- Client bundle is served fresh per request, so the new UI is visible after
  a browser refresh without a Studio restart; the host-half routes need the
  detached restart performed after this iteration.
