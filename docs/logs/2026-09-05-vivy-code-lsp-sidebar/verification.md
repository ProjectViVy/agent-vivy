# Verification

## Targeted checks

- `go test ./internal/app -run TestLanguageServerStatusSource -count=1` — PASS.
- `go test ./internal/rpc -run TestSessionSidebarUsesAuthoritativeOwners -count=1` — PASS.
- `go test ./internal/runtime -run 'TestWorkspaceManager(Existing|Allocates)' -count=1` — PASS.
- `go test ./internal/storage/sqlite ./internal/storage/postgres -run 'TestConformance|TestBackend' -count=1` — PASS.
- `go test ./sdk/tui/view ./internal/tui` — PASS.
- `cd plugins/lsp; go test ./...` — PASS.
- `cd faces/tui; go test ./...` — PASS.
- Coverage includes exact-root isolation, side-effect-free known-empty inspection, starting/initialized truth, default-generation absence, latest-primary session ownership, historical/child workspace exclusion, SQL bounded lookup, no-create resolution, provider timeout, invalid-state/path/ANSI filtering, built-in/packed mapping, hidden/known-idle rendering, post-tool coalescing, and TTL refresh persistence after a transient unknown snapshot.

## Product and plugin gates

- Final code-stable `just ci` — PASS: formatting, UI typecheck, 201 UI tests,
  production UI build, Go vet/full tests, headless compilation, and every
  plugin/face vet+test slice passed.
- `just vivy-code` and `vivy-code.exe --help` — PASS; the independent
  headless-tagged executable built and printed the expected private-instance
  VIVY CODE contract.
- `just sdk` and `vivy-sdk verify plugins/lsp` — PASS.
- `vivy-sdk pack --with lsp` — PASS; generated
  `dist/gen_aff6734611e88dd9/vivy.exe` with artifact SHA-256
  `3a153100e1b895be01e43bb3df1c9b0b115e4d6f4a8aa9c675e74c6033046d66`.
- `vivy-sdk inspect-artifact dist/gen_aff6734611e88dd9` — PASS; manifest
  names only plugin `lsp`, seam `tool-world`, the three declared grants, and
  the five expected `lsp_*` tools.
- A generated-executable `--help` probe was attempted, but daily `vivy.exe`
  does not implement that flag and started the service with built-in defaults.
  The exact candidate path was resolved, PID 36636 was matched by executable
  path and stopped, and no executable smoke is claimed. It selected the default
  user home before termination; existing user state was neither inspected nor
  cleaned.

## Real path

- A native Windows PTY launch was attempted with isolated
  `VIVY_USER_HOME=.workspace/lsp-sidebar-smoke-home`, but this execution host
  failed before process startup with `CreateProcessW` OS error `-1073283067`
  (`FormatMessageW` error 317). The directory was not created and no
  interactive pass is claimed. The real Bubble Tea update loop is covered by
  shared/built-in/packed tests; executable construction, the non-interactive
  `--help` launch, plugin verification, and packed-artifact inspection passed.

## Review

- Three GPT-5.6-LUNA MAX specialists audited plugin ownership, session/run/workspace resolution, control-plane/face mappings, and Crush status semantics before implementation.
- The final review round approved the bounded latest-primary lookup,
  timeout/coalescing isolation, strict state/label filtering, and TTL refresh
  behavior with no merge-blocking findings.
