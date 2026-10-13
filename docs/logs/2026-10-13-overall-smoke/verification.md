# Verification

Source: `65b561f6`. Windows amd64, Go `1.26.4`, Node `24.14.1`, pnpm
`10.33.0`, just `1.49.0`, and installed Google Chrome.

## Repository gate

Ran from the repository root, with Go added to the process PATH, `CI=true`,
and a unique disposable `VIVY_USER_HOME`:

```powershell
$env:PATH = 'C:\Program Files\Go\bin;' + $env:PATH
$env:CI = 'true'
$env:VIVY_USER_HOME = Join-Path $smokeRoot 'ci-home'
just ci
```

**FAIL, exit 1.** The main Go sweep finished with seven failing tests across
`internal/app` (2), `internal/bundles` (1), and `internal/runtime` (4).
`just` stopped the `test` recipe at its first Go command. The bounded command,
package-result, and failure excerpts are preserved in `ci-results.txt`.

Confirmed stages:

| Stage | Result |
|---|---|
| Pinned Laputa bootstrap | PASS, checked out `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24` |
| Bootstrap/source-closure tests | PASS, 11 tests |
| Go formatting | PASS |
| Frozen frontend dependency installation | PASS |
| Frontend typecheck | PASS |
| Frontend tests | PASS, 86 files / 639 tests |
| Frontend production build | PASS; bundle-size warning |
| Translation completeness | PASS, 1,569 keys / 163 placeholders per locale |
| Cross-face translation checks | PASS, 8 tests and 14 shared semantic units |
| `go vet ./...` | PASS |
| Main Go sweep | FAIL, 7 tests / 3 packages; every failure reproduced separately |
| SQLite package | PASS, 333.493s |
| SDK internals / Assembly / Conformance | PASS, 499.577s / 7.909s / 122.041s |
| SDK TUI and UI packages | PASS |

The frontend tests also print React test-environment `act` warnings and the
intentional error-boundary fixture exception; their test result is green.

## Focused reproductions and remaining gates

```text
go test ./internal/app -run '^(TestToolkitLifecycle|TestToolkitOwnerOmissionIsReflectedByApp)$' -count=1
go test ./internal/bundles -run '^TestGitAcquisitionHasNoExecution$' -count=1
just headless-compile plugin-ci diva-recall-test
```

- The first command failed both tests with the Eino version-anchor error.
- The second failed with `invalid_manifest (git_origin)`.
- A focused Runtime run selected the four Shell/Hook failures with `-count=1
  -timeout 2m` and reproduced all four (5.869s). Two Hook scripts use POSIX
  expressions while `runHookProcessScoped` selects `cmd /c` on Windows. The
  command fixtures require external `cat`/`echo`; PowerShell aliases are not
  executable utilities for Go. The original environment resolves `bash` to
  `C:\Windows\system32\bash.exe` and has no external `cat`/`echo` on PATH.
- With `C:\Program Files\Git\usr\bin` added only to the focused test process's
  PATH, the HereDoc and shell-prefix tests both passed (0.603s). No system PATH
  or source file was changed. This control isolates those two failures to
  missing utility prerequisites; it does not turn the original gate green.
- The combined remaining-gates command passed. Headless Vivy, VIVY CODE, and
  UI compilation passed; every independent Plugin/Face module's vet/test gate
  passed; all six SDK-generated DIVA recall tests executed and passed without
  skips.
- The broad `scripts/test-memory-loop.ps1` pass was not reached because the
  preceding Go sweep failed. It is distinct from the six-test recall gate;
  omitted `TestMemoryLoop*` tests are not counted as passed. No attempt was
  made to label the failed gate green by skipping its failures.

## Browser and executable smoke

The disposable driver started the prescribed split pair and a loopback mock
at `127.0.0.1:9911`. The model session selected embedded DeepSeek, with the
provider base overridden to this fixture. Requests used raw
`deepseek-flash`, with no gateway prefix. No live provider credentials or
endpoints were used.

Fourteen distinct core smoke checks passed across the main browser run and
focused reruns; see `core-smoke-results.json`. Actions, transcript RPCs,
terminal run status, and Journal events were checked in addition to
screenshots. The mock emitted streamed text, a real `write_file` tool call,
a hanging call for cancellation, and HTTP 503 for failure handling. The
approved tool run completed and its assistant continuation was persisted.
The physical `smoke.txt` contained `verified-smoke-file`, and the failed
provider run had durable `status=failed`; see `core-run-evidence.json`.
Forced process restart respected the 30-second organism lease before opening
the same disposable database; the completed transcript survived.

