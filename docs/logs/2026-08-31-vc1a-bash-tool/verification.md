# Verification — VC-1a bash tool

All commands run in the `agent-vivy-vc0` worktree on branch
`feat/vc1a-bash-tool`.

## Unit / integration suites

- `go test ./internal/tools/` — bashclass tier table (~30 cases: safe
  pipelines, git/go subcommand tiers, redirect writes, /dev/null
  exemption, expansions, fork bomb, piped curl|sh, root rm denied,
  workspace rm mutating-only), syntax-error rejection, tool-level
  expand/deny/proposal/classify tests, ValidateArgsSafety bash
  exemption + unconditional NUL check. ok.
- `go test ./internal/runtime/ -run 'TestBash|TestToolAdapterTiered|TestServiceBashToolEndToEnd'`
  — real bash execution outside the executable allowlist, read-only
  sandbox denial, deny-table defense-in-depth, malformed invocation
  (missing shell, non-`-c` args), tiered-approval adapter matrix
  (safe+auto executes / mutating+auto interrupts / safe+ask interrupts /
  safe+never denied / denied+full-auto denied / whitelisted unchanged),
  and the service-level e2e below. ok.
- `go test ./internal/config/ ./internal/rpc/` — default enabled surface
  includes bash when `tools.enabled` omitted. ok.
- `gofmt -l internal/` — clean. `go vet ./internal/runtime/ ./internal/tools/`
  — clean.

## Service-level e2e (real path through the full stack)

`TestServiceBashToolEndToEnd` wires engine + approval adapter + real
`bash` backend over sqlite, pins the session to approval policy `auto`,
and scripts the model to call `bash {"command":"echo vivy_bash_e2e"}`.
Asserted from the journal:

- run reaches `run.completed`;
- no `tool.approval_required` event (safe classification auto-ran);
- governance `policy.evaluated` with reason "safe read-only invocation
  auto-approved" between `tool.requested` and execution;
- `tool.finished` carries the echoed marker `vivy_bash_e2e`.

## Full gate

- `just ci` — first run failed on `TestCronAtJobDeletesAfterSuccessfulRun`
  (the pre-existing flake tracked as TFLAKE-CRON in `docs/TODO.md` §0.1;
  passes twice in isolation immediately after). Second full run: **exit 0**,
  all packages ok, UI build ok.

## Notes

- Browser smoke at :3015 is not applicable to this slice: bash is a
  kernel tool with no UI surface of its own. The service-level e2e above
  is the real-path evidence (engine → adapter gate → backend → real
  bash process), matching the iteration rule that unit tests alone do
  not close a user-visible change.
