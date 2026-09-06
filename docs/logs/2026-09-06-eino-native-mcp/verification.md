# Verification

以下命令均在 `agent-vivy-eino-mcp` worktree、`feat/eino-mcp` 分支运行；未访问 tenant data。真实路径 smoke 只访问本机 loopback 上的官方 mcp-go fixture，并把 Vivy 数据根隔离到系统临时目录。

## 已运行

- `gofmt -w internal/runtime/mcp_backend.go internal/runtime/mcp_backend_test.go internal/app/app.go` — 通过。
- `go mod tidy` — 通过；新增直接依赖 `github.com/cloudwego/eino-ext/components/tool/mcp v0.0.9` 与 `github.com/mark3labs/mcp-go v1.0.0`。
- `go test ./internal/runtime -run '^$' -count=0` — 通过（编译检查）。
- `go test ./internal/runtime -run '^(TestBoundedMCPBodyReaderSemantics|TestEinoMCPBackend)' -count=1 -v -timeout 180s` — 通过，12 个 MCP/边界测试；覆盖 JSON/SSE、Eino schema、modern discover 无 legacy session、auth rotation、concurrent init、failed-init recovery/close、session retry/no-retry、replace/remove/close、resources/prompts/non-text fail-closed、tools/resources 32-page bounds、aggregate/content/raw bounds、`io.Reader` exact/over-limit/zero-read 语义、error mapping/browser-use/sort、closed-backend rejection。
- `go test -race ./internal/runtime -run '^(TestBoundedMCPBodyReaderSemantics|TestEinoMCPBackend)' -count=1 -timeout 180s` — 通过。
- `go test ./internal/app -run '^TestAppShutdownBounded$' -count=1 -timeout 180s` — 通过；验证 App shutdown 后 MCP backend 拒绝操作。
- `just ci` — 通过（UI 24 files / 201 tests、UI build、Go vet/test、headless compile、plugin/faces checks；`internal/runtime` 193.582s）。
- 真实 split-path smoke：
  - 以 `go run github.com/mark3labs/mcp-go/examples/everything@v1.0.0 -t http` 启动官方 Streamable HTTP fixture（`127.0.0.1:8080/mcp`）。
  - 以隔离的 `VIVY_USER_HOME=<system-temp>/vivy-eino-mcp-smoke-*`、`VIVY_ADDR=127.0.0.1:8797` 运行 `just run`；在 `ui/` 以 `VIVY_BACKEND_ADDR=http://127.0.0.1:8797` 运行 `pnpm dev`。
  - 通过 `http://127.0.0.1:3015/rpc/bootstrap` 与同源 WebSocket 依次执行 `settings/mcp/upsert`、`settings/mcp/probe`、resources list/read、`commands/list` 与 prompt expand：探测到 6 个工具、101 个资源，resource read 保持 `untrusted=true`，发现 2 个 MCP prompts 并成功展开 simple prompt。
  - CUA 没有可用 browser，`agent-browser` 技能对应 CLI 也未安装；未安装新工具，改用仓库已存在的 Playwright 1.62.1 打开 `http://127.0.0.1:3015/mcp`。页面显示 `official-local`、正确 endpoint、`已连接` 与 `6 个工具`，console error 为 0；全页截图经人工检查无错误态、遮罩或布局异常。
  - smoke 后按已核对的 PID 停止官方 fixture、临时 Vivy 与 Vite；3015/8080/8797 均不再监听。系统安全策略拒绝删除隔离的临时目录，因此未把它误当仓库产物清理；该目录不在 worktree、不会提交。
- 历史一次 `go test ./internal/runtime -count=1 -v -timeout 120s` 曾在既有 `TestRunWithoutProvenanceKeepsUISource` 的 SQLite 临时数据库迁移处超时；后续 `just ci` 的完整 runtime 测试已通过，故该现象未复现为本次 MCP 失败。
