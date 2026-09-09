# Vivy Console — Studio first-party console bundle (dsh-vivy-console)

Date: 2026-08-27
Scope: Vivy Studio overlay (Studio bundle), not the Vivy kernel.

## What changed

The gateway debugger is upgraded from a floating bottom-right button into a
first-party **Vivy Console** (`dsh-vivy-console`) that manages the Vivy
development/distribution lifecycle inside the Studio UI, per the user's
three requests:

1. **Full lifecycle management component** — the console owns the gateway
   lifecycle (status / logs / start / stop / restart / EXE override) and the
   Studio distribution ledger (`generations / evals / releases / installs /
   events / worktrees`) plus the actions `pack / eval / release / reject /
   install / rollback / inspect`, all executed through `vivy-studio.exe`
   (`cmd/vivy-studio`). Release keeps the human gate (NG-25): the API only
   forwards `--actor human --yes` after an explicit UI confirmation
   (`confirm:true`); the CLI refuses any other actor or a missing `--yes`.
2. **VIVY WEB in Studio with frontend debugging tools** — the console serves
   a same-origin facade of the VIVY WEB UI at `/vivy-web/` (proxy through
   the Studio webServer; `text/html` responses are rewritten: absolute
   asset paths → `/vivy-web/…` and a debug-bridge script injected before the
   app bundle). The bridge (`hook.js`) captures `console.*`, page errors,
   unhandled rejections, and WebSocket (JSON-RPC) traffic and relays them to
   the console tab via `postMessage`; the tab offers reload / open-in-new-
   tab / target switch (embedded gateway UI vs Vite `127.0.0.1:3015`) /
   capture filters / clear / an evaluate box that runs expressions in the
   VIVY WEB page. The embedded app's RPC stays **direct** to the gateway:
   the generated mock config adds `server.allowed_origins` for the Studio
   origin, and a root `/vivy-config.json` route returns the gateway
   `controlPlaneUrl`.
3. **Old plugin retired, new placement** — the floating bottom-right ◈
   debugger (`studio/dsh-vivy-debugger/`) is removed from the profile and
   deleted. The console registers a **「Vivy Console」** tab in the
   `conversation.view` ring (id `vivy-console`, order 30) beside
   Trajectory and Context (dsh-context's pattern), via a hand-authored
   client module in the DSH client-modules handoff format (no build step).

### Files

- New bundle `studio/dsh-vivy-console/`: `index.js` (host half), `client.js`
  (conversation-view tab), `hook.js` (frontend-debug bridge), `package.json`
  (`dsh.bundle` patch + `dsh.client` declaration + `exports["./client"]`),
  `cordis.patch.yml` (plugin row `vivy-console`), `README.md`.
- `launch-vivy-studio.ps1`: composes `dsh-vivy-console` instead of
  `dsh-vivy-debugger` (deps + bundles + seal marker), and removes a stale
  installed `dsh-vivy-debugger` copy.
- Deleted `studio/dsh-vivy-debugger/` (retired).
- `.gitignore`: `.zcode/` local agent scratch.
- Baseline commit `0b97a1c` landed the previously uncommitted Studio bundle
  state (skin theme, launch script, five first-party/vendored bundles, two
  delivery logs) so the console work sits on a clean tree.

## What was explicitly not done

- No changes to the Vivy kernel, `internal/`, `cmd/`, `ui/`, `sdk/`, or the
  DSH harness checkout. No Go/DB schema changes; lifecycle authority stays
  with `vivy-studio.exe` and its ledger (`data/studio-home/studio.db`).
- No auto-release, no hot-swap of a live process (CLI semantics unchanged).
- The dev-target (Vite 3015) iframe is a direct cross-origin embed: display,
  reload, and open-in-tab work, but the debug bridge cannot be injected
  there (no HTML control) — the UI states this honestly.
- `dsh-context`, `dsh-better-sidebar`, `dsh-cost-meter`, `dsh-plugin-hub`
  were not modified.
- Studio-ledger reads/actions are only reachable while `vivy-studio.exe`
  exists (built by `just studio`); otherwise the API reports a hint.

## Notes

- Baseline hygiene: the root tree held uncommitted Studio state (launch
  script, skin theme, five bundles, two logs). It was committed first as one
  baseline commit; the console ships as its own focused commit.
- The `vivy-studio` profile manifest was updated and `pnpm install` re-ran
  to materialize `dsh-vivy-console`; the stale `dsh-vivy-debugger` install
  was removed so the retired FAB cannot linger.
