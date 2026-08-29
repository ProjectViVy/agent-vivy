# 2026-08-29 · 真实 LLM 全流程闭环

## 目标

废弃产品 MOCK，把 Vivy 接到真实 OpenAI 兼容 API：向导登记第一个供应商 → 设置→模型增删改 → 顶栏切换当前模型 → 日常聊天端到端打到该供应商。多版本 / 多进程共用一套用户工作区。

## 变更

### 配置与启动

- 产品启动不再读取 cwd `config.yaml`。`VIVY_CONFIG` 才是部署叠加；否则用 `config.Default()`，数据根为 `VIVY_USER_HOME` 或 `~/.vivy`。
- 缺 API Key **不再 abort 启动**。无配置时进程起来，聊天返回可行动错误。
- `just dev` / `just run` 使用 `VIVY_USER_HOME=data/dev-home` 与 `config.dev.yaml`（只放 Vite `allowed_origins`），不再 `runtime.mock: true`。
- SQLite Journal 增加 organism lease：第二进程占用同一工作区时明确失败。

### 模型解析

- `provider.Ref.Model` 改为 `ModelSpec{ID, APIKey, BaseURL}`，不再 `os.Getenv(bundle.EnvKey)`。
- `internal/app.ModelResolver`：ENV 临时会话（任一 bundle `env_key` 非空）冻结本进程且 UI 只读；否则读用户工作区 `settings.yaml`。
- `provider.NewResolvingChatModel` 每次 Generate/Stream 按当前 spec 构造；设置写入后下一条消息即生效。
- 删除 `applySettingsEnv` / `os.Setenv` 主路径。

### 产品 MOCK

- Catalog 不再解析 `mock`。`runtime.mock` / `mock_scenario` 从产品 Config 删除。
- UI 目录去掉 Mock 条目。`NewMock` 仅留测试替身。

### UI

- 向导第二步登记真实供应商（显示名 / Base URL / 模型 / API Key），走 `settings/providers/upsert` + `settings/update`。
- 设置→模型：目录与自定义都可填 Key；`frozen` 时整卡只读。
- 顶栏切换不再发送空 `api_key` 清密钥。
- e2e 用本地 OpenAI 兼容桩（`ui/e2e/openai-stub.mjs`）替代 mock provider。

### 规则

- `AGENTS.md` / `ui/AGENTS.md` / `vivy-kernel-ci`：运行时密钥在用户工作区；ENV 仅冻结本进程；不要碰操作者 `~/.vivy`。

## 明确不做

- 不把 provider 做成运行时热挂插件。
- 不接线 Anthropic Messages 适配器。
- 不迁移旧 `vivy.ui.customProviders` localStorage（`UI-PROV-REGISTRY` 仍 OPEN）。
- 不在单测里打真实外网。
