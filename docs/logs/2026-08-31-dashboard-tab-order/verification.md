# Verification record

## Commands and results

- `just ci` — passed. It includes Go `fmt-check` / `vet` / `test ./...` / `headless-compile`, plus UI `pnpm install --frozen-lockfile` / `typecheck` / `test` (all 175 cases in 21 files passed) / `build` (the Vite production build succeeded).

## Real-path smoke test (split Vite)

Environment: the split development pair was already running (Vite `http://127.0.0.1:3015` + control plane `127.0.0.1:8787`), and Vite hot-loaded this change.

Browser (the ZCode built-in browser) visited `http://127.0.0.1:3015/dashboard`:

- The tablist order was confirmed as `tab "Token" [selected]` → `tab "Trajectory"` → `tab "Sessions"` (DOM rect x coordinates 332/396/448, consistent from left to right).
- Token is selected immediately by default, and the Token statistics panel renders normally (total Token 3.9K, model distribution, usage trend, and session details).
- Clicking `Trajectory` → `[selected]`; the trajectory panel and toolbar render normally.
- Clicking `Sessions` → `[selected]`; the panel shows the original overview content (runtime status: 12 sessions / 2 active runs / 1 pending Review; recent activity list).

## Coverage note

- Did not run `just ui-e2e` (it is not part of the `just ci` gate; this was a tab reorder, and the browser smoke test covered the interaction path).
