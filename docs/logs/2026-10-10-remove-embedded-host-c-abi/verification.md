# Verification

## Passed

- `just fmt-check` — passed after the change; includes current tracked and
  untracked Go files and ignores removed paths.
- `git diff --check` — passed.
- `go test ./sdk/generation` — passed.
- `node --test scripts/ensure-laputa.test.mjs` — 6 passed, 0 failed.
- Active source/config/documentation scan for C ABI exports, Embedded Host
  packages, `go-host`/`shared` targets, `HostBuild`, and FFI harness paths — no
  matches outside the task plan and historical logs.

## Blocked

- `just ci` passed `ensure-laputa`, all six bootstrap tests, `fmt-check`, and
  the frozen pnpm install. `ui-core` then failed while compiling the locked
  Laputa revision `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`:
  `garden/internal/ingest/capture_activity.go` refers to missing
  `Service.Actmem` and `Service.activityMu` fields. `vet`, the Go test suite,
  headless compile, and plugin CI were not reached.
- `go test ./sdk/internal/assembly ./sdk/generation` and the focused App test
  could not start because `ui/embed.go` requires `ui/dist`, which the failed
  UI build did not produce.
- The focused `sdk/internal` test build reached the same pinned Laputa compile
  error, before running tests.

The blocker is in the locked sibling dependency and is outside this branch's
diff. The application and SDK lifecycle tests therefore remain unexecuted in
this environment.
