# CH-P0-1 verification

## Environment

- Linux worktree on `feat/channel-p0-1-contracts`.
- Go `1.26.4`, Node and pnpm available.
- `just` and PowerShell are unavailable. The repository `justfile` fixes its
  shell to `powershell.exe`, so literal `just ci` cannot run in this
  environment. The equivalent Linux commands were run individually; this
  limitation is not reported as a `just ci` pass.
- Literal invocation result: `just ci` -> `/bin/bash: just: command not found`.
- Go commands use `GOFLAGS=-buildvcs=false` because the isolated worktree
  environment cannot obtain VCS stamping status.

## Baseline identity

Baseline revision: `9e6db43c81e1d7ce249ee785f1b6efd91a09c5bb`.

Selected baseline Git object IDs captured before implementation:

| Path | Blob |
|---|---|
| `internal/app/channels.go` | `db6c9a5457c218ffbf11bc85f98aaa0e63affc79` |
| `internal/channelhost/host.go` | `4be9ae1c3db2dcd00f8e55a94df0a3252d754f76` |
| `internal/modules/defaults/constructors.go` | `502d9dd8e96cf9f01fc4120bf0d5eb147c38d84c` |
| `internal/rpc/control.go` | `49be6f6fdef7c50155d71b821d9623f9b58a04db` |
| `sdk/internal/assembly/compiler.go` | `20f729fac3bb1f8733084f06dca3734756b0c6b1` |
| `ui/src/components/settings/SettingsView.tsx` | `545fd233e07e6136134b6e598b25925b9794a5c9` |
| `ui/src/lib/api.ts` | `462c576056f94c20663582331c20457290c53dcb` |

These are baseline source objects, not packaged release-artifact digests.

## TDD evidence

| Contract | RED | GREEN |
|---|---|---|
| RPC contribution | compile failed with undefined `MethodBinding`, `Contribution`, and `ValidateMethodBindings` | `go test ./internal/rpc -count=1` passed |
| Channel composition | package reported no non-test Go files and missing contract symbols | `go test ./internal/channelcontract -count=1` passed |
| Channel Host selection | Provider-without-Host and duplicate-Host cases returned no diagnostics | focused and full Assembly suites passed after adding the catalog entry |
| UI projection backend | compile failed with undefined `capabilitiesResult` and `UIExtensionProjection` | focused RPC test passed |
| UI projection frontend | `pnpm typecheck` rejected missing `UIExtensionProjection` and `ui_extensions` | typecheck and UI test suite passed |

The settings preservation case passed against existing persistence behavior;
it freezes that behavior without changing production settings code.

## Commands and results

| Command | Result |
|---|---|
| `go test ./internal/channelcontract ./internal/rpc ./internal/app/settings ./internal/moduleport ./internal/modules/... ./sdk/internal/assembly -count=1` | PASS |
| `go test ./sdk/internal/assembly -run 'TestChannelHostContract|TestCompilePluginV1GraphFixtures' -count=1` | PASS |
| `go test ./internal/app/settings -run 'Test.*Channel.*(RoundTrip|Preserv)' -count=1` | PASS |
| `pnpm --dir ui typecheck` | PASS |
| `pnpm --dir ui test` | PASS, 38 files / 334 tests |
| `pnpm --dir ui build` | PASS; existing large-chunk warning only |
| I18N completeness and cross-face scripts | PASS; 1409 keys per locale, 13 shared semantic units |
| headless compile (`go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui`) | PASS |
| full repository `go vet` excluding the pre-existing `internal/workflow` carve-out | PASS |
| full repository `go test -timeout 20m` excluding that carve-out | First run exposed only the expected source-bound conformance digest mismatch described below; the complete post-refresh rerun passed, including `sdk/internal` (306.362s) and `sdk/internal/conformance` (76.354s) |
| all independent `plugins/*` and `faces/*` modules: `go vet ./... && go test ./...` | PASS |
| `git diff --check` | PASS |
| `go list -deps ./cmd/vivy` | PASS with 1101 packages; this is a baseline/full-build observation, not omission proof |

### Source-bound conformance refresh

The full test run executed the real Provider suites, then
`TestCheckedInProviderConformanceMatchesExecutedSuites` rejected the previous
checked-in digest because CH-P0-1 adds files under `internal/`. This is the
expected producer gate behavior, not a Provider failure. The live internal
source digest was recomputed through the repository command:

```text
go run ./sdk/internal/cmd/source-hash internal ""
f76ee77e1d0f1ad35fe147f3d945ae5494e1c2943854c74534022ba72f74fab6
```

The five internal-rooted entries in
`sdk/internal/assembly/conformance_results.json` were mechanically repinned
from `5e386f84...` to `f76ee77e...`. The independent producer gate was then
rerun and passed:

```text
go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1
ok agent-vivy/sdk/internal/conformance 66.310s
```

## Scope-fence evidence

The following checks pass:

- `recipes/web-no-channels.vivy.yml`,
  `recipes/web-channels-no-ui.vivy.yml`, and
  `recipes/web-telegram-only.vivy.yml` do not exist;
- no diff exists under `internal/generated/assembly`,
  `ui/src/generated/assembly.ts`, or `ui/src/components/settings`;
- `internal/app/app.go`, `internal/app/channels.go`, and
  `internal/channelhost/*` match the baseline; and
- neither `internal/app` nor a production `internal/modules/channel` imports
  or consumes `channelcontract`.

Consequently this change provides no physical omission evidence and makes no
claim that Channel is removable at runtime yet.
