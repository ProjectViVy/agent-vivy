# Restore runtime.mock for offline start (2026-08-30)

## 变更内容

编译修复把 `ModelResolver` / `NewResolvingChatModel` 合回主线后，产品路径不再走 `config.Runtime.Mock`，Catalog 也拒绝 `mock`。`just dev` / `dev.ps1` 在没有 API key 时仍切到 `config.dev.yaml`（`runtime.mock: true`），结果进程能听端口，但解析出的模型 `Ready=false`，对话立即失败（欢迎向导 / 「no model configured」）。

本迭代把离线 mock 接回 resolver 与 Catalog，不改 UI、不删 `runtime.mock` 配置。

### 内核

- `ModelResolver`：`runtime.mock=true` 时解析为 ready 的 `mock` / `mock:<scenario>`，且不冻结 ENV 会话（避免残留 `OPENAI_API_KEY` 盖过 `just dev`）。
- `Catalog.For("mock")` 再次返回 mock Ref（仅配置/测试路径；operator settings 仍拒绝 mock）。
- `defaultModelFor` 恢复 mock / mock_scenario 回退。

## 明确不做

- 不删除 `config.Runtime.Mock`（`just dev` 与 e2e 仍依赖它）
- 不改欢迎向导 / 设置页文案
- 不处理本机已占用的 Vite `:3015`（环境问题，不是这次代码缺口）

## 变更文件

- `internal/app/model.go`, `internal/app/model_test.go`, `internal/app/app.go`
- `internal/provider/{catalog.go,doc.go,mockref.go,provider_test.go}`
