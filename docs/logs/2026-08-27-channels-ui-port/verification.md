# 验证记录

## 执行环境

- 代码：隔离 worktree `../agent-vivy-channels-ui`（分支 `feat/channels-ui`，
  基于 main `8e18293`）；根树未写入，符合并行车道规则。
- 命令均在 worktree 根目录 / `ui/` 下执行。

## 命令与结果

| 步骤 | 命令 | 结果 |
|---|---|---|
| 依赖 | `cd ui && pnpm install --frozen-lockfile` | 通过（pnpm 10.33，365 包） |
| 类型 | `cd ui && pnpm typecheck` | 通过（tsc --noEmit 0 错误） |
| 单测 | `cd ui && pnpm test` | 通过：17 文件 / 138 用例；新增 `channel-schema.test.ts`（19）、`channel-store.test.ts`（14），更新 `diva-preview-data.test.ts`（2）；i18n zh/en parity 校验通过 |
| 构建 | `cd ui && pnpm build` | 通过（vite build，2211 模块；仅 chunk>500kB 常规警告） |
| 全量门禁 | `just ci`（fmt-check / vet / test / headless-compile / ui-ci） | **通过（exit 0）**：Go fmt/vet/test 全绿（internal/runtime、sqlite、rpc、ui 等全 ok），ui-ci（typecheck + 138 用例 + build）通过 |

> 备注：首轮 `just ci` 因与 store 快照稳定性修复（`channel-store.ts` 由每次
> 克隆改为「不可变写入 + 稳定引用快照」，满足 `useSyncExternalStore` 要求）并行
> 竞跑而失败一次；修复后全量重跑通过。该修复同时把旧 tests 中「读侧深拷贝」断言
> 与「下架通道写入」语义对齐（saveChannel 对下架通道为 no-op）。

## 新增测试覆盖

- `channel-schema.test.ts`（19 用例）：平台覆盖与已知性、字段默认值
  （discord / email / 未知平台）、coerce 按类型收敛（boolean / number /
  string-list / text）、`splitIdList` / `joinIdList` 往返、
  `normalizeChannelConfig`（已知字段收敛 + 未知字段与 enabled 保留）、
  必填字段与 `validateConfig`、`fieldsByGroup` 分组。
- `channel-store.test.ts`（14 用例）：localStorage 读写与坏数据过滤
  （坏 JSON / 非对象条目丢弃 / 下架通道读入即隐藏 / 写侧深拷贝）、
  `toggleChannel` / `removeChannel`（下架通道 no-op）、写操作触发自定义事件、
  Discord 读入归一化（gateway_url / intents=37377 / 布尔与列表默认值）、
  `channelStatusFor` 就绪近似（必填齐全 → ready / 缺失列出）、SSR guard。

## 冒烟（用户可见行为，真实 Chromium）

Vite dev（`ui` 目录临时端口 **3016**，`VIVY_BACKEND_ADDR` 指向既有 8787 后端；
3015/8787 被另一并行车道占用故未抢占）。Playwright 脚本实走
`http://127.0.0.1:3016/settings?tab=channels` 全链路：

1. 空态「暂无通道配置」出现 ✓（另：工具栏「添加通道」按钮可见）
2. 向导添加 Telegram：「通道配置向导」→ 平台卡选中 → 快速指引
   「如何获取 Telegram 凭证？」→ 填 Bot Token → （验证教程弹窗：
   「Telegram 配置教程」标题 + 接入方式 Long Polling 概览，可关闭）→
   下一步 → 「配置完成」→ 完成 → 卡片网格出现 Telegram ✓
3. 卡片停用/启用：标题「停用」→「已禁用」→「启用」→「已启用」 ✓
4. 列表视图：左栏选中 telegram → 改 token → 「保存配置」 ✓
5. 刷新页面：Telegram 仍在，`vivy.ui.channels` 中 `telegram.token =
   smoke-token-456`（持久化生效）✓
6. 删除：confirm「确定要删除通道 "telegram" 吗？此操作不可撤销。」
   （`{{name}}` 插值正确）→ 回到空态 ✓

结论：冒烟通过（补充截图见冒烟过程临时产物，已按交付清理规则移除，不随
交付入库）。

## 全量门禁结论

`just ci` exit 0。UI 变更后 Go 侧无对应变更，Go 测试为回归确认。