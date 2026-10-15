# S6 CI preparation: nested BML and native Windows/browser gates

This checkpoint wires required gates before final S5/S6 candidate acceptance. It
does not accept the unfinished consumer integration, Windows execution, installed
product, real-model quality or rendered browser experience. Working source was
based on Vivy `6e4096e8444b89713a2857b061e63fa1c2e4be01`, with the unchanged
published Laputa source lock `249a756e04c5f28af781e4dadea9e9086689bde0`.

`just bml-ci` enters the independently declared `bml` module and runs the selected
Go toolchain's `vet -p=1 ./...` followed by `test -p=1 -count=1 -timeout=5m ./...`.
Each nonzero command propagates failure. Both `ci` and `backend-ci` require this
task after their existing gates. All earlier recipes, their order, the default
35-minute package timeout, separate MemoryLoop/DIVA scripts, headless compile and
plugin gates remain unchanged. Race remains a separately qualified check.

The independent required `native` Windows job has a 20-minute job bound and uses
the existing immutable checkout/setup-Go actions, declared Go version and normal
Laputa bootstrap. It executes full native BML tests from `bml` (declared SQLite
1.56.0), ACTMEM tests from the pinned native Laputa module, and Garden ingest,
runtimecore and agentapi tests from that same native checkout. Each lane runs
`go test -json -p=1 -count=1 -timeout=5m` with no test-name narrowing or
cross-compilation. Evidence records Vivy HEAD, lock and actual native HEAD,
Go/host OS and architecture, declared module JSON, commands, exit codes, elapsed
times, pass/fail/skip counts, original JSONL and separate stderr. Failure in any
lane fails the job; the always-running aggregate requires `native` success along
with backend, UI and browser success.

The browser job completes its old full-UI test and waits for that selected
process to stop before invoking
`pnpm --dir ui exec playwright test --config playwright.issue58.config.ts` with
the actual installed `GO_EXE`. The existing disposable real App/owner fixture
and Vite pair retain exclusive 8787/3015 ownership and no existing-server reuse.
Recap assertions and fixtures are unchanged. The dedicated config now writes to
`ui/test-results/issue58-recap`, captures screenshots on success and failure, and
retains traces on failure. A missing/failed new test cannot make the browser job
successful.

Always-running artifact uploads preserve only native test evidence and browser
test results, screenshots and traces, using official
`actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9`
(v7.0.2, node24; upstream tag/action metadata was verified in the preceding CI
preparation). Missing outputs warn, for example after setup failure, without
changing the failed gate. No application data roots or credentials are uploaded;
workflow permissions remain `contents: read`.

## Author checks and limits

All executed checks used the unchanged credential-stripping, offline-dependency,
non-forwarding deny-proxy runner. Command metadata reports zero external requests
rejected/forwarded. Raw attempts and a hash manifest are retained in the S6 CI
wiring evidence alongside the implementation report.

| Check | Result |
|---|---|
| Actual `just bml-ci`, Go 1.26.4 on Linux | PASS: vet and uncached full native BML package tests; 1 package, per-test counts not emitted by this recipe |
| YAML/configuration structure | PASS: four required lanes, native commands, unchanged earlier Justfile recipes, shutdown ordering and immutable artifact pin |
| Installed PowerShell parser | PASS: all 17 workflow PowerShell run blocks, including new native and browser blocks |
| Dedicated Playwright `--list` | PASS: 1 existing Recap test collected; collection is not execution |
| Strict TypeScript check of config/spec/teardown | PASS |
| `git diff --check` | PASS |
| Installed `just --fmt --check`, before edits | FAIL: existing interpolation spacing differs from the formatter; preserved rather than reformatted unrelated recipes |
| Full local `just ci` | NOT RUN by the bounded preparatory assignment; final S5/S6 source is unfinished |
| Actual native Windows job / rendered Recap browser | NOT RUN locally; required supported browser is unavailable, and configuration/parse checks do not establish either result |

No runtime, App, BML implementation, adapter, dependency, generated Assembly,
Recipe, native Laputa source, assertion or fixture changed. No PostgreSQL DSN,
live provider, installed #66 producer, deployment or tenant activation is claimed.
Root owns fixed-candidate checks, publication, actual CI observation and later
whole-candidate acceptance. This delivery leaves the existing product acceptance
and unavailable S5 integration states explicit.

## Root fixed-candidate checkpoint

Published source `2e6273d8b4a6a58d5154633b1805137a633a59b9` maps to remote
`f1d651622bf91b6aabdfc408964850eea3980ea8`, exact tree
`89dbbe29a3fd08f7d116ff44b7e0a7c8c11e4093`. Root verified all four source
hashes and all 37 author raw artifacts. Local and remote commit metadata differ;
the published tree was independently read back and verified.

Root executed the following on the actual clean detached source checkpoint,
with a real .git directory and the unchanged local-only runner:

| Check | Fixed-candidate result | Seconds |
|---|---|---|
| Config/earlier recipe preservation | PASS | 0.114 |
| Actual `just bml-ci` | PASS, vet + uncached full module tests | 2.641 |
| Dedicated Playwright `--list` | PASS, one test collected only | 5.207 |
| Strict config/spec/teardown TypeScript | PASS | 5.458 |
| Workflow PowerShell parser | PASS, all 17 blocks | 0.365 |

All five executions returned zero and recorded zero rejected/forwarded external
requests. The root config validator uses the explicit previous Justfile rather
than the now-fixed HEAD as its preservation baseline. Its source, extracted
PowerShell, original logs and metadata are retained separately from author logs.

Author archive: `evidence/s6-ci-wiring-author.tar.gz`, SHA256
`6386960a61b258cf4e26c354a80e11bf8551cfa0af0bacdf617aa4abc5a3b1e9`.
Root fixed archive: `evidence/s6-ci-wiring-root-fixed.tar.gz`, SHA256
`d225cc4189e9fd044722e1af3b884b03c2fa5455c801d6b593c761e08c108baf`
(29 hashed raw/helper artifacts). Original baseline formatter failures remain
inside the author archive.

Actual Windows run 38824982363 was pending at publication. Native Windows, new
rendered Recap E2E, screenshot review and full final-candidate `just ci` remain
PENDING/NOT RUN, never inferred from collection or parser checks. S5 consumer
source remains unfinished. Later source edits require a new fixed candidate.
