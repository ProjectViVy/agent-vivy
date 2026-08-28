# TUI probe Verification

Date: 2026-08-29

## Commands

| Check | Result |
|---|---|
| `gofmt -w` on `cmd/vivy/tui.go`, `internal/tui`, `internal/rpc/protocol.go` | applied |
| `go test ./internal/tui ./internal/rpc` | **pass** (`ok` tui 0.940s, rpc 12.652s) |
| `go test -c ./cmd/vivy` | **blocked** by pre-existing HEAD compile errors in `internal/app` (`undefined: ModelResolver`, `TakeOrganismLease`) — same gap as the face-pack docs lane; not introduced by the TUI files |
| `just ci` | not re-run; same `internal/app` blockage as `docs/logs/2026-08-29-face-pack/verification.md` |

No browser smoke: the probe is a TTY client. Interactive attach to a live
gateway was not executed in this worktree because `cmd/vivy` does not
compose on `faeb76e`.
