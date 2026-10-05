# Verification

## Completed

- Original `go list -m all` reproduced the missing `../laputa/garden/go.mod`.
- The unchanged Laputa evolution suite reproduced the missing `../../INOFY`
  replacement; canonical Laputa main now pins the published INOFY revision.
- Bootstrap tests: 6/6 passed, using real local Git repositories without network,
  including direct invocation from another directory without `-RepoRoot`.
- Actual `just setup`: passed both in the working checkout and an independent
  fresh clone with no sibling repositories.
- A separate external module downloaded and compiled the published canonical
  Garden agentapi without local replacements.
- `just ui-build`: passed after opting out of PowerShell telemetry for the
  verification process. No repository telemetry setting was introduced.
- UI typecheck, 70 test files / 527 tests, build and I18N checks: passed in
  `just ci`. The shared bootstrap tests still pass after Windows fixture fixes.
- Runtime and SDK external-consumer tests: passed with the Go toolchain on PATH.
- Bare-clone `just dev`: automatically cloned the pin, installed the frozen UI
  dependencies, compiled without `ui/dist`, and started the backend. Vite then
  failed because this Linux container denies `uv_interface_addresses`; Windows
  process cleanup is also unavailable here. This is not a full native-Windows
  startup acceptance claim.
- `go generate ./internal/generated/assembly`: passed; no output drift.
- `git diff --check`: passed.
- Full `just ci`: passed on `d5d62784` with the Go toolchain on PATH and
  `GOPROXY=https://proxy.golang.org,direct` for the verification process.
  This includes format, UI, vet, all Go tests, headless compile and plugin gates.
  SDK packaging completed in 403.708 seconds and conformance in 74.595 seconds.
- Windows CI run [37310455843](https://github.com/ProjectViVy/agent-vivy/actions/runs/37310455843):
  fresh-clone setup and independent cold dev both passed. The latter required
  backend `/healthz` and Vite `/` to return HTTP 200 without inheriting setup's
  sibling or prebuilt `ui/dist`. UI CI and full UI browser smoke also passed.

## Full gates

The Windows backend full-suite job is still running; its final outcome is not
yet claimed. Local full `just ci` and the targeted native startup checks passed.
An initial conformance-only run before creating the embedded UI failed on
missing `ui/dist`; the required UI build subsequently completed.

A separate full Go run hit a missing subprocess Go PATH and a goproxy.cn
checksum-service 502. The affected runtime and external-consumer packages passed
after supplying PATH and the official Go proxy for that verification process.
Overlapping full runs temporarily edited the same repository recipe and caused
the recipe-drift and reproducibility checks to fail. The duplicate run was
stopped, its temporary recipe change restored, and the final full gate passed
serially without implementation edits during sealing.

The first native Windows run exposed an empty `PSScriptRoot` in the bootstrap's
parameter default under Windows PowerShell 5. Root resolution now runs in the
script body, and the added default-entry regression plus the repeated native
setup/dev checks passed. The final local full gate includes that correction.

An initial CI run was blocked by automatic approval review for an external
Visual Studio telemetry request. Microsoft documents PowerShell's Application
Insights telemetry and its `POWERSHELL_TELEMETRY_OPTOUT` option. Subsequent local
PowerShell verification processes set that option before startup.

Laputa's regular module tests and vet passed. Garden's optional e2e suite has a
`TestGardenCleanBreakEndToEnd` 503 `memory_unavailable` failure reproduced on
unchanged Laputa main `6f2eed2`; it is outside this dependency/bootstrap fix.
