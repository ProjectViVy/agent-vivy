# HITL Release Closure Verification

Date: 2026-08-12

## Deterministic boundary

The Playwright server uses a fresh SQLite database and workspace under
`ui/.e2e-workdir`, the mock provider, and loopback-only configuration. No API
key is required and no live HTTP/MCP endpoint is used by the mandatory gate.

The test-only provider accepts these scenario values: `hitl`, `approval`,
`question`, `timeout`, and `stale`. The UI smoke uses `hitl` with the explicit
messages `e2e approval: save a note` and `e2e question`.

## Commands

| Check | Result |
|---|---|
| `gofmt -l .` | pass; no files reported |
| `go test ./...` | pass |
| `go test -race ./...` | pass |
| `go vet ./...` | pass |
| `npm run build` | pass |
| `npm run e2e` | pass; 1 real Go-server Playwright test |

The Go binary used by Playwright was rebuilt from `./cmd/vivy` after the
runtime and UI changes.

## Existing lifecycle evidence included in the gate

- approval approve/deny, expiry sweeper, cancellation, and decision guards;
- question answer, cancellation, and restart recovery;
- SQLite first-writer-wins decisions and concurrent journal writers;
- filesystem and Skills precondition/stale-target checks;
- local HTTP/MCP backend bounds and transport tests;
- ReviewItem redaction and metadata projection tests.
