# Verification

All commands run from the repository root on 2026-09-19.

## 1. The failure is real and reproducible

`internal/workflow` is untracked WIP from another lane, so `go build ./...` is
clean but `go vet ./...` is not; nothing below touches it.

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui` | exit 0 (`ok agent-vivy/cmd/vivy`, `ok agent-vivy/ui`) |
| `go build -tags vivy_headless -o dist/vivy-backend.exe ./cmd/vivy` | exit 0 |

So the **kernel itself builds and starts**. The failure is only in the config
the Studio console hands it:

| Command | Result |
|---|---|
| `VIVY_CONFIG=<console config> VIVY_USER_HOME=<console scratch> go run ./cmd/vivy` | **exit 1** — `startup aborted: ... field bundle_dir not found in type config.Providers` |
| `vivy-backend.exe` with the same env | **exit 1** — same error, `gateway.out.log` tail |

## 2. Live venue reproduction (before the fix)

The running Studio at `http://127.0.0.1:3090` was exercised through the console
plugin's own HTTP API rather than a simulation:

```text
GET  /vivy-console/api/status   -> {"running":false,...,"listening":false,...}
POST /vivy-console/api/start    -> {"ok":false,"message":"进程在监听 8787 前已退出（exit 1）；日志尾部：… startup aborted … field bundle_dir not found …"}
```

This is the user-visible symptom, produced by the shipped code path.

## 3. The fixed producer boots the backend

The console config was regenerated from the fixed template and the managed
binary started through the managed path (`VIVY_CONFIG` + `VIVY_USER_HOME` =
Studio scratch, cwd = `data/studio-home/vivy-console`):

| Check | Result |
|---|---|
| `vivy-backend.exe` with the fixed config | running, not exited |
| `Get-NetTCPConnection -LocalPort 8787 -State Listen` | LISTENING, pid captured |
| `POST /rpc/bootstrap` | HTTP 405 from the control plane (route reached — the server answered) |
| `gateway` stdout | `msg":"vivy starting","addr":"127.0.0.1:8787"`, no `startup aborted` |

## 4. Unit tests

```text
node --test studio/dsh-vivy-console/config.test.mjs studio/dsh-vivy-console/logs.test.mjs
```

Result: **17 pass, 0 fail** — 3 new config tests plus the 14 pre-existing log
model tests (unchanged), confirming the refactor did not disturb the module the
console already imports.

`node --check` on the edited `index.js` (as `.mjs`): exit 0.

## 5. Installed-copy sync

The profile install is pnpm `file:` with hard-linked files, so the checks confirm
**content** identity rather than trusting the copy:

| Check | Result |
|---|---|
| `Select-String 'export function backendConfig'` in source + both profile copies | hit in all three |
| `Select-String 'bundle_dir: '` (the emitted key) in source + both profile copies | no hits |
| SHA256 of the eight shipped files, source vs both profiles | all equal |

## 6. `just ci` was not used as this change's gate

Every file this change touches lives under `studio/` (the submodule) or
`docs/`. The host gate is
`ci: fmt-check ui-ci vet test headless-compile plugin-ci` — it compiles and
tests the Go module, the UI and the independent plugin modules, and loads no
Studio-overlay JavaScript, so it cannot observe this defect in either
direction: it was green while the backend could not start, and it stays green
now that it can. `git diff --check` is clean.

The repo's own board records that the Actions `just ci` job is also red for an
unrelated pre-existing reason (`CI-BROWSER-SMOKE-WEBSERVER`, `docs/TODO.md`
§0.1), so a local rerun would not have been a meaningful signal either way.
`studio/`-side validation is the unit test in §4 plus the acceptance click in
`acceptance.md`.

## 7. Left open

- The live host caches host-plugin code in memory (`dsh plugin` has no reload),
  so the running server was restarted detached via
  `.agents/skills/vivy-studio-skin/scripts/restart-studio.ps1`; the user must
  hard-refresh `http://127.0.0.1:3090` and press 一键启动 to confirm the fixed
  console actually brings the backend up. `just ci` does not cover `studio/`
  files (Go fmt/vet/test only), so that click is the real gate for this change.