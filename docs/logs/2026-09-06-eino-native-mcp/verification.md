# Verification

All commands below were run in the `agent-vivy-eino-mcp` worktree on the `feat/eino-mcp` branch; tenant data was not accessed. The real-path smoke accessed only the official mcp-go fixture on the local loopback and isolated Vivy’s data root in a system temporary directory.

## Run

- `gofmt -w internal/runtime/mcp_backend.go internal/runtime/mcp_backend_test.go internal/app/app.go` — passed.
- `go mod tidy` — passed; added direct dependencies `github.com/cloudwego/eino-ext/components/tool/mcp v0.0.9` and `github.com/mark3labs/mcp-go v1.0.0`.
- `go test ./internal/runtime -run '^$' -count=0` — passed (compile check).
- `go test ./internal/runtime -run '^(TestBoundedMCPBodyReaderSemantics|TestEinoMCPBackend)' -count=1 -v -timeout 180s` — passed, 12 MCP/boundary tests; covered JSON/SSE, Eino schema, modern discover without a legacy session, auth rotation, concurrent init, failed-init recovery/close, session retry/no-retry, replace/remove/close, resources/prompts/non-text fail-closed, tools/resources 32-page bounds, aggregate/content/raw bounds, `io.Reader` exact/over-limit/zero-read semantics, error mapping/browser-use/sort, and closed-backend rejection.
- `go test -race ./internal/runtime -run '^(TestBoundedMCPBodyReaderSemantics|TestEinoMCPBackend)' -count=1 -timeout 180s` — passed.
- `go test ./internal/app -run '^TestAppShutdownBounded$' -count=1 -timeout 180s` — passed; verified that the MCP backend rejects operations after App shutdown.
- `just ci` — passed (UI 24 files / 201 tests, UI build, Go vet/test, headless compile, plugin/faces checks; `internal/runtime` 193.582s).
- Real split-path smoke:
  - Started the official Streamable HTTP fixture with `go run github.com/mark3labs/mcp-go/examples/everything@v1.0.0 -t http` (`127.0.0.1:8080/mcp`).
  - Ran `just run` with isolated `VIVY_USER_HOME=<system-temp>/vivy-eino-mcp-smoke-*` and `VIVY_ADDR=127.0.0.1:8797`; ran `pnpm dev` in `ui/` with `VIVY_BACKEND_ADDR=http://127.0.0.1:8797`.
  - Through `http://127.0.0.1:3015/rpc/bootstrap` and the same-origin WebSocket, sequentially executed `settings/mcp/upsert`, `settings/mcp/probe`, resources list/read, `commands/list`, and prompt expand: discovered 6 tools and 101 resources, resource read retained `untrusted=true`, and 2 MCP prompts were found and successfully expanded as a simple prompt.
  - CUA had no available browser, and the CLI corresponding to the `agent-browser` skill was also not installed; no new tools were installed, and the Playwright 1.62.1 already present in the repository was used to open `http://127.0.0.1:3015/mcp`. The page showed `official-local`, the correct endpoint, `Connected`, and `6 tools`; console error was 0, and manual inspection of the full-page screenshot found no error state, mask, or layout anomaly.
  - After the smoke, stopped the official fixture, temporary Vivy, and Vite using the verified PIDs; 3015/8080/8797 were no longer listening. System security policy refused to delete the isolated temporary directory, so it was not mistakenly treated as repository-artifact cleanup; the directory is not in the worktree and will not be committed.
- A historical `go test ./internal/runtime -count=1 -v -timeout 120s` once timed out during the SQLite temporary-database migration in the existing `TestRunWithoutProvenanceKeepsUISource`; the complete runtime tests in a later `just ci` passed, so the behavior was not reproduced as an MCP failure in this run.
