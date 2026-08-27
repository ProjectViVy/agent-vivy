# 设置 → 网络工具：适配 EINO 原生网络支持 + 真实设置分区（2026-08-27）

## 变更内容

将 `设置 → 网络工具` 分区从「Agent-Diva 迁移预览」（假数据：bocha/brave/zhipu，
后端并不存在）升级为**真实设置分区**，暴露 Vivy 运行时已有的
[EINO 原生网络工具](/docs/AGENT-VIVY-ARCHITECTURE-V0.md)（`network_search`：
bing/google/duckduckgo/searxng/wikipedia；`http_request`：只读网页抓取，
本次只做状态面）。后端先行、前端后行；按用户要求**只打基础，不做端到端
可用**（不做线上搜索验证、不接 browser-use、不在 UI 输入密钥）。

### 后端（第一阶段，先行）

- `internal/config/config.go`：`Tools` 增加 `NetworkSearch` 配置段（
  `tools.network_search.provider`，白名单 bing/google/duckduckgo/searxng/
  wikipedia，空=自动）；`toolsDoc` mirror + `UnmarshalYAML` 直通；
  `Validate()` 拒绝未知 provider；`Default()` 空 provider。`config.example.yaml`
  补文档注释（只说明环境变量名，不含密钥，D-010）。
- `internal/app/settings/settings.go`：设置文档 `Settings` 增加
  `NetworkSearch.provider`（非密钥，空=沿用 config）；`Validate()` 白名单；
  Save/Load round-trip 覆盖。**保留** api_key 覆盖层字段与语义（tool-polish
  原型删除了它，本迭代不跟进）。
- `internal/runtime/network_search.go`：`NetworkSearchService.SetPreferredProvider`
  ——请求未指定 provider 时优先用首选；首选不可用（缺密钥）自动降级到免密钥的
  duckduckgo/wikipedia，不失败。新增 `NetworkSearchProviderAvailability()`：
  按偏好顺序返回 provider 名单 + 环境变量**存在性**（`os.Getenv != ""`，
  永不读取/返回密钥值，D-010）。
- `internal/app/app.go`：`applySettingsOverlay` 把设置文档的
  `network_search.provider` 叠到 `cfg.Tools.NetworkSearch.Provider`（下次启动
  生效，无热切换）；构建 `searchOps` 后 `SetPreferredProvider(...)`；
  `ControlDeps` 增加 `ConfigNetworkSearchProvider` 供 RPC 回显 config 默认。
- `internal/rpc/control.go`：`settings/get` 增加 `network_search` 分区
  `{provider, config_provider, providers[]}`；`settings/update` 接受并持久化
  `network_search.provider`（保留 api_key 语义）。密钥值永不回传。

### 前端（第二阶段，rebase 到该车道落地后的 main 后实施）

- `ui/src/lib/api.ts`：`Settings` 增加 `network_search` 视图类型 +
  `NetworkSearchProviderInfo`；`SettingsUpdate` 接收 `network_search.provider`。
- 新组件 `ui/src/components/settings/NetworkToolsCard.tsx`：真实网络工具卡，
  走 `useVivyStore`（`settings/get|update`，禁 demo-api）——provider 名单 +
  keyless/configured 徽标 + 环境变量名提示；首选 provider 选择器（自动 + 5）；
  保存时合并现有 provider/base_url/api_key（`customApiKeyFor` 回显覆盖层，
  不清掉模型卡的密钥）；
  `read_only` 时禁用保存。
- `SettingsView.tsx`：`网络工具` 提升为一等分区（无「预览」徽标，深链
  `?tab=network` 可用，`SettingsTab` 含 `'network'`）；`diva-preview-data.*`
  移除 `network` 并更新测试；`DivaSettingsPreview.tsx` 删除被替代的
  `NetworkPreview` 假预览（含 `Globe2`/`NetworkProvider`/`network` 分支）。
- i18n：新增顶层 `networkTools`（zh/en 结构一致，含 provider 名、
  环境变量提示、密钥存在性说明）；删除孤立的 `diva.network.*` 假预览词条。
- e2e：`ui/e2e/network-tools-setting.spec.ts`——深链 `?tab=network` → 真实
  roster（免密钥 2 项「已配置」、需密钥 3 项「待配置」）→ 选 Wikipedia →
  保存 → 刷新保持 → 恢复自动。

## 范围说明（用户要求：打基础，不做端到端）

- **没有做**：真实网页搜索/抓取的线上调用验证；browser automation
  （browser-use 明确排除）；UI 内输入/保存 API Key（密钥仅环境变量存在性
  展示，值永不进 UI/日志/设置文档，D-010）。
- `http_request`（网页抓取）只出现在后端可用性 roster 之外的工具名单里，
  本次未给它建独立 UI 配置面——启停与域名白名单由 `config.yaml`
  `runtime.http_allowed_hosts` 控制，记入 `docs/TODO.md` §0.1 作后续。
- 网络工具配置生效方式与模型 provider 一致：保存到
  `data/agent-home/settings.yaml`，**下次启动生效**，无热切换。
- 并行车道治理：根树当时被「设置-语言/压缩」车道占用，本特性按
  `parallel-worktree-isolation` 在 `../agent-vivy-network-tools` worktree +
  `feat/network-tools` 分支开发，落地走 merge/PR；前端文件在 rebase 到该车道
  落地后的 main（`0791a3d`）后实施。

## 变更文件清单

后端：`internal/config/config.go`、`internal/app/settings/settings.go`、
`internal/runtime/network_search.go`、`internal/app/app.go`、
`internal/rpc/control.go`、`config.example.yaml` 及对应 `_test.go`。
前端：`ui/src/lib/api.ts`、`ui/src/components/settings/NetworkToolsCard.tsx`（新）、
`SettingsView.tsx`、`DivaSettingsPreview.tsx`、`diva-preview-data.ts(.test.ts)`、
`ui/src/i18n/{zh,en}.ts`、`ui/e2e/network-tools-setting.spec.ts`（新）。