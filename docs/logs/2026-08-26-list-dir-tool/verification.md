# Verification — 2026-08-26 list_dir tool

## just ci (product gate)

Run from repository root, 2026-08-26:

```
just ci
```

- Exit code 0 (re-run with captured log: `exit=0`, zero `FAIL` occurrences).
- Stages completed in order: fmt-check, vet, `go test ./...`, headless
  compile, UI vitest suite (12 files / 57 tests) and Vite production build.

## Focused tests (pre-gate iteration)

```
go test ./internal/tools/ ./internal/runtime/ ./internal/config/ -count=1
```

All three packages `ok`. New coverage:

- `internal/tools/filesystem_test.go` — `recordingFileOps.ListDir` fake;
  forwarding assertions for `path`/`recursive`/`depth`/`max_entries` and run
  identity; invalid-boolean arg rejection; `list_dir` included in the spec
  sanity loop.
- `internal/runtime/filesystem_backend_test.go` —
  `TestEinoFilesystemBackendListDir`: flat listing with dir/file entry shape
  (size, mtime, is_dir), subdirectory listing, depth-1 recursive equivalence,
  default-depth recursive ordering including listed-but-not-traversed
  `.git`/`node_modules`, depth-2 boundary, `max_entries` truncation flag,
  non-directory target error, and path-escape rejection.

## Real-path smoke (executable behavior, smoke-for-user-visible-change)

Isolated instance (temp data dir + temp sqlite, `127.0.0.1:8799`, dummy
`OPENAI_API_KEY` — preflight never calls the model; no repo `data/` touched):

1. `go build -o <tmp>/vivy-smoke.exe ./cmd/vivy`, config.yaml with
   `storage.sqlite.path`/`data_dir` inside the temp dir; server started with
   CWD = temp dir (needed `fixtures/provider/*` copied alongside).
2. `GET /healthz` → `{"status":"ok","stage":"e2-recovery"}`.
3. `GET /rpc/bootstrap` → RPC token.
4. Node WebSocket JSON-RPC client: `session/create` → `preflight/run` with
   text "List the workspace directory tree so I can see what is here."

Result:

```
selected_tools: ["list_dir"]
tool_decisions: [{"tool_name":"list_dir","decision":"allow","reason":"readonly tool is allowed by default"}]
SMOKE PASSED
```

This exercises the full registration path: default `tools.enabled` config →
registry resolve → keyword selector → policy decision, through the real
control plane. Tool execution semantics are covered by the backend tests
above (workspace isolation, symlink/escape rejection, bounds).

Server terminated (`taskkill`), port confirmed closed, temp directory
removed.
