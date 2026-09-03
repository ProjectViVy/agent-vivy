# Verification

Commands are run from the repository root unless noted.

| Command | Result |
|---|---|
| `go test ./sdk/tui/command ./sdk/tui/view ./internal/tui/... -count=1` | PASS |
| `cd faces/tui; go test ./... -count=1` | PASS |
| `go test -race ./sdk/tui/... ./internal/tui/...` | PASS |
| `cd faces/tui; go test -race ./...` | PASS |
| `go run ./sdk verify faces/tui` | PASS |
| `go run ./sdk pack --face tui --out .workspace/tui-slash-commands-pack-20260904-v1` | PASS; generation `gen_8d9f3476fb77aa01`, SHA-256 `d642b683194dada9b44422b49fdaa45032cd09355621479d9abebdef6f10c43a` |
| `just ci` | PASS after final result-rendering, draft-retention, and whitespace fixes; UI typecheck/197 tests/build, Go vet/tests/headless, every plugin/face passed |
| `vivy-code` interactive PTY smoke | PASS at 80×24: `/help` opened the local command overlay; `/not-real` opened a local error and retained its draft; Ctrl+C exited cleanly |

The PTY smoke used the independent `vivy-code` private-instance path. No
production tenant Journal paths were used by that smoke.

A GPT-5.6-LUNA MAX final review found that the legacy line REPL sent
`run/cancel` without consuming the terminal event, leaving its local busy
state wedged. `/cancel` now drains through the terminal event and a regression
test asserts that `busy` and `runID` clear. Race tests and `just ci` were rerun
after this fix.
