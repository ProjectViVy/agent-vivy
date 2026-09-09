# Verification — Vivy Console frontend dev + unified logs + standalone WEB

Date: 2026-08-27

## Commands run

| Step | Command | Result |
| --- | --- | --- |
| Gate | `just ci` (root) | ✅ exit 0 — gofmt, vet, go test, headless compile, 57 UI tests, Vite build ok |
| Syntax | `node --check studio/dsh-vivy-console/{index.js,client.js,hook.js}` | ✅ OK ×3 |
| Dangling refs | grep `iframe|vc-frame|frameKey|lifecycle` in client.js | ✅ none (only header text + removed CSS) |
| Profile sync | copy console files → `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/` | ✅ hashes match for all 6 files |
| pnpm launcher | `cmd /c pnpm --version` | ✅ 10.33.0 (host spawn `shell:true` resolves `pnpm.cmd`) |
| Served client bundle | `GET /plugins/dsh-vivy-console/client.js` on running Studio | ✅ 200, len 22083; contains FrontendPane / "Frontend" / "Open standalone page" / backend·frontend chips; no iframe / vc-frame; no LifecyclePane |

## Pending (next turn, after detached Studio restart)

- Browser smoke at `http://127.0.0.1:3090`:
  1. 「Vivy Console」shows 4 sections: Gateway / Frontend / VIVY WEB / Logs.
  2. Gateway pane: start mock gateway → running; Frontend pane: start Vite dev
     (`:3015`) → running; Logs pane shows one timeline with `[Backend]` /
     `[Frontend]` tags and source-chip filtering.
  3. VIVY WEB pane: 「Open standalone page」opens `/vivy-web/` in a new tab; RPC
     capture relays back to the console tab via `window.opener`.
  4. Old lifecycle routes `GET /vivy-console/api/lifecycle/*` still 404.
