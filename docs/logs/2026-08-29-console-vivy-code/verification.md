# Verification — console Vivy Code panel

Date: 2026-08-29

| Check | Result |
|---|---|
| `node --check studio/dsh-vivy-console/index.js` | pass |
| `node --check studio/dsh-vivy-console/client.js` | pass |
| Sync into `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/` | done |
| Host routes live | `GET /vivy-console/api/code/status` → 200 `ready:true` root=`../agent-vivy-tui-crush` |
| Open panel | `POST /vivy-console/api/code/open {"mode":"demo"}` → `ok:true` |
| Client card visible | served `client.js` contains `Vivy Code` + `openCode` fallback |
| Studio process | **not killed** in the final pass |

Kernel / `just ci`: not required for this Studio-overlay-only change; no
`internal/` or `ui/` product path touched.
