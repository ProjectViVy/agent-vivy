# 2026-08-27 · 「已选模型」快捷切换（Agent-Diva savedModels 移植）+ 模型交互理顺

## 变更内容

把 Agent-Diva 的「已选模型」快捷切换移植到 Vivy 前端：快捷列表唯一存储于
本机 localStorage，设置页与顶栏同源消费；模型选择统一为「一个意图一个动作」
——点击即选用并保存，移除只动本地列表，手改表单仍是显式提交。

三个决策点按推荐项执行：

1. **设置页点模型 = 立即选用并保存**（diva 式一步到位），不再只是填表单；
2. **顶栏下拉只显示已选模型**，删除"当前厂商全部目录模型"段——浏览目录归
   设置页；
3. **设置页新增「已选模型」管理区**（对应 diva 的"选定模型 / 供应商列表
   两种情况"）。

### 新增

- `ui/src/components/settings/saved-models.ts` — 快捷列表持久化模块（仿
  `mask-catalog.ts` 的同款样板：模块级缓存 + `useSyncExternalStore` +
  自定义事件 / `storage` 事件广播，不进 zustand store）：
  - `SavedModelEntry = { provider, baseUrl, model }`：运行三元组，无密钥、
    无冗余 displayName——显示名渲染时经 `savedModelVendorLabel` /
    `matchProviderEntry` 解析（目录命中 → 厂商 displayName，否则回退原始
    provider），单一权威来源；
  - localStorage key `vivy.ui.savedModels`（真实功能，禁用 `vivy.demo.*`）；
  - `getSavedModels`（SSR guard + try/catch + 逐条字段校验过滤坏数据）、
    `addSavedModel`（按 triple 去重、尾部追加、无上限，与 diva 一致）、
    `removeSavedModel`（按 triple 过滤）、`useSavedModels()` hook。
- `ui/src/components/settings/saved-models.test.ts` — 16 个 vitest 纯逻辑用例：
  triple 去重与追加顺序、remove、坏 JSON / 缺字段条目过滤、label 解析
  （目录命中 / 自定义网关回退）、localStorage 写回、自定义事件与
  `storage` 跨标签页广播、SSR guard。

### 修改

- `ui/src/components/settings/ModelSettingsCard.tsx`（设置页「模型」Tab）：
  - 双栏 grid 上方新增「已选模型」区：平铺 chips（`厂商 · 模型`），点击
    chip = 立即 `saveSettings(triple)`（与顶栏快捷切换同语义），行内
    X = `removeSavedModel`（本地偏好随时可删，不受锁影响）；空态提示
    "在下方供应商列表点击模型加入"；
  - 供应商模型行点击升级：填表单 + `addSavedModel`（幂等）+ 立即
    `saveSettings`；忙碌沿用 `settingsPhase/locked`，错误沿用现有
    `settingsError` 段；
  - 模型行尾部状态标记：`Check` = 当前运行配置；非当前但已加入 →
    muted `Bookmark` 图标（title=已加入快捷列表）；
  - 三输入框手改 + 「保存真实设置」按钮保留（自定义组合的显式提交边界
    不变）。
- `ui/src/components/chat/MaskAndModelSwitcher.tsx`（顶栏）：
  - `ModelMenu` 数据源换 `useSavedModels()`；删除 `MODEL_DESCRIPTION_KEYS`
    与"当前厂商可选模型"目录段（浏览归设置页）；
  - 菜单结构：当前配置块 → 分隔 → 「已选模型」rows（过滤当前 triple 防
    重复；title=模型 id、subtitle=厂商名；点击 = `saveSettings` 立即切换）
    → 空态文案 → 分隔 → 管理入口
    `navigate({ to: '/settings', search: { tab: 'model' } })`；
  - 行内移除：X hover 显示（`opacity-0 group-hover:opacity-100`，
    SessionDrawer 既有样式惯例），`onSelect`/`stopPropagation` 保持菜单
    打开且不触发行切换；
  - **不移植** diva 的"移除即清空运行配置"副作用：移除书签不改 live
    config。
- `ui/src/i18n/{zh,en}.ts` — 新增词条（结构同步受 `i18n.test.ts` 强制）：
  `settingsModel.savedTitle/savedEmpty/added/removeSaved`、
  `maskSwitcher.savedModels/noSavedModels/removeSavedAria`；删除已随
  目录段移除而失效的 `maskSwitcher.optionalModels` 与 `maskSwitcher.models.*`。

## 交互契约（oil-frontend 对齐）

- **一个意图一个动作**：模型行 / chip / 顶栏行点击 = 选用即生效；移除 =
  只动本地列表；手动编辑 = 显式保存。
- **忙碌范围**：请求中锁定切换（沿用 `settingsPhase/locked`），移除不受锁
  影响、不阻塞。
- **同一数据单一来源**：快捷列表唯一存储于 `saved-models` 模块，设置页与
  顶栏同源 `useSavedModels()` 消费。
- **空态 / 只读态**：顶栏与设置页各有空态文案；`read_only` 部署禁切换、
  允许整理本地列表（移除按钮不受只读影响）。

## 明确不做

- 顶栏"当前厂商全部目录模型"浏览（决策 ② 移除，浏览归设置页）。
- diva 的移除即清空运行配置副作用（书签与 live config 解耦）。
- 列表上限与排序编辑（按 diva 语义：无上限、追加序）。
- 浏览器冒烟：8787 / 3015 被用户 Vivy Studio 调试会话占用，本次跳过自测，
  由用户 Studio 会话代验（见 `verification.md`）。
- 新增 Playwright e2e（本次以 vitest 纯逻辑 + 用户 Studio 代验覆盖）。

## 发布说明

无独立发布：随 UI 常规构建发布，`just ci` 已含 ui build，不单写
`release.md`。