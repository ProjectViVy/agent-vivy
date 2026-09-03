# Verification

Commands were run from the repository root unless noted. Go was invoked via
`C:/Program Files/Go/bin/go.exe` because it is not on this shell's PATH.

| Command | Result |
|---|---|
| `go test ./sdk/tui/... ./internal/tui/... ./internal/rpc/... -count=1` | PASS |
| `cd faces/tui; go test ./... -count=1` | PASS |
| `go test -race ./sdk/tui/... ./internal/tui/... -count=1` | PASS |
| `cd faces/tui; go test -race ./... -count=1` | PASS |
| `go run ./sdk verify faces/tui` | PASS |
| `go run ./sdk pack --face tui --out .workspace/tui-advanced-commands-pack-20260904-v1` | PASS; generation `gen_0b7278891b6d08b3`, SHA-256 `b8704b0b5ac9661b8a19da798ec441cd1da4bc309208a5252b17b7302c83ab5d` |
| `just ci` | PASS; UI typecheck/197 tests/build, Go formatting/vet/tests/headless compile, and every plugin/face gate passed |
| `vivy-code` interactive PTY smoke | PASS at 80x24; `/stats` opened a real aggregate result overlay and Ctrl+C exited cleanly |

The PTY smoke used the independent VIVY CODE private-instance path. No
production tenant Journal path was read or written.

Regression coverage includes argument validation, unavailable/escaped `!/@`
prefixes, confirmation cancellation and session epochs, mutation in-flight
blocking, compact no-op truth, RPC routing, result labels, fork switching, and
compact/rewind history/context convergence in both fullscreen drivers and the
line REPL.
