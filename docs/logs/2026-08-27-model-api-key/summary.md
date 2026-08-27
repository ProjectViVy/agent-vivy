# 2026-08-27 · 模型密钥端到端支持（界面可填写 API Key）

## 目标与背景

用户反馈"不能填写模型密钥"，要求修复。Vivy 原有产品规则是密钥只由运行环境
注入（config `env_key`，D-010；`api_key:` 在 config.yaml 中是硬错误），UI 不
持有任何密钥字段，`settings/update` 只接受 `(provider, default_model, base_url)`。
用户明确选择**方案 A：端到端真实生效**——界面可填写密钥，密钥进入后端运行数据
并注入运行束 env，而非仅界面记录（伪操作）。

## 变更内容

### Go 后端

- `internal/app/settings/settings.go` — `Settings` 增加 `ApiKey string
  \`yaml:"api_key"\``；`Validate` 拒绝含换行符的密钥（防 YAML 注入畸形条目）；
  包文档更新：密钥可选的明文落盘于 `data/agent-home/settings.yaml`（0600，
  gitignored 运行数据，非提交配置），绝不写日志、绝不回传控制面；空值 = 无覆盖层，
  环境变量密钥照常生效。
- `internal/app/app.go` — `applySettingsOverlay` 启动时（与现有 base_url overlay
  对称）：非空 `api_key` 经 `os.Setenv` 注入当前运行束的 `env_key` 环境变量；
  空值不动环境变量（env 注入流程不受影响）；日志只打 `key_set` 布尔，绝不打印值。
- `internal/rpc/control.go` — `settings/get` 与 `settings/update` 新增
  `api_key_set` 布尔（值本身从不出现）；`settings/update` 接受可选 `api_key`
  （整档覆盖语义：缺省/空 = 清除覆盖层）。
- 测试：
  - `settings_test.go`：含密钥的 Save/Load round-trip；换行密钥校验拒绝。
  - `control_test.go`：设置密钥 → `api_key_set=true` 且结果 JSON 不含密钥值；
    不带 api_key 的更新清除覆盖层（`api_key_set=false`）。
  - 新增 `internal/app/settings_overlay_test.go`：overlay 注入密钥到 env_key；
    空密钥不动环境变量；无 settings 文档为 no-op。

### TS 前端

- `ui/src/lib/api.ts` — `Settings.api_key_set?: boolean`；新增
  `SettingsUpdate = Pick<Settings,'provider'|'default_model'|'base_url'> &
  { api_key?: string }`；`updateSettings` 归一 `api_key ?? ''`（缺省即清除）。
- `ui/src/lib/store.ts` — `saveSettings` 签名改为 `api.SettingsUpdate`。
- `ui/src/components/settings/custom-providers.ts` — 注册表条目增加 `apiKey`
  （本地副本 `vivy.ui.*`）；读侧兼容字段引入前保存的旧条目（缺省补 `''`）；
  新增 `customApiKeyFor(bundle, baseUrl)`：目标三元组命中自定义供应商时返回其
  密钥，目录/未知返回 `''`。
- `ui/src/components/settings/ModelSettingsCard.tsx`：
  - 自定义供应商对话框新增「API Key」字段（password，`autoComplete=off`，
    留空 = 应用时清除已配置密钥；编辑态预填本地副本），并附"仅存本机运行数据"
    提示；
  - 主表单三输入区扩展为 2×2：新增「API Key」密码框，随所选供应商回显（自定义
    条目显示其注册密钥、目录条目清空），随「保存真实设置」显式提交（留空 =
    清除覆盖层）——密钥在主表单区直接可见可填，不再只在对话框里；
  - 三处应用路径携带 `api_key`：目录/自定义模型行（`applyModelNow`）与已选
    chip（`applySavedNow`）经 `customApiKeyFor` 解析；「保存真实设置」表单提交
    `form.api_key`（空即清覆盖层）；
  - 运行配置已配置密钥时在保存按钮下方显示
    「已配置 API Key（值不会回传界面）」提示。
- `ui/src/components/chat/MaskAndModelSwitcher.tsx` — 顶栏快捷切换
  `selectModel` 同样携带 `customApiKeyFor` 解析的密钥。
- 文案：`i18n/{zh,en}.ts`（`settingsModel.apiKey / apiKeyPlaceholder /
  apiKeyHint / apiKeyConfigured`；`customDialogHint`、`settings.modelConfigDescription`、
  `welcome.introBody / secretNote` 改为新口径）；`SettingsView.tsx` 模型卡
  CardDescription 同步；`ui/AGENTS.md` 密钥规则行改写为新产品口径。

## 密钥语义契约（与后端一致，一处权威）

- `settings/update` 是整档覆盖：每次提交都携带完整（可能为空的）密钥状态。
- 界面解析：目标三元组命中自定义供应商且其 `apiKey` 非空 → 提交该值；否则
  `''`（清除覆盖层，回落运行束 env_key 环境变量密钥）。
- 生效时机：与 base_url overlay 一致，下次启动生效（无运行时热切换）。
- 安全边界：值落盘 `data/agent-home/settings.yaml`（0600、gitignored 运行数据）；
  `settings/get` 只回 `api_key_set`；控制面响应与日志均不含值。

## 明确不做

- 每网关独立密钥：运行束级 env 注入下，同一束不同 base_url 无法各自持钥 →
  登记 `docs/TODO.md` §0.1（`UI-MODEL-KEY-SCOPE`）。
- 密钥加密存储/二次确认：保持明文落盘 0600（同目录 settings.yaml 既有形态）。
- 运行时热切换：仍下次启动生效（与 base_url 语义一致）。
- 演示/本地模拟预览区的密钥字段：仍为占位（`vivy.demo.*` 禁用密钥）。
- 浏览器冒烟：8787 / 3015 被用户 Vivy Studio 调试会话占用，本次跳过自测，由
  用户 Studio 会话代验（见 `verification.md`）。

## 发布说明

无独立发布：随常规构建发布，`just ci` 已含 ui build 与 Go 全量测试，不单写
`release.md`。
