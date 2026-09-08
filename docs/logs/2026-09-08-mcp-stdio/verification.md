# Verification

以下记录包含本 lane 的定向回归，以及监督端已实际执行的统一门禁结果。

| 命令 | 结果 |
|---|---|
| `"C:\Program Files\Go\bin\go.exe" test ./internal/commandpolicy ./internal/config ./internal/app/settings ./internal/app ./internal/tools -run 'TestExecutablePolicy|TestShellEscapePolicy|TestMCP|TestValidateMCP|TestSaveAndLoadMCPServers|TestApplySettingsOverlayMCPServers|TestBashClassifier' -count=1` | PASS |
| `"C:\Program Files\Go\bin\go.exe" test ./internal/runtime -run 'TestMCPBackend|TestMCPStatusError|TestBoundedMCPStatus' -count=1` | PASS；HTTP 回归、stdio lazy/resources/prompts/env/cwd/fail/death/replace/close、bounds 与脱敏 |
| stdio targeted（3 组） | PASS |
| `"C:\Program Files\Go\bin\go.exe" test ./internal/runtime -run '^TestMCPBackendStdioLazyProjectionAndCall$' -count=20` | PASS；复核原 transport-closed flaky |
| `"C:\Program Files\Go\bin\go.exe" test ./internal/runtime -run 'TestMCPBackendStdioReplaceRebuildsAndCloseIsIdempotent' -count=10` | PASS；复核 Windows close race |
| `"C:\Program Files\Go\bin\go.exe" test ./internal/rpc ./sdk/tui/live ./sdk/tui/view -run 'TestMCP|TestSessionSidebar|TestMapSidebarView|TestSidebarRendersMCP' -count=1` | PASS；RPC/sidebar/live/surface/render 投影 |
| `pnpm typecheck`（`ui/`） | PASS |
| `pnpm test -- src/components/mcp/mcp-import.test.ts --run`（`ui/`） | PASS；6 tests |
| `git diff --check` | PASS |
| `just ci` | PASS；监督端统一门禁 |
| 首次 `just ui-e2e` | 启动浏览器前失败：本机缺少 Playwright Chromium；没有执行到测试断言 |
| `pnpm exec playwright install chromium`（多次） | 环境阻塞：下载约 90% 处反复 `ERR_SSL_DECRYPTION_FAILED_OR_BAD_RECORD_MAC`；这是浏览器安装/网络错误，不是测试断言失败 |
| split-browser `http://127.0.0.1:3015` smoke | PASS；使用仓库隔离 E2E config 启动 Go `:8787` + Vite `:3015`；agent-browser 连接系统 Chrome，`/mcp` 有内容且无 page/console errors；新建 stdio `smoke-local`，警告可见，列表只显示 command；缺宿主 env 投影 child key `MCP_TOKEN`；reload 持久化；toggle 后 probe disabled；delete 回空态。未触碰 tenant Journal |

本轮测试未读取或写入 `data/vivy.db`、`data/demo/`、`data/workspaces/`。

补充：`agent-browser-verify` 的 CUA 通道不可用；Playwright Chromium 下载又受 `ERR_SSL_DECRYPTION_FAILED_OR_BAD_RECORD_MAC` 阻塞，因此真实 smoke 改由 agent-browser 连接系统 Chrome CDP 完成，不将其记作 `just ui-e2e` PASS。
