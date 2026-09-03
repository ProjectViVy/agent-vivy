# Verification

## Automated

- `go test ./internal/runtime -run '^(TestBashBackend|TestServiceBashToolBackgroundEndToEnd|TestLocalWorkspaceManagerMountsProjectRoot)' -timeout 2m -count=1` — passed.
- `go test ./internal/tui/... ./internal/config -count=1` — passed after correcting mutation-result projection.
- `cd faces/tui; go test ./... -count=1` — passed.
- `go test ./internal/tools -count=1` — passed; Windows uses the in-process cancellable job seam while Unix retains real-bash coverage.
- `just ci` with `GOFLAGS=-p=2` — passed. The gate included gofmt, UI typecheck, 24 UI files / 197 tests, UI production build, Go vet, all root Go tests, headless build-tag compile checks, and every plugin/face module.

The Windows test environment used dedicated `TEMP`, `TMP`, and `GOCACHE` directories under `C:\tmp`; no tenant Journal paths were used.

The first unconstrained gate run exposed a full-load timing failure in an unrelated headless approval test; its isolated rerun passed. Package concurrency was then limited to two for the successful full gate. A separate first gate attempt also exposed that old job-registry tests invoked the Windows WSL placeholder as `bash`; the tests now exercise the portable in-process seam on Windows.

## Real path smoke

- Built `.workspace/tui-smoke/vivy.exe` and launched `vivy tui` in a visible PowerShell terminal from the project root.
- The process remained live and acquired only `.workspace/tui-smoke/home/vivy.db`, proving that the interactive face composed the real kernel with an isolated Journal rather than the demo store.

## Scope audit

- `git status` / diff audit confirms no `studio/` path is modified.