Also built disposable executables:

```text
go build -o <temp>/vivy.exe ./cmd/vivy
go build -tags vivy_headless -o <temp>/vivy-code.exe ./cmd/vivy-code
vivy-code.exe --help
vivy-code.exe --version
vivy-code.exe --no-tools -p SMOKE-CODE
```

All passed, and the print call returned the loopback model reply. Chrome also
loaded the embedded built UI at `127.0.0.1:8787` without uncaught exceptions.
This is compilation/execution of the default development body, not an SDK
sealed release or certification.

## Toolkit browser acceptance

Used the disposable compiled executable with an explicit
`VIVY_TOOLKITS_E2E_BINARY`, fresh test homes/configs, and the existing split
Playwright configuration:

```text
cd ui
pnpm exec playwright test --config playwright.toolkits.config.ts
pnpm exec playwright test --config playwright.toolkits-accessibility.config.ts
```

- The unmodified lifecycle test failed at line 160: the status locator
  resolved both the saved notice and loading status. Earlier preview,
  installation, enablement, and binding save succeeded on the fresh instance.
- An isolated copy of that test narrowed only `getByRole('status')` to the
  expected notice text. The full install/enable/Hook and MCP consent/binding
  save/update/fresh consent/disable/uninstall workflow passed. No repository
  test source was edited; this does not change the original suite's red result.
- The unmodified accessibility configuration passed both tests: keyboard
  entry on a narrow viewport and installed long content in an Escape-dismissible
  dialog with restored focus and no horizontal overflow.
- A fresh-instance regression driver then sent one ordinary successful chat,
  reloaded history, and entered Toolkits. Local source preview failed. Calling
  `session/get` on the same UI transport and retrying admitted that same source.
  The regression intentionally remains red; see the before/after JSON,
  screenshot, and bounded action-audit excerpt.

## Driver corrections and evidence limits

Initial temporary driver failures were investigated before attributing them
to the product: Persona requires initialization; composer command tags need
selection then execution; `turn/start` accepts `text`; `approval/respond`
accepts `approved`; `approval/list` already filters pending rows and exposes
`decision`, not `status`; session creation must update the active UI session;
some assembled pages contain nested `main` elements; and `--no-session` is
explicitly rejected because every Vivy run is journaled. Corrected checks were
rerun. None of these driver mistakes is counted as a product regression.

Persona initialization used the normal authenticated selected-session
control plane and the actual five-field browser form. Early cold-route
Persona/Mask denials were observed during driver setup; their exact timing
matrix was not isolated, so this report does not claim a separate confirmed
Persona bug. The completed-chat Toolkit reproduction was isolated independently.

Eino capability check was diagnostic only: the existing pinned Eino
`adk.NewChatModelAgent`, `adk.NewRunner`, and EinoExt OpenAI
`NewChatModel` remain the exercised runtime path. The fixture-anchor issue
uses Vivy's existing `EinoEngineVersion`/`SetEngineVersionOverride` contract;
no custom orchestration, upstream replacement, or runtime capability was added.

Studio, live providers/channels, PostgreSQL server integration, full workflow
or multi-agent acceptance, and release conformance were skipped because this
iteration is an isolated source-update smoke. They are not counted as passed.

All test servers were stopped and temporary drivers were removed.
The production Journal locations and pre-existing `.omc/` and `.omo/` were
left untouched. Final cleanup and diff checks are recorded below.

## Delivery checks

- Generated UI staging/routing writes had no semantic diff and were restored
  to the tested commit after the test processes finished.
- `git diff --check` passed for the documentation-only delivery.
- The report and four open-board rows are the only intended repository changes.
- Temporary driver files were removed individually and their empty workspace
  directories removed. Generated UI files were restored. The ordinary
  dependency caches, pinned Laputa checkout, and frontend build output remain
  available.
- Automatic approval review rejected recursive removal of the isolated test
  root with `blocked by policy`; no approval bypass was attempted. The test
  executable/config/database directory remains at
  `C:\Users\Administrator\AppData\Local\Temp\2\vivy-overall-smoke-72c72daeb4b54f1e9d35394df24229c9`.
  The four Playwright-created `vivy-toolkits-e2e-runtime-*` directories from
  this run also remain. They are disposable test data, outside production
  Journal locations. All test servers are stopped.
