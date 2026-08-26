# Execute Timeout Ceiling Verification

All commands run inside the worktree `../agent-vivy-execute-timeout`
(branch `feat/execute-timeout`). `ui/dist` was copied from the root checkout
into the worktree before building because the embedded-UI artifact is
gitignored and a fresh worktree cannot compile `internal/ui` without it.

## Gate

- `just ci` — PASS (exit 0; fmt-check, vet, test ./..., headless-compile,
  ui-ci). First run failed fmt-check on `internal/config/config.go`
  (struct-literal alignment after the edit); fixed with `gofmt -w`, then a
  full clean re-run passed.

## Targeted tests

- `go test ./internal/config/... ./internal/runtime/... -count=1` — ok
  (config 2.1s, runtime 48.7s).

## Real-binary startup smoke (config parse → validate → composition)

Built `vivy-smoke.exe` from the worktree (`go build -o $TEMP/vivy-smoke.exe
./cmd/vivy`) and ran it with `VIVY_CONFIG` against scratch configs under the
user temp dir; SQLite path and bundle_dir pointed into scratch / worktree
fixtures. The root tree's `data/` was never touched.

1. Valid config with `execute_max_timeout_seconds: 210`:
   process starts, listens on `127.0.0.1:8791`, `GET /` → HTTP 200,
   log line `vivy starting addr=127.0.0.1:8791`.
2. `execute_max_timeout_seconds: 601` → exit 1, `startup aborted` with
   `runtime.execute_max_timeout_seconds must be between 1 and 600 seconds`.
3. `execute_max_timeout_seconds: 0` → same rejection as (2).

## Scope note on the smoke shape

The mock provider's scenarios (`hitl`, `approval`, `question`, `timeout`,
`stale`) drive write_note / ask_user / write_file only, so no offline
end-to-end path can exercise an actual execute call. The full browser-UI
smoke slice is therefore intentionally replaced by the real-binary startup
smoke above plus backend-level clamping tests; the missing mock scenario is
filed as TEST-1 in `docs/TODO.md` §0.1.
