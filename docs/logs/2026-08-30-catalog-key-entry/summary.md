# 2026-08-30 · 目录厂商 API Key 可填写（设置 → 模型）

## 背景

用户反馈：在「设置 → 模型」选中目录厂商（如 DeepSeek）时，API Key 输入框
**无法编辑**，但提示文案却写着「目录厂商也可在此填写 API Key，写入本机用户
工作区。」——文案与行为矛盾。

根因：`catalogKeyHint` 文案在 `faeb76e`（sandbox 提交的措辞统一）中被改成
「目录厂商也可在此填写」，但 `ModelSettingsCard.tsx` 的输入框
`disabled={locked || !selectedEntry.custom}` 与 `commitPanelKey` 的
`!selectedRegistry` 早退从未同步放开——文案与实现脱节。用户选择了「真正开放
填写」方向。

## 变更内容

- `ui/src/components/settings/custom-providers.ts`
  - 新增目录厂商密钥**落地条**（catalog overlay）纯函数：
    `CATALOG_OVERLAY_PREFIX` / `catalogOverlayId(name)` /
    `isCatalogOverlayEntry(entry)` / `providerEntryByEndpoint(...)`（按
    `(bundle, base_url)` 查注册表条目，与后端 `ActiveKey` 解析口径一致）。
  - `allProviderEntries`：跳过 `catalog-*` 落地条，避免目录行旁出现重复的
    「自定义」行；普通自定义条目（含克隆）照常展示。
  - `customApiKeySetFor`：由「合并视图 custom 标记」改为「按端点命中注册表
    条目且 `api_key_set`」——目录厂商端点有密钥覆盖时同样显示「已配置
    API Key」。
- `ui/src/components/settings/ModelSettingsCard.tsx`
  - API Key 输入框启用条件改为 `locked || bundle === 'mock'`：目录厂商（
    openai/anthropic 束）可编辑；内置 Mock 离线束保持禁用。
  - `commitPanelKey` 支持目录厂商：失焦时按端点落盘——端点已有注册表条目
    （含既有自定义克隆）则更新其密钥，否则新建 `catalog-<name>` 落地条
    （空值且端点无条目时**不**凭空创建）；自定义条目路径保持原语义。
  - 新增 `panelKeyDirty` 脏标记：输入未被修改就失焦时不提交，防止「点进点
    出」误清已配密钥或凭空写条目（对自定义供应商同样生效）。
  - 占位符与提示：可编辑场景统一用 `apiKeyPlaceholder` / `apiKeyHint`；
    Mock 用 `catalogKeyHint`。
  - 顶部行为注释同步更新。
- `ui/src/i18n/{zh,en}.ts` — `catalogKeyHint` 改为 Mock 专属说明
  （「内置 Mock 供应商无需 API Key。」）；目录厂商可编辑场景已由
  `apiKeyHint`（写入 ~/.vivy/settings.yaml、不回传、下一条消息生效）承接。

## 安全边界（D-010）

无新增后端写入面：目录厂商密钥与自定义供应商密钥走同一条
`settings/providers/upsert` 路径，0600 写-only 落盘 `~/.vivy/settings.yaml`，
值不回传、不写日志；运行时由 `ActiveKey` 按 `(bundle, base_url)` 权威解析。

## 明确不做

- 后端零改动：`FindProvider` / `ActiveKey` / `upsertProvider` 本就按
  `(bundle, base_url)` 解析，目录厂商端点可直接命中注册表条目。
- 不做真实在线供应商目录同步（仍为 `docs/TODO.md` §0.1 `UI-PROV-RPC`）。
- 不为 Mock 提供密钥（内置离线束，settings 校验 `mock` 非法）。
- 不做「删除落地条」UI：空值失焦即清除密钥，条目留空无副作用
  （空 `api_key` 不产生覆盖）。