# Verification

All commands were run from the repository root on Windows. Explicit Go binary paths were supplied to `just` because this shell did not have `go` on `PATH`.

## Conflict and regression checks

- `go test ./sdk/internal/assembly ./internal/app -run 'TestP3ObserverAndStatusHostsAreSelectableCoreOwners|TestDefaultGenerationComposesEstablishedOptionalHosts' -count=1 -timeout 5m`
  - The RED run proved both integration gaps: compiler core-owner validation rejected the two Hosts, and the default generated Manifest omitted them.
  - Passed after adding the build-owned core owners, selecting both Hosts in the default Recipe, and regenerating the Assembly.
- `go run ./sdk/internal/cmd/generate-default -repo . -output internal/generated/assembly/zz_default.go`
  - Passed and regenerated the default Assembly from the reconciled catalog and compiler.
- `go test ./sdk/internal/assembly ./internal/modules/defaults ./internal/modelhost -count=1 -timeout 5m`
  - The first run exposed a P4 assertion coupled to `gofmt` spacing after the P5 Manifest field was added.
  - Passed after the assertion was made whitespace-insensitive while retaining the same semantic expectations.
- `go test ./internal/runtime -run '^TestCronTriggerManualConflictAndDisabledJob$' -count=50 -timeout 5m`
  - Passed 50 consecutive runs after waiting explicitly for the asynchronous active-run cleanup condition.

## Product gate

- `just --set go 'C:\Program Files\Go\bin\go.exe' --set gofmt 'C:\Program Files\Go\bin\gofmt.exe' ci`
  - The first post-merge run reached every package but failed `TestCronTriggerManualConflictAndDisabledJob`: the durable job row became observable before the asynchronous active-run entry was removed.
  - The final run passed with exit code 0.
  - UI typecheck, 31 files / 281 tests, production build, i18n completeness, and cross-face conformance passed.
  - Go format, vet, all packages, headless compile, and every independent plugin/face gate passed.
  - The final fresh rerun completed with exit code 0; `sdk/internal` took 81.339 seconds, while the already-green app, runtime, and Assembly packages reused their cache entries.
  - Vite emitted its existing large-chunk advisory; it was not a gate failure.

## Real-path smoke

- Default and minimal Generations were packed and inspected with `go run ./sdk pack` and `go run ./sdk inspect-artifact`.
  - Post-review default generation: `3d1e895cba7759813f2a00b7ad732709081748530f45fad5adc87f85c6bf592d`.
  - Post-review minimal generation: `5b441befdbd9a5411e0cec3ed28304f5b2c036305d4b04b6bbd845f5edfea993`.
  - The default artifact contained `vivy/provider-profiles`, `vivy/context-source`, `vivy/skill-source`, `vivy/mcp-host`, `vivy/observer-host`, and `vivy/status-host`; the minimal artifact omitted all six.
  - Both temporary pack directories were removed after inspection.
- The backend ran with an explicit `.workspace/smoke-pr19/config.yaml` whose SQLite, logs, workspace, and skills paths were all isolated under `.workspace/smoke-pr19`; Vite served the split UI at `http://127.0.0.1:3015`.
- An installed Chrome browser opened `/settings?tab=model` through Playwright. The page returned success, rendered meaningful content, showed OpenAI and Anthropic, had no Vite error overlay, and produced no application or RPC errors.
- The browser requested the absent `/favicon.ico`, producing the existing isolated 404 console message. No application response failed.
- Both smoke servers were stopped, ports 8787 and 3015 were confirmed clear, and the isolated SQLite/log/browser scratch artifacts were removed.

## Air-gap incident

Before the isolated config was created, the diagnostic command `go run ./cmd/vivy --help` unexpectedly started the server because the binary does not handle `--help` as a terminal CLI action. It loaded the checkout's default configuration and may have opened or modified its configured tenant SQLite/log/settings paths. The exact `vivy.exe --help` process was identified and stopped promptly; no tenant data was inspected, deleted, or cleaned up. The unfixed CLI behavior is recorded as `CLI-HELP-EXIT` in `docs/TODO.md`.
