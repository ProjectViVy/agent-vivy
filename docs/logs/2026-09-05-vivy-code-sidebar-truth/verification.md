# Verification

## Automated

- `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/rpc ./internal/runtime ./sdk/tui/view ./internal/tui` — PASS.
- `cd faces/tui; go test ./...` — PASS.
- `go test ./internal/storage/sqlite ./internal/storage/postgres` after missing-session touch hardening — PASS.

## Product gate

- Final code-stable `just ci` — PASS: formatting, UI typecheck, 201 UI tests,
  production UI build, Go vet/full tests, headless compilation, and every
  plugin/face vet+test slice passed.
- `just vivy-code` and `vivy-code.exe --help` — PASS; the independent
  headless-tagged VIVY CODE executable built and identified its private-instance
  product contract.

## Real path

- A native Windows PTY launch was attempted with an isolated
  `VIVY_USER_HOME=.workspace/sidebar-smoke-home`, but this execution host failed
  before starting the process with its existing `CreateProcessW` OS error
  `-1073283067`. No interactive pass is claimed. The keyboard/resize/sidebar
  paths were instead exercised through the real Bubble Tea update loop tests;
  executable construction and non-interactive startup (`--help`) passed.

## Review

- Three GPT-5.6-LUNA MAX specialists audited UI/Crush parity, storage/migration
  correctness, and RPC/performance contracts.
- Review findings fixed before the final gate include: swallowed editor input,
  missing built-in/subscribe-failure refreshes, stale mutation responses,
  startup/load error visibility, old-server capability fallback, fork/edit/
  rewind activity gaps, non-atomic projected-message activity, incomplete
  pricing marked known, full-store usage scans, and multi-gigabyte file-version
  reads.
- Storage re-review reported no remaining P1/P2. The final implementation
  filters usage at the database boundary and aggregates it while streaming into
  at most 256 route groups plus one conservative unknown overflow group;
  modified files are capped at 20 paths and only oldest/newest snapshots are
  read.
