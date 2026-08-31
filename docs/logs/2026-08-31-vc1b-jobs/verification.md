# Verification — VC-1b background jobs

All commands run in the `agent-vivy-vc0` worktree on branch
`feat/vc1a-bash-tool` (VC-1b lands on the same feature branch stack,
under the VC-1a commit).

## Unit / integration suites

- `go test ./internal/tools/` — job registry: tail retention + gap
  accounting, RunUntil completion (no registration), timeout adoption
  (registered, running, incremental read returns empty on second read),
  launch/read/kill, run-context cancellation reaps jobs, unknown-id
  errors for Read/Kill, launch limit fail-closed at 16; bash tool
  background-flag propagation + ValidateArgs boolean typing. ok.
- `go test ./internal/runtime/ ./internal/config/ ./internal/rpc/ -count=1`
  — backend: background launch→output→kill, foreground timeout adoption,
  jobs die with run context; config/rpc suites stay green with the two
  new default-enabled tool names. ok.
- `gofmt -l internal/` — clean (fixed two files after the first run).
  `go vet ./internal/runtime/ ./internal/tools/` — clean.

## Service-level e2e (real path through the full stack)

- `TestServiceBashToolEndToEnd` (VC-1a, still green): safe foreground
  bash under `auto` approval policy runs without interrupt.
- `TestServiceBashToolBackgroundEndToEnd` (new): scripted model launches
  `bash {"command":"echo vivy_bg_e2e","run_in_background":true}`, polls
  `job_output {"job_id":"job_000001"}` three times, then closes. Asserted
  from the journal: run.completed; no approval interrupt anywhere in the
  flow (safe classification + readonly job_output auto-run); the
  deterministic job id and the echoed marker both reach the journal
  through job_output. ok.

## Full gate

- `just ci` — the two runs before the gate finally passed were each
  brought down by `TestCronAtJobDeletesAfterSuccessfulRun` (the
  pre-existing load-sensitive flake tracked as TFLAKE-CRON in
  `docs/TODO.md` §0.1, unrelated to this slice). After the test-budget
  mitigation landed as its own commit, the full gate passed: **exit 0**,
  all packages ok, UI build ok.

## Notes

- Browser smoke at :3015 not applicable: background jobs are a kernel
  tool capability with no dedicated UI surface. The service-level e2e is
  the real-path evidence (engine → approval gate → bash backend → real
  bash process → job registry → job_output tool → journal).
