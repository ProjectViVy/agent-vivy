# Toolkits closure attempt — 2026-10-12

**Branch:** `TOOLKITS`. **Status:** Main implementation path integrated and verified; TK-13 and the full Toolkits plan remain In progress.

The default App now composes the Toolkit, Mask and MCP owners. A real App/RPC lifecycle test covers native preview, install disabled, enable, captured Skill read, disable, uninstall, orphan history and reopen. A second App test removes Mask and MCP from the compiled Assembly and proves their capabilities are reported unavailable and their component previews are rejected.

The packaged default binary passed the dedicated Playwright flow. UI coverage includes native Toolkit preview/install, PM/Dev recommendation-only Masks, independent Skill reading, separate Hook and MCP consent/bindings, no eager MCP connection, update consent invalidation, disable and uninstall history. Both SQLite and live PostgreSQL storage conformance passed. The default artifact and two owner-omission artifacts packed and inspected successfully.

During omission verification, the App reported Mask/MCP as unavailable, but the Manager still hard-coded every owner as available for preview and resource validation. The Manager now uses its configured compiled-owner capability map. The App omission regression test failed before this change and passes afterward.

An expanded 390px mobile browser pass found that the component-content dialog did not return keyboard focus to its opener after Escape. `ToolkitPage` now restores focus to the exact component button, and the browser test covers populated content, long-token wrapping, dialog bounds, Escape and focus return. The test runs under a separate Playwright config so its installed fixture cannot contaminate the lifecycle E2E database.

The full v1 exit is not claimed. Remaining gates include the complete A01–A28 / B1–B14 reconciliation, publication/recovery fault injection, the complete run-admission and concurrency matrix, a real MCP transport smoke, broader viewport and blocker/error-state UI acceptance, and full `just ci`. The macOS environment does not have `just`, `pwsh` or `powershell`. See the verification and acceptance logs for exact evidence and limits.
