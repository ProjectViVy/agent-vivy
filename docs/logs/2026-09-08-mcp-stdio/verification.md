# Verification

The following records include this lane's targeted regressions and the unified
gate results actually run by the supervisor.

| Command | Result |
|---|---|
| `"C:\Program Files\Go\bin\go.exe" test ./internal/commandpolicy ./internal/config ./internal/app/settings ./internal/app ./internal/tools -run 'TestExecutablePolicy|TestShellEscapePolicy|TestMCP|TestValidateMCP|TestSaveAndLoadMCPServers|TestApplySettingsOverlayMCPServers|TestBashClassifier' -count=1` | PASS |
| `"C:\Program Files\Go\bin\go.exe" test ./internal/runtime -run 'TestMCPBackend|TestMCPStatusError|TestBoundedMCPStatus' -count=1` | PASS; HTTP regression, stdio lazy/resources/prompts/env/cwd/fail/death/replace/close, bounds, and redaction |
| stdio targeted (3 groups) | PASS |
| `"C:\Program Files\Go\bin\go.exe" test ./internal/runtime -run '^TestMCPBackendStdioLazyProjectionAndCall$' -count=20` | PASS; rechecked the original transport-closed flaky behavior |
| `"C:\Program Files\Go\bin\go.exe" test ./internal/runtime -run 'TestMCPBackendStdioReplaceRebuildsAndCloseIsIdempotent' -count=10` | PASS; rechecked the Windows close race |
| `"C:\Program Files\Go\bin\go.exe" test ./internal/rpc ./sdk/tui/live ./sdk/tui/view -run 'TestMCP|TestSessionSidebar|TestMapSidebarView|TestSidebarRendersMCP' -count=1` | PASS; RPC/sidebar/live/surface/render projection |
| `pnpm typecheck`（`ui/`） | PASS |
| `pnpm test -- src/components/mcp/mcp-import.test.ts --run`（`ui/`） | PASS；6 tests |
| `git diff --check` | PASS |
| `just ci` | PASS; supervisor's unified gate |
| First `just ui-e2e` | Failed before browser startup: the local machine lacked Playwright Chromium; no test assertions were reached |
| `pnpm exec playwright install chromium` (multiple attempts) | Environment blocked: the download repeatedly hit `ERR_SSL_DECRYPTION_FAILED_OR_BAD_RECORD_MAC` at about 90%; this was a browser-installation/network error, not a test-assertion failure |
| split-browser `http://127.0.0.1:3015` smoke | PASS; used the repository-isolated E2E config to start Go `:8787` + Vite `:3015`; agent-browser connected to system Chrome, `/mcp` had content and no page/console errors; created stdio `smoke-local`, the warning was visible, and the list showed only command; missing host env projected child key `MCP_TOKEN`; persisted across reload; probe was disabled after toggle; delete returned to the empty state. Tenant Journal was not touched |

This round of testing did not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.

Additional note: the CUA channel for `agent-browser-verify` was unavailable, and
the Playwright Chromium download was again blocked by
`ERR_SSL_DECRYPTION_FAILED_OR_BAD_RECORD_MAC`. Therefore, the real smoke was
completed by having agent-browser connect to system Chrome CDP; it is not
recorded as a `just ui-e2e` PASS.
