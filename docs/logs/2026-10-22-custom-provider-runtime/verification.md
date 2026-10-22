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
- Browser smoke at `http://127.0.0.1:3015` was not run against the candidate code. The isolated backend serving the manual page is still the pre-fix binary; its replacement/restart was blocked by execution policy. The HTTP-path regression test exercises the same Eino adapter against a deterministic local server.
