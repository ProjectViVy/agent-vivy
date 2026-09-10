# Verification — Command output capture race

- RED: an uncached full `go test ./...` returned exit 0 with empty stdout from
  `go version` in `TestCommandBackendRunsInsideWorkspaceAndBuildsProposal`.
- RED: after fixing the direct path, another uncached full run returned exit 0
  with empty stdout from Bash in `TestBashBackendRunsShellOutsideAllowlist`,
  exposing the same pipe/Wait ordering in `JobRegistry`.
- `go test -count=20 ./internal/runtime -run
  '^(TestCommandBackendRunsInsideWorkspaceAndBuildsProposal|TestBashBackendRunsShellOutsideAllowlist)$'`:
  passed.
- `go test -count=10 ./internal/tools -run 'Job|Command'`: passed.
- `gofmt` and `git diff --check`: passed.

The workspace has no `just` or PowerShell executable. The broader P1/P2
checkpoint ran every `just ci` constituent directly before this focused fix;
the final focused rerun completed with:

- `go vet ./...`: passed.
- `go test -count=1 ./...`: passed, including Runtime, MCP replacement, SDK,
  Assembly, storage, and Studio packages.
- `go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui`:
  passed.
