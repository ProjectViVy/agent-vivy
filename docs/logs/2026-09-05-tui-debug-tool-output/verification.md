# Verification

## Automation

- `go test ./internal/config ./sdk/tui/view ./sdk/tui/face`: passed.
- `go test -tags vivy_headless ./internal/codeface ./cmd/vivy ./cmd/vivy-code`: passed.
- `just ci`: passed. This included fmt-check, UI typecheck, 24 Vitest files / 201 tests, UI build, Go vet, full-repository Go test, headless compile, and checks for all independent plugin/face modules.

## Real-path smoke

- `just vivy-code`: successfully built the real `vivy-code.exe`.
- Started the default configuration with an isolated `VIVY_USER_HOME`: the process stayed running and entered the interactive TUI loop; it was terminated after 2 seconds according to the test plan.
- Started the same EXE again with a temporary `VIVY_CONFIG` (`tui.debug: true`): the configuration passed strict parsing, and the process stayed running and entered the interactive TUI loop; it was terminated after 2 seconds according to the test plan.
- The temporary EXEs, configuration, and user directories from both smoke runs were deleted; no existing tenant Journal was read or written.
