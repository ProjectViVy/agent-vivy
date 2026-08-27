# 2026-08-28 · Provider 直接写配置 + 写时环境变量同步 + 系统级用户默认工作空间

## 目标与背景

用户指出「现在不支持直接配置写入，得在环境变量处理」是有问题的产品方向；
要求（1）**完全变更整体写 provider 逻辑**——UI 里新增/编辑/删除供应商、配置
base_url/模型列表/API Key 不再是 localStorage 本地偏好，而是真实写入后端持久化
文档，且写入后**同步更新环境变量值**；（2）默认有一个**系统级用户工作空间**
（diva 式：`~/.vivy` 含 `workspace/` 与其它内容）。

## 变更内容

### Go 后端

- `internal/app/settings/settings.go` — Settings 新增 `Providers []ProviderEntry`
  注册表（id/display_name/bundle/base_url/default_model/models/api_key）；逐条校验
  （缺字段、坏 bundle、坏 URL、换行 key、重复 (bundle,base_url) 拒绝，mock 不可
  注册）；`FindProvider`（按 bundle+base_url 命中）、`ActiveKey`（注册表条目密钥
  优先、legacy `api_key` 覆盖层兜底）、`UpsertProvider`、`IsZero`；Load 归一空
  注册表为 nil。
- `internal/app/app.go` — 抽出 `applySettingsEnv`（base_url → `VIVY_API_BASE`、
  解析出的 key → 活动 bundle 的 `env_key`），启动 overlay 与**写时同步共用**
  （经 `ControlDeps.ApplySettingsEnv` 回调）：设置/供应商写入后立刻更新当前进程
  环境变量，下次启动由同文档重放。
- `internal/rpc/control.go` — 新 RPC：`settings/providers`（列注册表，只回
  `api_key_set`）、`settings/providers/upsert`（按 id 新建/更新，key 写-only）、
  `settings/providers/delete`；`settings/update` 改为**读-改-写**（不再整档覆盖，
  注册表/网络/execute 段全部保留），active api_key 改由后端权威解析；能力广播
  增三项；`settings/get` 的 `api_key_set` 反映解析后的 key。
- 测试：注册表 round-trip/校验/冲突、ActiveKey 优先级、UpsertProvider、
  RPC 列表/upsert/delete/拒绝/read-only、update 保注册表、写时 env 回调计数。

### TS 前端

- `ui/src/lib/api.ts` — RPC_METHODS 增三项；`ProviderEntry`/`ProviderEntryInput`/
  `ProvidersView` 类型与 `listProviders/upsertProvider/deleteProvider`。
- `ui/src/lib/store.ts` — `providers/providersPhase/providersError` 状态与
  `loadProviders/saveProvider/removeProvider`；`initialize()` 并行载入注册表。
- `ui/src/components/settings/custom-providers.ts` — **删除 localStorage**
  （`vivy.ui.customProviders` 不再读写）；改为纯逻辑层（校验/冲突/合并/折叠/
  检索/`customApiKeySetFor`），数据源 = store 中 wire `ProviderEntry[]`。
- `ui/src/components/settings/ModelSettingsCard.tsx` — 注册表 CRUD 走
  `saveProvider/removeProvider` RPC；对话框/新增模型/面板 Key 均写后端；选模型/
  chip/快捷切换 `settings/update` **不再携带 api_key**（后端按注册表解析）；
  「已配置 API Key」提示用 `customApiKeySetFor`。
- `ui/src/components/chat/MaskAndModelSwitcher.tsx`、`NetworkToolsCard.tsx`、
  `GenerationParamsCard.tsx`、`saved-models.ts` — 厂商标签/快捷切换/网络偏好
  适配注册表参数与去 key。
- `ui/src/i18n/{zh,en}.ts` — apiKeyHint 改「写入运行数据并同步环境变量」；
  新增 `errors.saveFailed`。
- `ui/AGENTS.md` — 密钥/注册表规则改写为新口径。
- 测试：custom-providers.test.ts 重写为纯逻辑；saved-models.test.ts 标签接
  providers 参数；store.test.ts mock 补 `listProviders`。

### 系统级用户默认工作空间（diva 式）

- `internal/config/config.go` — 新增 `userDataRoot()`：`VIVY_USER_HOME` →
  `os.UserHomeDir()/.vivy` → 回退 `data`（CI/开发兜底）；`Default()` 的
  sqlite/workspace_root/skills_root/data_dir 默认全部落到该根下；
  `DataDirectory()` 的 postgres/兜底分支同步。
- `config.example.yaml` — 默认路径注释改指用户主目录（显式值仍优先）。

## 密钥语义契约（一处权威）

- `settings/providers/upsert` 的 `api_key` 是**写-only**：落 0600 运行文档，
  永不回传、永不进日志；`settings/providers` 只回 `api_key_set`。
- active key = 注册表 (bundle,base_url) 命中条目的 key，否则 legacy
  `settings.ApiKey` 覆盖层；都没有则不动环境（回落运行束 env_key）。
- 生效时机：写入即同步进程环境变量；已构造的模型仍「下次启动生效」（与既有
  overlay 契约一致，无运行时热换）。

## 明确不做

- 不做运行时引擎热切换（写入同步 env；模型重建仍需重启）。
- 不做每网关独立密钥的运行时解析改造（`UI-MODEL-KEY-SCOPE` 的 provider 层
  按 base_url 取 key 仍 OPEN；注册表与写路径后端化为其铺路）。
- 不做真实在线供应商目录同步（`UI-PROV-RPC` 保持 OPEN）。
- `saved-models` 书签继续 localStorage（UI 偏好，非 provider 配置）。
- 本迭代在独立 worktree `feat/provider-direct-write` 完成（根树并行合并中）。

## 发布说明

无独立发布：随日常构建发布，`just ci` 已含 go test + ui build；不单写
`release.md`。