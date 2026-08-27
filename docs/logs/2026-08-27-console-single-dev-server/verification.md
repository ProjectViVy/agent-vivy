# Verification — 2026-08-27 console single-dev-server

Commands run from the repo root (`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy`).

## Gate: `just ci`

```text
just ci 2>&1 | Tee-Object -FilePath data/studio-home/just-ci-console-single-dev.log
```

Result: **exit 0** —
- fmt-check: clean
- `go vet ./...`: clean
- `go test ./...`: all packages ok (cached)
- headless-compile: `go test -run '^$' -tags vivy_headless ./cmd/vivy ./ui` ok
  (proves the `vivy_headless` build tag compiles, which the console uses)
- ui-ci: `pnpm install --frozen-lockfile` up to date · `pnpm typecheck` clean ·
  `pnpm test` 57/57 passed · `pnpm build` built

## Headless backend build (what the console runs on start)

```text
go build -tags vivy_headless -o data/studio-home/vivy-console/vivy-backend.exe ./cmd/vivy
```

Result: exit 0, `vivy-backend.exe` 46,726,144 bytes.

## JS syntax

```text
node --check studio/dsh-vivy-console/index.js   # exit 0
node --check studio/dsh-vivy-console/client.js  # exit 0
```

## Live smoke of the new host half (in isolation, mock ctx)

`data/studio-home/vivy-console/smoke.mjs` imports the edited `index.js`, drives
the registered `/vivy-console/api` routes, then stops every spawned child.
Run: `node data/studio-home/vivy-console/smoke.mjs` → **exit 0, SMOKE ALL PASS**
(22 checks):

| Check | Result |
| --- | --- |
| status: autoBuild plan → scratch `vivy-backend.exe`; no legacy `webPath`/`studioOrigin` | PASS |
| `POST /vivy-console/api/start` compiles + spawns headless backend | PASS |
| backend `http://127.0.0.1:8787/healthz` answers | PASS |
| backend `/` returns **404** — no embedded frontend served | PASS |
| backend has no `/vivy-config.json` | PASS |
| `POST /vivy-console/api/frontend/start` (pnpm dev) → `http://127.0.0.1:3015/` serves the app | PASS |
| frontend status: running, `rpcTarget = http://127.0.0.1:8787` (live backend port) | PASS |
| unified `/logs`: backend + frontend lines present | PASS |
| `resolve` reports exe + autoBuild | PASS |
| `stop` + `/frontend/stop`: both children down, status shows stopped | PASS |

Prepared environment: the stale old-profile console gateway (left over from the
superseded iteration, config with `allowed_origins`, data isolated under
`data/studio-home/vivy-console/`) was on :8787 and was stopped with
`taskkill /PID <pid> /T /F` before the smoke; it was Studio scratch, not the
tenant Journal.

## Deployed copy + restart

- Installed profile copy synced and hash-verified identical:
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`
  (`index.js`, `client.js`, `package.json`, `README.md`, `cordis.patch.yml`);
  stale installed `hook.js` removed.
- Studio restarted (detached, `data/studio-home/restart-studio.ps1`); the
  listener on `http://127.0.0.1:3090` came back up and the console tab is
  reachable after a browser refresh (client bundle is served fresh per
  request; host routes load at boot).