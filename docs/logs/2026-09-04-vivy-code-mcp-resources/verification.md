# Verification

Commands were run from the repository root unless noted. Go uses
`C:/Program Files/Go/bin/go.exe` because it is not on this shell's PATH.

- `gofmt` on all changed Go files — passed.
- `go test ./internal/runtime -run 'TestEinoMCPBackend' -count=1` — passed.
- `go test ./internal/rpc -run 'TestSettingsCapabilitiesAdvertised|TestMCPSettingsCRUDAndProbe' -count=1` — passed.
- `go test ./sdk/tui/command ./sdk/tui/view -count=1` — passed.
- `go test ./internal/tui -run 'TestLiveAdvancedCommandsUseAuthoritativeRPCAndOverlayResult|TestREPLAdvancedCommandsUseRPCAndPrefixesStayLocal' -count=1` — passed.
- `go test . -count=1` from `faces/tui` — passed.
- `go test ./internal/runtime ./internal/rpc ./internal/tools ./sdk/tui/... ./internal/tui/... -count=1` — passed; includes the full long-running runtime and RPC suites.
- `go test ./... -count=1` from `faces/tui` — passed.
- `go test -race ./internal/runtime -run 'TestEinoMCPBackend' -count=1` — passed.
- `go test -race . -count=1` from `faces/tui` — passed.
- `go run ./sdk verify faces/tui` — passed.
- `go run ./sdk pack --face tui --out .workspace/tui-mcp-resources-pack-20260904-v4` — passed; generation `gen_617e8d183cf28612`, SHA-256 `5554a102dcec9fd880cfe27c2bfb3bf4c9ba5c045c3d7b9fbf16b2e7a726bbde`.
- `just ci` — passed; UI typecheck/197 tests/build, Go formatting/vet/tests/headless compile, and every plugin/face gate passed.
- `vivy-code` interactive PTY smoke — passed at 80x24; `/mcp resources missing` opened a local command error with `rpc error -32004: mcp server not found`, did not start a model turn, and Ctrl+C exited cleanly.
- LUNA MAX read-only final audit — PASS after three rounds; no remaining P1/P2 blocker in pagination, SSE/request IDs, effective catalog, bounds, representation, error safety, or TUI parity.

Regression coverage includes bounded multi-page cursors, multi-event SSE with
an interleaved notification, JSON-RPC response ID matching, negotiated protocol
headers, config-only catalogs, opaque URI preservation, malformed text/blob
content rejection, empty text/blob representation, multi-server fan-out bounds,
typed remote errors, and terminal-control sanitization/redaction.

One broad race run (`internal/runtime`, RPC, and TUI packages together) hit the
pre-existing timing-sensitive `TestCronAtJobDeletesAfterSuccessfulRun` timeout;
the RPC and TUI packages in that run passed. The changed MCP backend and packed
face were then rerun separately under the race detector and passed. The normal
full runtime suite passed. A later `just ci` attempt transiently hit the existing
`TestCronAtJobDisablesAfterRun` and `TestJobRegistryForegroundRunContextError`
timing tests; both passed immediately when rerun alone, and the subsequent
unchanged-tree `just ci` passed completely. No production tenant Journal or
Studio state was read or written.
