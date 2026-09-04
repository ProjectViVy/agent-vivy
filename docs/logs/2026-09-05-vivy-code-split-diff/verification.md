# Verification

## Automated

- `go test ./sdk/tui/stream ./sdk/tui/view ./internal/tui` — passed.
- `cd faces/tui; go test ./...` — passed.
- `just ci` — passed after the final review fixes: formatting, UI typecheck, 201 UI tests, production build, Go vet/full tests, headless compile, and every plugin/face module.
- `just vivy-code` — passed.
- `.\\vivy-code.exe --help` — passed.

Coverage includes authoritative field projection, non-mutation spoof rejection, wide split/narrow unified rendering, terminal-control and absolute-target redaction, vertical/mouse/horizontal scrolling, bounded offsets, gate-state reset, decision keys, risk-slice ownership, and built-in/packed wire parity.

## Real path

An isolated interactive launch was attempted with `VIVY_USER_HOME` under `.workspace/split-diff-smoke-home`. The PTY runner failed before process creation with Windows `CreateProcessW` OS error `-1073283067` (`FormatMessageW` error 317), matching the prior environment failure. No product runtime or tenant Journal was opened, so this is recorded as an environment limitation and not counted as a successful interactive smoke.

The built executable was moved to ignored `.workspace/build-artifacts/vivy-code.exe`; no build artifact is part of the delivery commit.
