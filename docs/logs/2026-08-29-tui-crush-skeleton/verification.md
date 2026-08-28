# Crush-style TUI mock skeleton — verification

Date: 2026-08-29

## Commands

| Check | Result |
|---|---|
| `go get` bubbletea + lipgloss; `go mod tidy` | pass (deps in `go.mod`) |
| `gofmt -w` on `internal/tui/demo`, `internal/tui/view`, `cmd/vivy/tui.go` | applied |
| `go test ./internal/tui/...` | **pass** (tui, demo, view) |
| `just ci` | not claimed green on this worktree if HEAD still lacks `ModelResolver` in `internal/app` (pre-existing; unchanged by this lane) |

No browser smoke: TTY-only deliverable. Fullscreen program is covered by
`View()` string assertions under a fixed `WindowSizeMsg`, not a real console.
