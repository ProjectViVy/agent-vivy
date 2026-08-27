# 2026-08-27 · 自定义供应商注册表（可自定义供应商与模型）

## 目标与背景

上一交付（「已选模型」快捷切换）的快捷列表只能由静态目录模型行加入；手工输入
的自定义组合（private 网关 / 本地 LLM 等不在 48 家静态目录里的端点）无法进入
快捷流，且自定义条目厂商标签回退为原始束名（如 "openai"），难以识别。

本迭代按用户确认的两个决策落地：

1. **范围 = 自定义供应商注册表**（Agent-Diva custom provider CRUD 的 vivy 适配）：
   设置页可新增/编辑/删除自定义供应商（显示名 + 运行束 + Base URL + 默认模型 +
   模型列表），并入左侧供应商面板；点其模型即加入「已选模型」并立即选用。
2. **厂商标签来源 = 显示名优先**：注册表存 displayName（单一权威来源）；未注册
   的手打组合回退 baseUrl 主机名（如 `my-gateway.example.com`、`localhost:11435`）；
   无 baseUrl 再回退原始 provider 束名。

## 变更内容

### 新增

- `ui/src/components/settings/custom-providers.ts` — 注册表持久化模块（沿用
  saved-models.ts / mask-catalog.ts 样板：模块级缓存 + `useSyncExternalStore` +
  自定义事件 / `storage` 事件广播，不进 zustand store；SSR guard + 逐条校验）：
  - localStorage key `vivy.ui.customProviders`（真实功能，禁用 `vivy.demo.*`）；
  - `CustomProvider = { id, displayName, bundle(openai|anthropic), baseUrl,
    defaultModel, models }`；id 自动生成（`custom-<uuid>` + 非 crypto 回退），
    重命名/编辑不改 id；快捷列表经 baseUrl 关联，不引用 id；
  - `getCustomProviders / addCustomProvider / updateCustomProvider /
    removeCustomProvider / useCustomProviders`；add/update 校验 `(bundle, baseUrl)`
    与目录及既有自定义条目冲突，冲突返回 null（表单就地报错）；
  - `parseCustomModels`：换行 / 逗号 / 中文逗号分隔，去空去重；
  - 合并视图（薄适配层，`provider-catalog.ts` 静态目录保持纯净）：
    `MergedProviderEntry = ProviderCatalogEntry & { custom, registryId? }`、
    `allProviderEntries`（目录在前 + 自定义在后）、`searchMergedProviders`、
    `matchMergedProviderEntry`（目录优先，其次自定义，base_url 空沿用束名回退）、
    `splitMergedByFold`（自定义永不入 more）。
- `ui/src/components/settings/custom-providers.test.ts` — 16 个 vitest 纯逻辑用例：
  CRUD / 冲突 / 坏数据过滤 / 写回 / `parseCustomModels` / id 唯一 / 合并视图
  （顺序、custom 标记、检索、匹配优先级、折叠排除、搜索态平铺）。

### 修改

- `ui/src/components/settings/saved-models.ts` — `savedModelVendorLabel` 改为：
  目录/注册表命中 → displayName；否则带 Base URL → 主机名（`new URL().host`，
  try/catch，非法 URL 或空 host 回退）；否则原始 provider。书签标签经 baseUrl
  关联注册表，重命名即全局生效，无派生副本。
- `ui/src/components/settings/saved-models.test.ts` — 标签断言更新与新增：目录内
  `vllm` 的 `localhost:11434` 仍命中目录（目录优先）；未注册自定义网关回退主机名
  （含端口）；非法 URL / 空 baseUrl 回退原始 provider；注册表 displayName 命中。
- `ui/src/components/settings/ModelSettingsCard.tsx`（设置页「模型」Tab）：
  - 左栏数据源换合并视图：非搜索态 = 目录可见 → 自定义行 → 「更多供应商」折叠；
    搜索态 = 合并检索平铺（自定义命中带标记混排）；
  - 自定义行：行尾 muted「自定义」pill；hover 显示 编辑(Pencil)/删除(X)
    （`opacity-0 group-hover:opacity-100`，SessionDrawer 惯例）；`ProviderRow`
    增加可选 `actions` 渲染——有动作时行根改为 `div.group > button(选择) + 动作`
    结构，避免 button 内嵌 button 的非法 HTML；
  - 左栏底部常驻虚线行「＋ 新增自定义供应商」→ 新建 Dialog；
  - 新增/编辑共用 Dialog（ui/dialog）：显示名 / 运行束(Select) / Base URL /
    默认模型 / 模型列表(textarea)；字段级校验 + 冲突就地报错；编辑预填、标题
    区分「新增/编辑」；删除仅移出注册表，书签与运行配置不受影响；
  - 点自定义行 = 填表单，点其模型 = 填表单 + `addSavedModel` + 立即
    `saveSettings`，与目录模型完全同一路径；
  - 注册表整理（增删改）与移除书签不受 `locked` 影响（本地偏好）；模型行 /
    chip / 切换类操作沿用 `settingsPhase/locked`。
- `ui/src/components/chat/MaskAndModelSwitcher.tsx` — `displayProvider` 复用
  `savedModelVendorLabel`（删除对 `matchProviderEntry` 的直接依赖），触发器与
  「当前配置」块对自定义网关显示可识别厂商名。
- `ui/src/i18n/{zh,en}.ts` — 新增 `settingsModel` 词条：
  `customBadge / addCustomProvider / customDialogTitleNew / customDialogTitleEdit /
  customDialogHint / displayName / bundle / bundleOpenai / bundleAnthropic /
  baseUrl / defaultModel / modelsList / modelsListHint / save / cancel / editAria /
  removeAria / errors.{displayNameRequired,baseUrlRequired,baseUrlInvalid,duplicateBaseUrl}`
  （zh/en 结构同步受 `i18n.test.ts` 强制）。

## 交互契约（沿用既有体系）

- 一个意图一个动作：注册表 CRUD 只动本地 `vivy.ui.customProviders`，永不改运行
  配置；运行配置只由 模型点击 / chip / 顶栏行 / 「保存真实设置」改变。
- 单一权威来源：显示名只存于注册表条目，书签与顶栏经 baseUrl 解析，无派生副本。
- 忙碌 / 只读：切换类操作沿用 `settingsPhase/locked`；注册表整理与移除不受锁影响。
- 无密钥：注册表与快捷列表均不存密钥（vivy 规则：secrets 不属于 UI）。

## 明确不做

- 不做 A/C 方案的「加入快捷列表」按钮：手打组合通过把模型 id 录入注册表的
  模型列表进入快捷流（用户已确认范围 B）。
- 不提供 mock 束的自定义（mock 为内置离线束）；自定义仅 openai/anthropic。
- 不手改生成目录（`provider-catalog.ts` 数据由脚本生成；目录的 `custom`/`vllm`
  占位条目保持不变）。
- 真实后端供应商元数据 RPC 仍归 `docs/TODO.md` §0.1 `UI-PROV-RPC`（本交付为
  UI-local 结构，不关闭该项）。
- 浏览器冒烟：8787 / 3015 被用户 Vivy Studio 调试会话占用，本次跳过自测，由
  用户 Studio 会话代验（见 `verification.md`）。

## 发布说明

无独立发布：随 UI 常规构建发布，`just ci` 已含 ui build，不单写 `release.md`。
