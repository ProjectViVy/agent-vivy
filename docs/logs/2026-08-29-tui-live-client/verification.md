# Verification — tui-live-client

Worktree: `../agent-vivy-tui-live` branch `feat/tui-live-client` (from
`8076a25`).

## Commands

```text
go test ./internal/tui/... -count=1
```

Result: PASS

- `agent-vivy/internal/tui` (includes `Live` boot/turn/approval/session tests)
- `agent-vivy/internal/tui/demo`
- `agent-vivy/internal/tui/view`
- `agent-vivy/internal/tui/surface` (no test files)

```text
go build ./cmd/vivy
```

Result: FAIL on this tip for **pre-existing** reasons unrelated to TUI:

- `ui/embed.go: pattern all:dist: no matching files found` (no `ui/dist` in
  clean worktree; gitignored build artifact)
- `internal/app`: undefined `ModelResolver` / `newModelResolver` /
  `provider.NewResolvingChatModel` / `TakeOrganismLease` — main tree had these
  as uncommitted WIP when the TUI skeleton merged; not introduced here.

`just ci` was **not** run end-to-end for the same tip breakage. Gate for this
deliverable is `go test ./internal/tui/...` plus package-local compile of the
touched TUI graph.

## Manual smoke (when a compiling gateway is available)

```text
# terminal A — any tree that can run the gateway
vivy   # or just run

# terminal B — this worktree once cmd/vivy composes, or:
go test ./internal/tui/...   # already green
# then with a built binary that includes cmd/vivy/tui.go:
vivy tui --live --addr 127.0.0.1:8787
```
