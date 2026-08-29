# 2026-08-30 model-list-sync — 设置 → 模型「刷新模型列表」

## summary.md

### 做了什么

设置 → 模型的模型列表新增「刷新」功能：从上游 OpenAI 兼容端点 `GET {base_url}/models`
拉取模型 id，并保存到本地供应商注册表（`settings.yaml` 的 `providers[].models`）。

分层改动（worktree `agent-vivy-model-sync`，分支 `feat/model-list-sync`）：

**后端**
- 新增 `internal/provider/discover.go`：`ModelListClient.List` —— 15s 超时上限、
  Bearer 密钥请求、4MiB 响应上限、trims/去重/保序；错误绝不携带密钥或 URL
  （`*url.Error` 剥离 URL 后保留 cause 链，D-010）。
- `internal/rpc/control.go` 新增 `settings/providers/refresh` RPC：
  - 按注册表 `id` 刷新，或按 `(bundle, base_url)` 定位；目录厂商（OpenAI 兼容）
    无注册表行时**克隆为新自定义条目**以持久化（前端提供 `display_name`）。
  - 密钥解析走 `settings.ActiveKey`（注册表条目密钥或旧版 overlay），写回时
    **只改 `models`，`api_key` 原样保留**（规避既有的“整条目替换清密钥”陷阱）。
  - **并集策略**：上游 id 在前（网关顺序），本地手动新增且上游没有的 id 追加在后，
    刷新不丢手工条目。
  - 失败不落盘：HTTP/解析失败返回脱敏错误，注册表保持原状。
  - Anthropic 原生条目/请求直接拒绝（其 API 无 `/models` 协议）；read-only 与
    Frozen(ENV 锁定) 场景与其它 settings 写一致拒绝。
  - capabilities 列表登记 `settings.providers.refresh`；`ControlDeps.ModelLists`
    注入（测试用 httptest，生产走默认 15s client）。
- 新增 `settings/providers/refresh` 的 API 冒烟与错误矩阵测试（`control_test.go`），
  复用既有 settings test env（`newSettingsHandlerEnvWith` 注入钩子，老签名不变）。

**前端**
- `ui/src/lib/api.ts`：`RPC_METHODS` 登记 + `ProviderRefreshInput` +
  `refreshProviderModels`。
- `ui/src/lib/store.ts`：`refreshProvider` action（成功 `loadProviders` 回读落盘结果，
  失败写 `providersError`）。
- `ui/src/components/settings/ModelSettingsCard.tsx`：模型列表头部在「新增」旁加
  「刷新」图标按钮（仅 OpenAI 兼容条目显示；刷新中 spinner + 禁用，防重复提交）；
  成功后显示「已从上游同步 N 个模型」；失败渲染 `providersError`。
- i18n：`settingsModel.refreshModels` / `refreshing` / `refreshed`（zh + en）。
- 新增 e2e `ui/e2e/model-refresh.spec.ts`：真浏览器 + 自起后端 + 本地 `/models`
  服务，覆盖空列表 → 刷新列出上游模型（校验 Bearer 密钥上行）→ 手动新增后并集
  保留 → 重载后落盘仍在且密钥不回显。

**e2e 过程中发现并修复的存量缺陷**：`toProviderEntryResult` 对空模型列表返回
`append([]string(nil), ...)` → JSON `models: null`；前端注册表校验
`isValidCustomProvider` 要求 `Array.isArray(models)`，于是**无模型的注册表条目被
静默丢弃、创建后不可见**（新建自定义供应商不填模型列表即触发）。修复为 wire 恒定
`models: []`（`internal/rpc/control.go` `toProviderEntryResult`），并加回归断言
（空模型 upsert / list 均序列化为 `[]` 而非 `null`）。

### 没做什么（明确范围外）

- 目录静态快照 `provider-catalog.ts` 未退化为运行时目录（仍在 TODO 语义内，需单独
  迭代）；刷新只影响经刷新/克隆的注册表条目与其展示。
- Anthropic 原生端点的模型列表拉取（协议不支持）；其列表仍为静态/手填。
- 自动选中新模型 / 改写 `default_model`（刷新不改动当前选择）。
- `vivy.ui.customProviders` localStorage 存量迁移（UI-PROV-REGISTRY，独立条目）。
- Studio overlay、用户插件、租户 Journal（`data/vivy.db`、`data/demo/`、
  `data/workspaces/`）均未触碰。

### 关键决策

- **并集而非替换**：保留用户手动「新增」的模型，避免刷新静默丢失本地条目。
- **克隆目录条目**：目录是前端静态快照、后端无对应行；要「保存本地」必须有可写
  目标，克隆为自定义条目是与既有「管理供应商」流一致的最小路径。
- **密钥只写不清**：refresh 与旧的 upsert 整条目替换不同，只带 `models` 回写，
  因此不会清掉已配置密钥；错误与响应永不携带密钥。