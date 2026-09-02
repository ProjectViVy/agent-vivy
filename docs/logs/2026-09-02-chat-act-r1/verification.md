# Verification — UI-CHAT-ACT R1

Kernel-only slice: no UI-visible change, so no 3015 smoke (R2 brings the
browser-facing wiring).

Commands (from repo root, Windows Git Bash):

| Command | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./internal/runtime ./internal/rpc ./internal/storage/... ./internal/app` | clean |
| `gofmt -l internal/` | empty |
| `go test ./internal/runtime -run TestRewind -count=1` | ok (3 tests) |
| `go test ./internal/storage/sqlite ./internal/storage/postgres -run TestBackendConformance/CN-21 -count=1 -v` | sqlite ok (after fixing my own CN-21 assertion: cutoff is exclusive); postgres SKIP (needs live DATABASE_URL) |
| `go test ./internal/storage/... -count=1` | ok (sqlite full conformance CN-01..CN-21, 23s) |
| `go test ./internal/runtime -race -count=1` | ok (141s) |
| `go test ./internal/rpc -count=1` | ok (incl. new `TestSessionRewindRoute`) |
| `go test ./internal/domain ./internal/app -count=1` | ok after bumping the event-vocabulary guard 36→37 for `session.truncated` |
| `just ci` | CI-EXIT:0 (log: `/tmp/ci-chat-act-r1.log`) |

Tests added:

- `internal/storage/conformance/suite.go` — CN-21 `cnSessionTruncationMarkers`
  (newest-marker-wins per session, exclusive fold, fork pass-through,
  stale-cutoff fail-open), suite guard 20→21.
- `internal/runtime/rewind_service_test.go` — mark+filter+audit event,
  validation (unknown cutoff / busy / foreign-run no-block / unwired).
- `internal/rpc/control_test.go` — `TestSessionRewindRoute` happy path,
  filtered `session/messages`, NotFound / InvalidParams / Conflict.
- `internal/domain/domain_test.go` — vocabulary guard 36→37.
