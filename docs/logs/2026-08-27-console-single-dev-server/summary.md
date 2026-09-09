# Vivy Console — single dev model: pure-API backend + DEV dev server only

Date: 2026-08-27
Scope: Vivy Studio overlay (`studio/dsh-vivy-console` + one tiny
`ui/vite.config.ts` knob), not the Vivy kernel.

## What changed

Per user request: the console **no longer splits into an embedded-UI version
and a standalone-UI version**, and it no longer starts/debugs either of those
as separate targets. The unified development loop is exactly the split pair
from AGENTS.md:

1. **Backend = pure-API binary, no embedded frontend.** The console's
   「Backend」 pane compiles and runs the `vivy_headless` build
   (`go build -tags vivy_headless -o <scratch>/vivy-backend.exe ./cmd/vivy`)
   into Studio scratch — the binary serves only the `/rpc` control plane
   (`ui/headless.go`; `/` is a 404). Every managed start recompiles
   incrementally so the backend always matches workspace source; an explicit
   EXE override or `VIVY_HEADLESS_EXE` skips the compile. The stale
   "rebuild your `vivy.exe`" requirement is gone — the console builds the
   right artifact itself.

2. **Frontend = the DEV dev server only.** `pnpm dev` (Vite `:3015`) is the
   single user-facing app. `ui/vite.config.ts` now reads
   `VIVY_BACKEND_ADDR` for the `/rpc` proxy target (default unchanged:
   `http://127.0.0.1:8787`), and the console passes the managed backend's
   live port (`VIVY_BACKEND_ADDR=http://127.0.0.1:<port>`) — so the dev
   server stays correct even when the backend lands on a probed port
   (8787 busy → 8790+).

3. **Dual-mode VIVY WEB debugging removed.** The 「VIVY WEB」 section
   (embedded proxy `:3090/vivy-web` of the gateway's embedded UI vs direct dev connection
   `:3015`) is deleted, along with:
   - the `/vivy-web/*` same-origin proxy + HTML rewrite in `index.js`,
   - the `hook.js` frontend-debug bridge (console/RPC capture + evaluate),
   - the Studio-origin `/vivy-config.json` route (its only consumer was the
     proxied embedded page),
   - the `allowed_origins` block in the generated mock config (nothing is
     cross-origin anymore: the Vite page talks to `/rpc` same-origin through
     its own proxy).
   The console sections are now Backend / Frontend / Logs; the Frontend pane has a
   plain 「Open http://127.0.0.1:3015」 button (the dev server *is* the app —
   frontend debugging happens in the normal browser).

### Files

- `studio/dsh-vivy-console/index.js` — host half: backend lifecycle now
  compiles the headless build and spawns it with the generated mock config;
  `/vivy-web` proxy, `hook.js` route, `/vivy-config.json` registration and
  `allowed_origins` removed; frontend child gets `VIVY_BACKEND_ADDR`.
- `studio/dsh-vivy-console/client.js` — client half: WebPane (VIVY WEB
  standalone-tab debugging) deleted; three sections Backend/Frontend/Logs;
  BackendPane shows Form 「Pure API backend (vivy_headless, no embedded frontend)」;
  FrontendPane shows the live `/rpc` target + Open button.
- `studio/dsh-vivy-console/hook.js` — **deleted** (no injection point).
- `studio/dsh-vivy-console/package.json` / `README.md` — scope rewritten;
  `files` entry for `hook.js` removed.
- `ui/vite.config.ts` — `/rpc` proxy target honors `VIVY_BACKEND_ADDR`.
- Installed profile copy synced:
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`
  (including removing the stale installed `hook.js`).

## What was explicitly not done

- No Vivy kernel / `internal/` / `cmd/` / `sdk/` changes; DB and
  `vivy-studio.exe` CLI semantics untouched; Studio distribution stays on
  the command line.
- No `dsh-context` / `dsh-better-sidebar` / `dsh-cost-meter` /
  `dsh-plugin-hub` / `dsh-vivy-studio` changes.
- Page-level console/RPC capture + evaluate are intentionally dropped with
  the standalone-tab debugging; the unified backend+frontend log timeline
  remains the console's debugging surface.
- No `vivy.exe` kernel build was touched: the console compiles `vivy_headless`
  into Studio scratch (`data/studio-home/vivy-console/vivy-backend.exe`),
  which is gitignored runtime scratch.

## Notes

- The previous uncommitted console state (frontend-dev iteration) is folded
  into this deliverable and committed together; the standalone-tab VIVY WEB
  behavior it introduced is superseded by the unified model.
- The superseded `2026-08-27-console-frontend-dev` log had no
  verification/acceptance; this log closes the console feature line with a
  full gate + live smoke.
- A stale console-managed gateway (from the old profile copy, running with
  `allowed_origins` config) was stopped during verification; it was an
  isolated scratch process under `data/studio-home/vivy-console/`, not the
  tenant Journal.
