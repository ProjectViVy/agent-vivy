# Verification

## Targeted automated checks

- `go test ./internal/runtime -run TestEinoMCPBackendListsCallsAndReconnects -count=1 -timeout 30s` — PASS.
- `go test ./internal/rpc -run 'TestSessionSidebar|TestBuildSidebar' -count=1 -timeout 30s` — PASS.
- `go test ./sdk/tui/view -run TestSidebar -count=1 -timeout 30s` — PASS.
- `go test ./internal/tui -run 'Test(MapSidebarView|SuccessfulMCPCommand)' -count=1 -timeout 30s` — PASS.
- `cd faces/tui; go test . -run 'Test(MapSidebarView|SuccessfulMCPCommand)' -count=1 -timeout 30s` — PASS.
- `git diff --check` — PASS.

## Product gate

- Final code-stable `just ci` — PASS: formatting, UI typecheck, 201 UI tests,
  production UI build, Go vet/full tests, headless compilation, and every
  plugin/face vet+test slice passed.
- `just vivy-code` and `vivy-code.exe --help` — PASS; the independent
  headless-tagged executable built and printed the expected private-instance
  VIVY CODE contract.

## Real path

- A native Windows PTY launch was attempted with an isolated
  `VIVY_USER_HOME=.workspace/sidebar-integrations-smoke-home`, but this
  execution host failed before process startup with `CreateProcessW` OS error
  `-1073283067` (`FormatMessageW` error 317). The directory was not created and
  no interactive pass is claimed. Mouse/focus/resize/dialog behavior is covered
  by the real Bubble Tea update-loop tests; executable construction and the
  non-interactive `--help` launch passed.

## Review

- Three GPT-5.6-LUNA MAX specialists audited LSP ownership, MCP/skills semantics, and Bubble Tea/Crush mouse behavior.
- Their findings drove the fixed-logo viewport, click-selected wheel owner, press-only event handling, honest `configured`/`initialized` MCP vocabulary, enabled-only skill label, post-`/mcp` refresh, and the explicit refusal to infer LSP state across run workspaces.
