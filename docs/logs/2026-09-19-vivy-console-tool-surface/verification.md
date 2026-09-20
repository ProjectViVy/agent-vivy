# Verification

All commands run from the repository root on 2026-09-19.

## 1. The failure was reproduced from live evidence, not inferred

| Check | Result |
|---|---|
| `tools/list` on the running backend at `127.0.0.1:8787` (control-plane WebSocket, token from `/rpc/bootstrap`) | `active: ["echo_info"]`, `overlay_written: false`, catalog size 32 |
| `run_events` (`model.request`) in the Studio scratch Journal | `selected_tools: ["echo_info"]` for `run_7691ce74c6005fff` (2026-09-19 01:26:47) |
| `run_events` (`tool.requested`) for that run | five `tool_search` calls whose queries match the reported symptom verbatim |
| `run_events` (`model.request`) 2026-08-27 → 2026-08-30 (14 runs) | `selected_tools: null` — the pre-two-tier surface was empty, never larger |
| `data/studio-home/vivy-console/config.yaml` | `tools: enabled: - echo_info` (the pin) |

Journal reads were read-only (`node:sqlite`, `readOnly: true`) against Studio
scratch only; no production Journal (`data/vivy.db`, `data/demo/`,
`data/workspaces/`) was opened.

## 2. Real-path smoke: the fixed template boots a backend with the full surface

The existing managed binary (`data/studio-home/vivy-console/vivy-backend.exe`)
was booted on `127.0.0.1:8799` with a config carrying the new template shape
(`tools.approval` only, no `enabled`) and a scratch `VIVY_USER_HOME`:

| Check | Result |
|---|---|
| process | started, did not exit |
| `tools/list` | `overlay_written: false`, `active` = **31 tools**, catalog size 32 |
| all eleven T1 names present | true |
| `echo_info` active | false |

## 3. Live fix on the running process

`tools/set-active` (the product's own RPC) applied the 31-name kernel default to
the live backend:

| Check | Result |
|---|---|
| `set-active` result | `overlay_written: true`, active count 31, T1 present true |
| re-read `tools/list` | active count 31, `echo_info` inactive |
| `data/studio-home/vivy-console/data/settings.yaml` | `tools_enabled:` carries the 31 names |
| gateway log | RPC calls answered, no error line |

The app schedules an engine reload when the merged active set changed
(`internal/app/app.go`, tools live-apply), so the next run binds the new
surface without a restart.

## 4. Unit tests and syntax

```text
node --check <index.js as .mjs>                      exit 0
node --test studio/dsh-vivy-console/config.test.mjs studio/dsh-vivy-console/logs.test.mjs
```

Result: **18 pass, 0 fail** — 4 config tests (1 new) plus the 14 pre-existing
log-model tests.

## 5. Installed-copy sync

| Check | Result |
|---|---|
| SHA256 of all eight shipped files, source vs both profile copies | 16/16 equal |

## 6. `just ci` was not used as this change's gate

Every edited file lives under `studio/` (submodule) or `docs/`. The host gate
(`fmt-check ui-ci vet test headless-compile plugin-ci`) loads no Studio-overlay
JavaScript and cannot observe this defect in either direction — it was green
while the managed model had one tool and stays green now. `studio/`-side
validation is §4 plus the live checks in §2/§3. `git diff --check` is clean.

## 7. Left open

- The running Studio host still holds the pre-fix template in memory; the
  durable template change takes effect at the next Studio launch, or after the
  detached restart used by `vivy-studio-skin` when the user wants it now.
- Until the managed backend has been restarted from a Studio that loaded the
  fixed plugin, Settings → Tools "restore configuration defaults" would write
  the running `config_enabled` (`["echo_info"]`) back over the overlay. The
  persistent fix is the restart; the fixed template then reports the 31-name
  default as `config_enabled`.
- `docs/TODO.md` was not touched: the board is another lane's uncommitted work,
  and the item is closed in this iteration rather than deferred.