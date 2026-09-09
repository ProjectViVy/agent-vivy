# Verification — Vivy Console scope trim + gateway-start fix

Date: 2026-08-27

## Commands run

| Step | Command | Result |
| --- | --- | --- |
| Gate | `just ci` (root) | ✅ exit 0 — gofmt, `go vet`, `go test ./...`, headless compile, 57 UI tests, Vite build ok |
| Binary rebuild | `go build -o vivy.exe ./cmd/vivy` (Go 1.26.4) | ✅ exit 0; new `vivy.exe` embeds current `ui/dist` |
| Syntax | `node --check studio/dsh-vivy-console/{index.js,client.js,hook.js}` | ✅ OK ×3 |
| Dangling refs | grep `lifecycle|resolveStudioExe|MAX_JOB_LINES|jobSeq|jobs\.|LifecyclePane|LEDGER_KINDS|ACTION_LABELS|useCallback` in console | ✅ only "gateway lifecycle" comments remain |
| Profile sync | copy console files → `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/` | ✅ hashes match source for all 6 files |
| Client bundle | `GET /plugins/dsh-vivy-console/client.js` on running Studio | ✅ 200; `LifecyclePane` / `lifecycle` / `vc-tbl` absent; gateway/log tabs present |

## Live console API smoke (running Studio, root-cause fix)

| Step | Result |
| --- | --- |
| `POST /vivy-console/api/start` | ✅ `{"ok":true,"pid":3424,"message":"Started … listening on 127.0.0.1:8787 (mock mode, isolated data)"}` |
| `GET /vivy-console/api/status` | ✅ `running:true, managed:true, listening:true, addr:127.0.0.1:8787, studioPort:3090` |
| Gateway log | ✅ `"level":"INFO","msg":"vivy starting","addr":"127.0.0.1:8787"` — no parse error (previous runs showed `field allowed_origins not found in type config.Server` twice) |
| `GET http://127.0.0.1:8787/healthz` | ✅ `{"status":"ok","stage":"e2-recovery"}` |
| `POST /vivy-console/api/stop` | ✅ `{"ok":true,"message":"Stopped (PID …)"}`; status flips to `running:false` |
| `POST /vivy-console/api/start` (again) | ✅ second start succeeds |
| `GET /vivy-console/api/logs` | ✅ JSON lines with INFO startup entry |
| `GET /vivy-web/` (proxied VIVY WEB) | ✅ 200; `hook.js` injected; asset paths rewritten to `/vivy-web/assets` |
| `GET /vivy-config.json` | ✅ `{"controlPlaneUrl":"http://127.0.0.1:8787"}` |
| `GET /vivy-console/api/lifecycle/list?kind=generations` (pre-restart old host) | answered (old in-memory host code still had the route — documented, removed by restart) |

## Pending (next turn)

- Detached Studio restart (`restart-studio.ps1`), then browser smoke at
  `http://127.0.0.1:3090`: the 「Vivy Console」 tab shows only Gateway / VIVY WEB /
  Logs (no Lifecycle); `GET /vivy-console/api/lifecycle/list` now 404s.
