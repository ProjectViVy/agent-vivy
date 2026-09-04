# Verification

## Automated

- `go test ./sdk/tui/view ./internal/tui -count=1` — PASS.
- `cd faces/tui; go test ./... -count=1` — PASS.
- `go test -race ./sdk/tui/view ./internal/tui -count=1` — PASS.
- `cd faces/tui; go test -race ./... -count=1` — PASS.
- `git diff --check` — PASS.

The focused tests cover terminal refresh dispatch, late/stale context rejection,
cross-session and out-of-order permission rejection, initial-subscription
failure cancellation plus refresh, failed-terminal queue preservation,
active-session queue counts, context-error stale hiding, prompt boot session
retention, and rendering of only the supplied sidebar/meta facts.

## Product gate and smoke

- First `just ci` — all changed TUI packages and every other slice passed except
  the already tracked `TestCronAtJobDeletesAfterSuccessfulRun` load-sensitive
  canary (`TFLAKE-CRON-DELETE-RECURRENCE`): the at-job was disabled but its
  asynchronous deletion did not settle inside 30 seconds.
- `go test ./internal/runtime -run '^TestCronAtJobDeletesAfterSuccessfulRun$' -count=3`
  — PASS (3/3, 4.184s), confirming the existing timing flake rather than a TUI
  regression.
- Final full `just ci` after the asynchronous fence review and fixes — PASS,
  including UI typecheck/201 tests/build, Go vet,
  all main-module tests, headless build checks, every plugin module, and the
  packed TUI face.
- `just vivy-code` — PASS.
- Real Windows PTY (`cmd.exe`, 80 columns): launched `vivy-code.exe`, observed
  the fullscreen VIVY CODE shell, entered `/help`, observed the real command
  overlay and registered advanced commands, then exited with `Esc`, `Ctrl+C`;
  process exit code 0.

## Review

- Three GPT-5.6-LUNA MAX specialists audited sidebar state, command-surface
  parity, and stream protocol gaps. Sidebar review initially found permission
  response pollution, failed-terminal dequeue, subscription-cancel freshness,
  stale context presentation, and cross-session queue-count issues. All were
  fixed and covered in both drivers; the final sidebar re-review reported no
  blocking findings.
- The protocol review kept `TUI-STREAM-N4/N6` open with a concrete versioned
  delta-source and idempotent Journal-to-Message projection design; no unsafe
  synthetic completion was added to this delivery.
