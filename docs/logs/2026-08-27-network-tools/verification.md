# 验证记录（2026-08-27，设置 → 网络工具）

## 执行的命令与结果

开发在独立 worktree `../agent-vivy-network-tools`（分支 `feat/network-tools`，
基于 rebase 后的最新 main `0791a3d`），全程未触碰根树（根树当时由设置-语言
车道占用，符合 `parallel-worktree-isolation`）。

### Go 单测切片（改动包）

```text
go test ./internal/config ./internal/app/settings ./internal/runtime ./internal/rpc   # 全绿
go test -tags vivy_headless ./internal/app                                           # 全绿
```

覆盖新增：config 解析/校验 `network_search.provider`（含未知 provider 拒绝）、
settings round-trip + 白名单校验、runtime 首选 provider 生效 + 不可用降级 +
availability 三态（t.Setenv，无真实网络）、app overlay 网络偏好、RPC
settings/get|update 的 network_search 分区（含错误 provider 拒绝、
provider 名与 roster 断言、api_key 不回传泄漏检查保留）。

### `just ci`（worktree 内，先 `pnpm install --frozen-lockfile` 建依赖）

```text
just ci   # fmt-check → vet → go test ./... → headless-compile → ui-ci[typecheck → vitest → vite build]
```

**通过，exit code 0**。UI 单测 105 passed（15 files），其中
`diva-preview-data.test.ts`（2 tests，断言网络分区已移除）、
`i18n/index.test.ts`（9 tests，zh/en 叶子结构一致，新增 `networkTools`
词条双语对称）通过；vite 生产构建成功（2202 modules）。

### e2e 真实路径（`pnpm e2e -- network-tools-setting.spec.ts`）

webServer 自起 `go run ./cmd/vivy`（E2E_ADDR 127.0.0.1:8799，隔离 mock
workdir，不触达生产 `data/`）：

- 首跑失败一次：`getByRole('option', { name: 'Wikipedia' })` 严格模式冲突——
  「自动（…wikipedia 免密钥）」选项文本含 wikipedia 子串，解析到 2 个元素。
  已修：`exact: true` + 自动项用 `/^自动/`。这是规格选择器问题，不是产品缺陷。
- 复跑 **通过（1.6s，1 passed）**：深链 `?tab=network` → 分区选中、真实卡片
  渲染（DuckDuckGo/Wikipedia 免密钥「已配置」、Bing/Google/SearXNG「待配置」
  + 环境变量名提示）→ 选 Wikipedia → 保存 → 刷新保持 → 恢复自动。

### 浏览器真实路径

按根 AGENTS.md「smoke-for-user-visible-change」在
`http://127.0.0.1:3015` 实走：**本次由 `ui-e2e` 的 Playwright 真实路径承担**
——webServer 自起 `go run ./cmd/vivy`（E2E_ADDR 127.0.0.1:8799，隔离 mock
workdir），浏览器经 Vite 代理 WebSocket 直连该真实后端，覆盖与本迭代 UI 完全
相同的代码路径（real RPC settings/get|update + NetworkToolsCard 渲染）。

补充说明：worktree 内另起后端 `127.0.0.1:8798`（VIVY_CONFIG=config.dev.yaml）
验证 `/rpc/bootstrap` 可握手（返回 token）；`/rpc` 为 WebSocket-only，
HTTP POST 直连被拒（400）属预期（与 UI 无关）。根树的 :8787/:3015 由并行
车道（chat-toolbar 等）的 dev 服务器占用且不带本次改动，故不以其作为本特性
smoke 依据。

## 验证结论

- `just ci` 全绿；网络工具 e2e 真实路径（roster → 选择 → 保存 → 刷新保持 →
  恢复自动）通过后即满足「后端先行、前端后行的基础版」验收。
- 未验证项：无真实线上搜索调用（用户明确不要求端到端）；read_only 部署路径
  由既有 RPC 行为保证（SettingsPath 空 → ReadOnly），未单独 e2e。
- 无残留在根树：本迭代所有改动只存在于 worktree 分支，合回 main 需授权。