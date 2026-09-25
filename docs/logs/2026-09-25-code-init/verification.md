# Verification

| Check | Result |
| --- | --- |
| New failing tests before implementation | Command registration, view dispatch, RPC status, and post-startup instruction discovery failed as expected. |
| `go test -timeout 20m -tags vivy_headless ./sdk/tui/... ./internal/rpc ./internal/runtime ./internal/codeface ./cmd/vivy-code` | Passed, including normal and Plan `/init` turns, fail-closed preflight, and next-turn instruction injection. |
| `go vet -tags vivy_headless ./...` | Passed. |
| `go build -tags vivy_headless -o ../vivy-code-smoke ./cmd/vivy-code` | Passed. A real terminal launch in a temporary project showed `/init` in `/help`; entering `/init` submitted the project-analysis turn through the code face. The model then reported a provider connection error because this environment has no configured provider, so live file creation could not be observed. |
| `go run ./sdk/internal/cmd/source-hash internal ''` | Computed `e47169eccb7062a67856d74f25b32ee9a4031ce61007a965e49a216f0565cc1e` after the final RPC test; refreshed the five internal source identities in `sdk/internal/assembly/conformance_results.json`. |
| `go run ./sdk/internal/cmd/generate-default --repo . --output internal/generated/assembly/zz_default.go` | Passed; generated code has a formatting-only change under Go 1.26.4. |
| `git diff --check` and Go 1.26.4 `gofmt -l` on changed Go files | Passed. |

`just ci` could not run in this Linux environment: the repository's `justfile` requires `powershell.exe`, which is unavailable. Its UI dependency step also could not complete: `pnpm install --frozen-lockfile --offline` reports a missing locked `@radix-ui/react-accordion` tarball, and the configured registry is unreachable here. An attempted full `go test -timeout 20m -tags vivy_headless ./...` passed the affected packages but failed in UI-dependent eval, Studio, and SDK assembly/conformance tests because `ui/dist` and the locked UI packages are absent. Re-run `just ci` in the repository's normal environment before release.
