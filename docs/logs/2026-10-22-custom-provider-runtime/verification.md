# Verification — custom provider runtime

## Completed

- RED: `go test ./internal/app -run '^TestResolveStoredSelectionChain$' -count=1` failed on the registered custom-provider case because it was reported unusable.
- RED: `go test ./internal/provider -run '^TestUserDefinedProviderRoutesThroughSealedAdapter$' -count=1` failed with `no embedded vendor data` before the request reached the test server.
- GREEN: `go test ./internal/provider ./internal/app -count=1` passed. Provider package: 0.461s; app package: 83.555s.
- `go test ./internal/provider -run '^TestUserDefinedProviderWithoutKeyFailsBeforeGateway$' -count=1` passed; the gateway received no request without a key.
- Final provider package rerun (including the `/v1` gateway path): `go test ./internal/provider -count=1` passed in 0.486s.
- `go vet ./internal/provider ./internal/app` passed with exit code 0.
- `gofmt` ran on the changed Go files; `git diff --check` passed.

These Go commands used the installed Windows Go toolchain at `C:\Program Files\Go\bin\go.exe`, because the Debian WSL environment has no Go executable on PATH. No toolchain was installed.

## Not run

- `just ci` and the Linux/WSL scoped gates were not run; this focused change touches only App provider selection and the existing provider adapter, and the Linux Go toolchain is unavailable in the current WSL environment.
- At source delivery time the manual backend still used the pre-fix binary and replacement was blocked. The later [live readiness follow-up](../2026-10-22-d14-provider-live/verification.md) records an approved native pack/Inspect/restart at this exact candidate and a real Tiny UI attempt. That attempt now reaches credential resolution and reports `API key missing`; credentialed browser success and human acceptance remain pending.
