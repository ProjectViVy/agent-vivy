# Verification — VC-3g files panel

Date: 2026-09-01 | All commands ran in worktree `agent-vivy-vc0` (branch
`feat/vc1a-bash-tool`).

## Go unit tests (full coverage of the new surface)

```
go test ./internal/runtime/ -run 'TestWorkspaceFiles' -race -count=1   → ok (5 cases:
    ListAndRead / ReadRejectsEscapes / ReadBinaryAndTruncation / ListSkipsSymlinks / NotWired)
go test ./internal/rpc/ -run 'TestWorkspaceRPC' -race -count=1         → ok (3 cases:
    DisabledWithoutDep → MethodNotFound / ListAndRead (including missing-argument -32602) / SurfacesInternalErrors)
go build ./...                                                          → ok
```

## just ci (product gate)

```
just ci   → CI_EXIT=0 (first run failed because internal/app/app.go was not gofmt'd;
             after gofmt -w and rerun, golangci-lint / go test ./... / UI tsc+eslint+vitest+build all passed)
```

## Browser e2e (no-provider shell state, real browser)

```
cd ui; npx playwright test files-panel
→ 1 passed (7.7s): open Files panel → assert empty-state copy → Escape closes → reopen remains empty
```

## Kernel RPC end-to-end smoke (real server, WebSocket)

Server: `VIVY_ADDR=127.0.0.1:8791 VIVY_CONFIG=ui/.e2e-workdir/config.yaml go run ./cmd/vivy`
(healthz → `{"status":"ok"}`). After obtaining a token through `/rpc/bootstrap`,
use WebSocket JSON-RPC:

| Call | Result |
| --- | --- |
| `workspace/list {run_id:"run_smoke"}` | `{"files":[{"path":"blob.bin","size":11},{"path":"notes/hello.txt","size":30}],"truncated":false}` (seed files are visible immediately and correctly sorted) |
| `workspace/read {run_id, path:"notes/hello.txt"}` | `{"binary":false,"content":"hello vivy workspace\nline two\n","path":"notes/hello.txt","size":30,"truncated":false}` (content matches byte-for-byte) |
| `workspace/read {run_id, path:"../escape.txt"}` | `-32603 internal error` (rejected without leaking validation details) |
| `workspace/read {run_id, path:"blob.bin"}` | `{"binary":true,"content":"","size":11}` (binary returns only the flag) |
| `workspace/read {run_id}` (missing path) | `-32602 "run_id and path are required"` |
| `workspace/list {}` (missing run_id) | `-32602 "run_id is required"` |
| `workspace/read {run_id, path:"missing.txt"}` | `-32603` (missing file folded into an internal error) |

The smoke seed directory was deleted afterward; the server process was stopped.

## Issues found and fixed during smoke

- The e2e config did not set `runtime.workspace_root`, so `Ensure` fell back to
  the default user root `~/.vivy/workspace`. `ui/e2e/global-setup.ts` now sets
  `runtime.workspace_root` explicitly (to `.e2e-workdir/workspace`); the rerun of
  the files-panel spec passed, and no e2e-created directory remains under
  `~/.vivy/workspace` (grep count 0).
- An older orphan `vivy.exe` was also found (go-build temporary directory path,
  started at 15:18; a `go run` child process that the Playwright webServer had
  previously failed to clean up on Windows) holding the organism lease and
  preventing the first smoke run. Its command line was verified and the orphan
  process was terminated.

## Known remaining items

- The full "real run produces a file → panel browsing" flow with a provider was
  not verified (no key on this machine); the kernel side is covered by the table
  above, while UI run binding depends on `currentRun.id` and shares the Review
  Center source. Verify it on a machine with a key under the UI-E2E track.
- The RPC layer folds path escapes and missing files into `-32603`: safe because
  no details leak, though semantics could be refined (`-32602`/`-32004`). That is
  not done now—paths clicked in the UI's read-only list necessarily exist, and
  Crush has no corresponding finer-grained surface.
