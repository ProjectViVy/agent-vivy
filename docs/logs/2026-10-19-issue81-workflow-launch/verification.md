# Verification — workflow launch (W2)

Same lane and host substitutions as
`../2026-10-19-issue81-workflow-inputs/verification.md`:
`GOTOOLCHAIN=go1.26.4`, lane-local `GOCACHE`, and
`npm_config_manage_package_manager_versions=false` (the host's pnpm 12 vs the
pinned `pnpm@10.33.0`); `just` is absent, so the `justfile` recipes were run as
their component commands.

## Commands and results

| Command | Result |
| --- | --- |
| `pnpm exec vitest run ../plugins/vivy-workflow/ui/vivy-workflow/src` | 6 files / 45 tests pass (launch-context 6, face-bridge 15, workflow-page 6, NodePanel 6, graph 7, edit 5) |
| `./node_modules/.bin/vitest run` (full UI suite) | 9 failed / 86 passed files, 39 failed / 680 passed tests — identical pre-existing failures to the W1 baseline (`happy-dom` `localStorage`); no new failure |
| `./node_modules/.bin/tsc --noEmit` | clean |
| `go test ./internal/runtime -run 'TestWorkflowProductLaunch' -count=1` | pass: active parent admitted and owns the child; unknown parent `ErrNotFound`; terminal parent `ErrChildAdmissionConflict`; no rejected launch admits a child; admitted revision keeps parent/session/operation key |
| `go test ./internal/runtime -run 'TestWorkflowProduct' -count=1` | all product tests pass (draft lifecycle, root pointer, unknown-node rejection, run binding, draft start, capabilities, cancel, disabled path, inputs roundtrip, launch admission) |
| `node scripts/fmt-check.mjs` | 1222 Go files formatted, clean |
| `go vet ./...` | clean |
| `node scripts/plugin-ci.mjs` | 22 modules passed (workflow Module included) |
| `node scripts/check-i18n-completeness.js` | PASS (en/zh 1575 keys, no new key) |
| `node --test scripts/check-i18n-cross-face.test.js` + `node scripts/check-i18n-cross-face.js` | PASS (14 shared semantic units) |
| `node ui/scripts/stage-ui-assembly.mjs` + `./node_modules/.bin/vite build` | built `ui/dist` for the `go:embed` browser gate |
| `go run ./sdk verify plugins/vivy-workflow` | `ok vivy/workflow-ui` with digest `ed919b60…` (both descriptor copies agree; recomputation returns the same value) |
| `go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui` | ok |
| `node scripts/memory-loop.mjs` | exit 0 (DIVA overlay pack + app tests) |
| `go test -p 1 -timeout 35m ./...` | first pass failed only in `sdk/internal/conformance`: `exec vitest run src/plugins/conformance.test.tsx: Error: ERR_PNPM_LOCKFILE_WRITE_FILE` — the pnpm version-switch, plugged by exporting `npm_config_manage_package_manager_versions=false`; the rerun of that package passes (`ok … 41.1s`). Final whole-tree run with the override: 83 packages ok, one failure — `TestClientAgainstRealVivyCode` (`codeclient: handshake get_state: codeclient: protocol stream ended`), which reproduces identically on the unmodified root checkout at `main` (`go test -run TestClientAgainstRealVivyCode ./sdk/codeclient` → same error), so it is pre-existing/environmental and outside this change. |
| `./node_modules/.bin/playwright test --config playwright.workflow.config.ts workflow-correctness.spec.ts` | 4 tests pass (6.5 s): author/publish; idle disabled + reason; finished-run disabled + reason; active parent → publish → Run → Runs tab `completed`, model request body contains the authored literal |

## Browser evidence detail

Isolated pair only: backend `127.0.0.1:18987`, Vite `127.0.0.1:3195`, loopback
model `127.0.0.1:18988`, disposable `ui/.workflow-e2e/`, system Chrome
(`channel: 'chrome'`), `en-US`. The active-parent case holds the parent's model
call open with the loopback `hang` reply (so the Run stays `active` with a live
tool ceiling), then launches the workflow from the same session; the child's
model call consumes the queued text reply. Assertions read committed state
(`inofy.getRevision`, `inofy.listRuns`, `inofy.getRun`), not mocked callbacks.

## Known limitations

- No backend regression suite exercises the *UI* predicate; the contract is
  pinned on both sides (unit truth table + backend admission test).
- Independent idle launch remains unavailable by design (see `summary.md`).
