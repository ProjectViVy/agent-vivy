# 多分支合入 main 集成记录

## 背景

用户要求把其余开发分支一并合入 main。根树此前已在 `feat/settings-genparams-provider`
(3c2181e) 与 `feat/trajectory-panel` (87980e4) 上；本次收口四个分支：
`feat/network-tools`（含 `feat/tool-polish` 调和）、`feat/execute-timeout`、
`feat/channels-ui`。

合入后 main 顶部（均为本地 merge，未推送 origin）：

```
8944a53 merge: feat/channels-ui
186b321 merge: feat/execute-timeout
49a2ad3 merge: feat/network-tools（含 tool-polish 调和 90f60a3）
3c2181e ui(settings): 生成参数演示收进通用-高级特性
87980e4 ui(dashboard): 中控台轨迹面板
```

## 冲突与决策

### 同主题演进去重（network_search，最关键）

`tool-polish`（base 6a391b0）与 `network-tools`（base 0791a3d）都实现了
`tools.network_search.provider` 首选 provider + 校验 + settings 覆盖层：
同名同职责、实现相近。先在 `feat/network-tools` 工作树内
`merge feat/tool-polish` 调和（12 个冲突文件逐一解决）后，再整体并入 main：

- 保留 tool-polish 独特交付：read_file 1-based 行号（patch 锚点）、
  echo_info 移出默认启用（`config.Default()` 的 `Tools.Enabled` 去掉）、
  filesystem 打磨、无 key 降级说明。
- 保留 network-tools 独特交付：设置「网络工具」真实 tab（NetworkToolsCard）、
  settings/get|update 的 network_search 分区 + 可用性 roster（密钥仅存在性）、
  `api_key_set` 覆盖层、`ui/e2e/network-tools-setting.spec.ts`。
- 去重：删除 tool-polish 在「工具」tab 内联的「网络搜索」卡（规范 UI 为
  network tab 的 NetworkToolsCard）；provider 配置/校验/roster 各保留一份。
- 语义修复：`MaskAndModelSwitcher` 切换模型时透传 `network_search` 偏好
  （settings/update 是整文档替换，不透传会清掉网络搜索设置）。
- 测试并集：`PrefersConfiguredProvider` + `HonorsPreferredProvider` 两个
  首选 provider 测试都保留；availability 测试取两边覆盖并集；
  `filesystem_test.go` 的 `staticReadOps` 补 `ListDir` 桩。

### execute-timeout 与既有能力合并

- `ControlDeps`/`settingsResult`/`settings/update` 载荷同时带 api_key、
  network_search、execute_max_timeout_seconds 三组字段（原来各分支各带一组）。
- `applySettingsOverlay` 依次应用 api_key → network_search → execute 顶。
- 设置→通用「执行超时上限」真实表单保留（含 `settings_overlay_test.go`
  add/add 合并：api_key/空 key/无文档/network_search/execute 五个测试并存）。
- `WelcomeWizard`/`MaskAndModelSwitcher` 保存时透传 network_search 与
  execute 顶，避免互清。
- 模型 Tab 维持 main 的 ModelSettingsCard 体系（execute-timeout 分支的
  表单式模型页被 main 的供应商目录体系取代）。

### channels-ui 升级为真实分区

- 设置新增「通道」真实 tab（ChannelsSettings 等 9 个新组件 + channel-store
  `vivy.ui.channels` + schema 校验）；删除迁移预览 `ChannelsPreview` 与
  `DIVA_CHANNELS`。
- `DIVA_PREVIEW_SECTIONS` 收敛为 `['general','self-evolution','sandbox']`
  （language/compaction/network/channels 均已真实或并入）。
- 「语言」tab 标签改走 i18n 词条（`settings.tabs.language`），随之修正
  `ui/e2e/language-setting.spec.ts`（刷新后英文 UI 下断言 'Language' 标签）。

## 未做（显式边界）

- 未推送 origin（需用户显式授权）。
- 未修复存量 `UI-E2E-STALE`（runtime.spec.ts:84、welcome-wizard.spec.ts:33
  过期断言，`docs/TODO.md` §0.1 已记录）。
- 通道后端读写、`http_request` 配置面等仍为 §0.1 OPEN 项（UI-CHANNELS-BE、
  UI-NETWORK-HTTP 等），本次只落前端形态。
- 分支调和只发生在 network-tools 分支内（90f60a3）；其余分支原样并入。
- 生成参数演示维持「通用→高级特性、按模型下拉编辑」的最终方向，未回退。