# Verification

Commands were run from the repository root unless noted. Go uses
`C:/Program Files/Go/bin/go.exe` because it is not on this shell's PATH.

| Command | Result |
|---|---|
| `go test ./sdk/tui/... ./internal/tui/... -count=1` | PASS |
| `cd faces/tui; go test ./... -count=1` | PASS |
| `go test -race ./sdk/tui/... ./internal/tui/... -count=1` | PASS |
| `cd faces/tui; go test -race ./... -count=1` | PASS |
| `go run ./sdk verify faces/tui` | PASS |
| `go run ./sdk pack --face tui --out .workspace/tui-thinking-mode-pack-20260904-v2` | PASS; generation `gen_e529705c6d29e5db`, SHA-256 `5b04ac8841ebc96444a844cb845cad5a9536c6496de6e9127000f75dcd1f1fb9` |
| `just ci` | PASS; UI typecheck/197 tests/build, Go formatting/vet/tests/headless compile, and every plugin/face gate passed |
| `vivy-code` interactive PTY smoke | PASS at 80x24; `/thinking off` displayed `next turn thinking: off` in the shared overlay and Ctrl+C exited cleanly |

Tests cover command validation, shortcut cycling, unsupported and unknown
capability refusal, outbound RPC parameters, queue snapshot semantics,
unsupported-session downgrade, new-session capability loading, and REPL
capability revalidation.

The PTY smoke used the independent VIVY CODE private-instance path. No
production tenant Journal path was read or written.
