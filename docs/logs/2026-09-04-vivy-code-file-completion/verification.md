# Verification

## Automated gates

- `go test -race ./sdk/tui/command ./sdk/tui/view ./internal/rpc ./internal/tui`
  — PASS.
- `cd faces/tui; go test -race ./...` — PASS.
- Focused built-in and packed RPC tests — PASS; both emit
  `project-context/list` with `{query, limit: 200}` and receive metadata-only
  results.
- `TestFileCompletionRunsThroughBubbleTeaProgram` — PASS under `-race`; a real
  Bubble Tea program loop processed `@REA`, the 120 ms debounce, asynchronous
  completion, and Enter selection at 80x24.
- `just ci` — PASS: formatting, UI typecheck, 201 UI tests, UI build, Go vet and
  full tests, headless compile, and every plugin/face module.

## Executable smoke

- `just vivy-code` — PASS; the independent headless-tagged executable built.
- `vivy-code.exe --help` — PASS and identified the independent VIVY CODE
  private-instance product.
- A native interactive PTY launch was attempted twice, but this execution
  environment failed before starting the binary while creating its PowerShell
  PTY host (`CreateProcessW`, OS error `-1073283067`). The feature's interactive
  key path was therefore exercised through the real Bubble Tea event loop test
  above rather than claimed as a native PTY pass.

## Review

Two GPT-5.6-LUNA MAX read-only reviewers audited architecture/security and
implementation/QA. Their blocking quoting finding and important candidate-cap,
metadata validation, I/O budget, and in-flight cancellation findings were fixed
before the final gate. The unrelated `/files` ownership finding was captured in
the backlog.
