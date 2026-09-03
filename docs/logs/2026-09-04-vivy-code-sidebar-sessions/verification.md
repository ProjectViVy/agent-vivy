# Verification

Commands run for this deliverable:

- `go test ./sdk/tui/view ./internal/tui ./internal/tui/view` — passed.
- `cd faces/tui; go test ./...` — passed.
- `go test -race ./sdk/tui/view ./internal/tui ./internal/tui/view` — passed.
- `cd faces/tui; go test -race ./...` — passed.
- `go run ./sdk verify faces/tui` — passed.
- `go run ./sdk pack --face tui --out .workspace/tui-sidebar-sessions-pack-20260904-v2` — passed after the final stale-response and gate fixes; generated `gen_ee57a31f99accde9`, artifact SHA-256 `4d87816977b94cd171a2feb74a99b27e98c78bde521a18cfde7b4d37cc216c2e`.
- `just ci` — passed after the final fixes (UI typecheck, 197 UI tests, build, Go vet/test, headless compile, all plugin/face vet/tests).
- Real PTY smoke: `go run ./cmd/vivy-code` launched the independent terminal
  at 80×24 in compact mode; Ctrl+S opened the Sessions dialog with the real
  private-instance session selected, and Ctrl+C exited cleanly. No tenant
  Journal paths were read or written.

A GPT-5.6-LUNA MAX read-only review found late A→B session-load overwrite,
gate-overlay bypass, asynchronously arriving gate, and send-during-load risks.
Monotonic load/session request fences, gate-first rendering/routing, and a
load fence that preserves the draft were added; deterministic tests cover
each path. Targeted race tests and `just ci` were rerun after these final fixes.

All commands were run from the repository root except the explicitly scoped
`faces/tui` commands. Go was invoked through the installed
`C:/Program Files/Go/bin/go.exe` because this shell's PATH does not expose the
Go installation.
