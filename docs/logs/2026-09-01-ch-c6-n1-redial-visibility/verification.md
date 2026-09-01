# Verification — CH-C6-N1

Environment: worktree `agent-vivy-vc0`, branch `feat/vc1a-bash-tool`.

| Command | Result |
| --- | --- |
| `go vet ./sdk/plugin/ ./internal/channelhost/` + vet in plugins/dingtalk, plugins/qq | ok |
| `go test ./internal/channelhost/ -count=1 -race` | ok (incl. new TestChannelEnvLoggerFace) |
| `go test . -count=1 -race` (plugins/qq) | ok 1.885s |
| `go test . -count=1 -race` (plugins/dingtalk) | ok 1.404s |
| new dingtalk tests `-count=3 -race` | ok |
| new qq tests `-count=3 -race` | ok |
| `gofmt -l sdk/plugin internal/channelhost plugins/dingtalk plugins/qq` | empty |
| `just ci` | exit 0 (main module fmt/vet/test, headless compile, plugin-ci 6 modules, UI tsc/eslint/vitest/vite build) |

## New tests

- `TestChannelEnvLoggerFace` (channelhost) — hostEnv implements
  `plugin.ChannelLogger`; the returned logger's output carries
  `channel=<name>` and reaches the Host's handler.
- `TestSuperviseRedialFailuresLogged` (dingtalk) — scripted stream: two
  failed redials warn (`failures` counter, `failed_attempts=2` on the
  recovery info line).
- `TestSuperviseSilentWithoutLogFace` (dingtalk) — env without the face
  keeps the loop silent and alive across failed redials (optionality
  contract).
- `TestRedialFailuresLogged` (qq) — drop the live attempt; session-stage
  warn, refused-dial warn, reconnect info with `failed_attempts=2`.
- `TestGiveUpLogged` (qq) — cannot-identify close (`errs.
  CodeConnCloseCantIdentify`) produces the terminal give-up error line.

## Fixes during the slice

- First qq `TestRedialFailuresLogged` run timed out on the refused-dial
  assertion: slog's TextHandler quotes the stage value, so the output is
  `stage="dial gateway"` and the plain-substring probe never matched.
  Assertion now matches on the error text (`err=refused`) with a comment
  explaining the quoting.
- First `-race` run caught the test's plain `bytes.Buffer` being written
  by the supervisor goroutine while the test polled it. Both new log
  buffers are now mutex-guarded (`logBuffer`) — a test-harness race, not
  product code.

## Gate

`just ci` exit 0.
