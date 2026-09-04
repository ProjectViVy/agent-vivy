# Verification

## Automated

- `go test ./sdk/tui/view ./internal/runtime ./internal/tui` — passed.
- `cd faces/tui; go test ./...` — passed.
- `just ci` — passed: formatting, UI typecheck, 201 UI tests, production build, Go vet/full tests, headless compile, and every plugin/face module.
- `just vivy-code` — passed.
- `.\\vivy-code.exe --help` — passed.

Coverage includes known and unknown model limits, estimated context percentages, the over-80% warning, full session usage rendering, known zero cost versus unknown cost, and built-in/packed DTO parity.

## Real path

An isolated interactive launch was attempted with `VIVY_USER_HOME` under `.workspace/token-cost-smoke-home`. The PTY runner failed before process creation with Windows `CreateProcessW` OS error `-1073283067` (`FormatMessageW` error 317), matching the prior environment failure. No product runtime or tenant Journal was opened, so this is recorded as an environment limitation and not counted as a successful interactive smoke.

The built executable was moved to ignored `.workspace/build-artifacts/vivy-code.exe`; no build artifact is part of the delivery commit.
