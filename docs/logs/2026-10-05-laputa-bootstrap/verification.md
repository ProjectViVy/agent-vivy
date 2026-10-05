# Verification

## Completed

- Original `go list -m all` reproduced the missing `../laputa/garden/go.mod`.
- The unchanged Laputa evolution suite reproduced the missing `../../INOFY`
  replacement; canonical Laputa main now pins the published INOFY revision.
- Bootstrap tests: 5/5 passed, using real local Git repositories without network.
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

## Full gates

Full `just ci` is running its SDK packaging tests; completion is not yet claimed.
The independent Windows startup smoke is pending GitHub Actions. An initial
conformance-only run before creating the embedded UI
failed on missing `ui/dist`; the required UI build has since completed.

A separate full Go run hit a missing subprocess Go PATH and a goproxy.cn
checksum-service 502. The affected runtime and external-consumer packages passed
after supplying PATH and the official Go proxy for that verification process.
That duplicate full run was stopped because existing SDK tests temporarily edit
a repository recipe; the authoritative `just ci` run proceeds on its own.

An initial CI run was blocked by automatic approval review for an external
Visual Studio telemetry request. Microsoft documents PowerShell's Application
Insights telemetry and its `POWERSHELL_TELEMETRY_OPTOUT` option. Subsequent local
PowerShell verification processes set that option before startup.

Laputa's regular module tests and vet passed. Garden's optional e2e suite has a
`TestGardenCleanBreakEndToEnd` 503 `memory_unavailable` failure reproduced on
unchanged Laputa main `6f2eed2`; it is outside this dependency/bootstrap fix.
