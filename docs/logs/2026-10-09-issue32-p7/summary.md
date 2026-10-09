# P7 integration summary

## Scope

P7 binds the issue #32 fixes to the final VIVY dependency closure, executes the
aggregate producer gates, exercises the split UI against a real local backend,
and pins DIVA to the resulting VIVY and Laputa sources. Publication and owner
acceptance remain separate gates.

## Integrated source findings

- P2.3 consumes Laputa's atomic `ApplyAtMissionRevision` seam. The locked
  planning baseline `ff3936f44ff8cf08c12af2cf698c194cfe474fd3` does not expose
  that API, so this package adds Laputa issue32 commit
  `30fa208e3cded4af43f8cb226d80b225b1910854` as the exact dependency closure.
- VIVY `go.mod` garden/laputa/mentle pseudo-versions and
  `laputa-source.lock.json` now match `30fa208e3cded4af43f8cb226d80b225b1910854`.
- Cancellation regression assertions now accept the two truthful settled
  outcomes: `cancelled` with stable duplicate-operation replay, or
  `recovery_required` with an active native Run and duplicate-start fencing.
- The checked-in internal source digest is
  `d61f174459bdf3157f2a29ea857ca58474360ec7f1eee7cc29f331d395e581e7`; the
  workflow UI module digest is
  `e98c354a4d2522cd3b99290553303d89badd1731d1fc2217a1a51c1922e6c7d2`.
  Repeated source-hash checks were stable.

## Engineering evidence

- Full VIVY `go test -timeout 35m ./...` passed, including SQLite and configured
  PostgreSQL suites, SDK pack/Inspect, and source-bound conformance.
- `go vet ./...`, headless command/UI compilation, UI TypeScript checking,
  80-file/620-test UI suite, production Vite build, root i18n checks, and the
  translated `plugin-ci` Go vet/test gate for all `plugins/` and `faces/`
  modules passed.
- The focused conformance replay
  `go test ./sdk/internal/conformance -run
  '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1
  -timeout=35m` passed.
- Split-loop browser smoke used the real VIVY backend on `127.0.0.1:8787` and
  Vite on `127.0.0.1:3015`, with an isolated SQLite/workspace under
  `/workspace/work/issue32/p7-smoke`. Through the product UI it initialized a
  temporary Persona, created a new session, authored/saved/validated/published
  revision 1, started a local mock-backed parent Run, started the published
  workflow while the parent was active, then opened the Run detail. The detail
  showed `completed`, `Engine: succeeded`, a completed node, and the committed
  `run.started` through `run_succeeded` events. Browser page errors: none.
  Screenshot: `/workspace/work/issue32/p7-smoke/run-inspect.png`.
- The mock provider listened only on `127.0.0.1:11434`; it delayed the parent
  response long enough for the workflow admission click and returned a fixed
  local response for the child. The Persona and all app files stayed under the
  isolated scratch data directory. This proves the split UI start/inspect path
  with a deterministic local provider, not production-provider behavior.
- The UI master Run list is a point-in-time page; after the terminal event its
  selected detail refreshed to completed/succeeded while the list row could
  still show its earlier active/running snapshot until Refresh. The committed
  journal detail and the VIVY status projection are authoritative; automated
  lifecycle tests cover status separation, cancellation, retry identity,
  pagination and recovery behavior.

## Environment limitation

`just ci` is not directly executable in this Linux image because the checked-in
VIVY `justfile` invokes `powershell.exe`, which is absent. Every Linux-executable
recipe component was run directly, including the source-closure checks and all
plugin/face Go modules. The six `scripts/ensure-laputa.test.mjs` cases have the
same PowerShell dependency; equivalent locked-commit, clean-tree and module
identity checks passed manually. This is a wrapper limitation, not a successful
literal `just ci` invocation.

GTK3/WebKit4.1, Wails CLI/runtime, and a Windows runner are unavailable here.
Those prevent native DIVA candidate builds and installed-product acceptance;
they do not invalidate the Linux headless producer/consumer gates.
