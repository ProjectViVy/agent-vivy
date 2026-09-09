# Vivy Console — Main Console (main console) interaction

Date: 2026-08-27
Scope: Vivy Studio overlay (`studio/dsh-vivy-console` client), not the Vivy
kernel. No host-half change.

## What changed

Per user request (the unified dev model "works now, but the interaction feels
fragmented"), the console's three sections Backend/Frontend/Logs are reorganized into
a **Main Console** so backend and frontend are not split across separate tabs:

1. **Main Console section** (was separate Backend + Frontend tabs):
   - one overall state line: All running / Backend running · Frontend not running /
     Frontend running · Backend not running / Not running;
   - **▶ Start all / ■ Stop all / ⟳ Restart all** — the client orchestrates the
     existing host routes sequentially (start: backend → frontend; stop:
     frontend → backend; restart: stop both → start both), reporting a
     combined result message; "Already running" from one side is treated as a
     no-op and the other side still runs;
   - two side-by-side status cards (backend + frontend): Status / PID /
     Listening / Started at / EXE / Form (pure API · vivy_headless) / Build method, and
     Entry / Command / Proxy (/rpc → Backend), respectively;
   - each card keeps its own individual Start/Stop/Restart (and the backend EXE
     override input) so fine control is not lost.
2. **Logs section** unchanged (unified Backend+Frontend timeline, source chips,
   pause, clear).
3. No host API change — the one-click actions reuse
   `/vivy-console/api/start|stop|restart` and `/frontend/*`.

### Files

- `studio/dsh-vivy-console/client.js` — two sections Main Console/Logs; OverviewPane
  (one-click orchestration + backend/frontend status cards) replaces the
  separate BackendPane/FrontendPane tabs; small `vc-card-title` style added.
- `studio/dsh-vivy-console/README.md` — client-half section rewritten for the
  Main Console layout.
- `studio/dsh-vivy-console/package.json` — description updated.
- Installed profile copy synced:
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`.

## What was explicitly not done

- No host-half (`index.js`) change, no kernel / `internal/` / `cmd/` / `ui/`
  change — so no Studio restart is required: the client bundle is served
  fresh per request and appears after a browser refresh.
- No change to the unified model (pure-API backend + Vite dev server only);
  the embedded/standalone dual-mode debugging stays removed.
- No new host routes (one-click is client-side orchestration of the existing
  API).

## Notes

- One-click Start all starts the backend first (which incrementally compiles
  `vivy_headless` when not overridden) and only then the frontend; a backend
  start failure aborts the sequence with the backend message.
- Status polling and the rest of the console behaviour are unchanged from the
  previous iteration.
