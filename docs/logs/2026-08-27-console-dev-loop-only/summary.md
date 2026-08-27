# Vivy Console — scope trim to the development loop + gateway-start fix

Date: 2026-08-27
Scope: Vivy Studio overlay (`studio/dsh-vivy-console`), not the Vivy kernel.

## What changed

Per explicit user decision, the console is trimmed to the **development
loop only** and the "plugin doesn't work" root cause is fixed.

1. **Lifecycle pane removed.** The `dsh-vivy-console`「生命周期」tab
   (`pack` / `eval` / `release` / `reject` / `install` / `rollback` /
   `inspect` + the generations/evals/releases/installs/events/worktrees
   ledger) is deleted from the client and its host routes
   (`/vivy-console/api/lifecycle/*`, the `jobs` map, `lifecycleList` /
   `lifecycleRun` / `jobSnapshot`, `resolveStudioExe`) are removed from the
   host. Distribution stays on the `vivy-sdk` / `vivy-studio.exe` command
   line — the Studio UI no longer exposes packaged-artifact management.
   This aligns the console with the "frontend/backend separation is a
   development-mode concern; `vivy.exe` remains a tenant product" contract
   in AGENTS.md: the console manages the dev loop, not the release pipeline.

2. **Gateway-start root cause fixed.** The console's generated mock config
   sets `server.allowed_origins` (needed so the embedded VIVY WEB RPC can
   reach the gateway cross-origin). The root `vivy.exe` was a stale build
   from 2026-08-23, predating `AllowedOrigins` (added 2026-08-25 in commit
   `ba18c1b`); config parsing is strict (`KnownFields(true)`), so the
   gateway aborted with `field allowed_origins not found in type
   config.Server` and never came up — the console was unusable
   ("插件用不了"). Rebuilt `vivy.exe` from current source
   (`go build -o vivy.exe ./cmd/vivy`); the gateway now starts, serves
   `/healthz`, and the `/vivy-web/` facade + debug-bridge injection work.

### Files

- `studio/dsh-vivy-console/index.js` — host half: dropped lifecycle API
  routes + job machinery; header comment now states the dev-loop-only
  scope.
- `studio/dsh-vivy-console/client.js` — client half: deleted the
  Lifecycle pane (component, ledger kinds, action labels, lifecycle-only
  CSS, section entry); header comment updated.
- `studio/dsh-vivy-console/package.json` — description updated to the
  dev-loop-only scope.
- `studio/dsh-vivy-console/README.md` — scope statement, route table,
  and requirements rewritten; documents the `vivy.exe` build requirement
  (the `allowed_origins` regression).
- Installed profile copy synced:
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`.

## What was explicitly not done

- No Vivy kernel / `internal/` / `cmd/` / `ui/` / `sdk/` changes; no Go or
  DB schema changes; `just ci` gate unchanged.
- No changes to `dsh-context`, `dsh-better-sidebar`, `dsh-cost-meter`,
  `dsh-plugin-hub`, `dsh-vivy-studio`.
- `vivy-studio.exe` CLI semantics (pack/eval/release/install/rollback)
  are untouched — only the Studio UI no longer drives them.
- The rebuilt `vivy.exe` is a local build artifact (`/vivy.exe` is
  gitignored), not committed.

## Notes

- The stale binary failure mode is now documented in the console README
  (rebuild with `go build -o vivy.exe ./cmd/vivy`), so a future
  developer who hits `field allowed_origins not found` knows the fix.
- Client bundle is served fresh per request, so the tab change is visible
  after a browser refresh; the host-half route removal needs the Studio
  restart performed after this iteration.
