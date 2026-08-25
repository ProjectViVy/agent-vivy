# Verification

Date: 2026-08-26

## Gate: `just ci`（仓库根目录）

结果 **EXIT=0**（含用户两次反馈收敛后的复跑）：Go fmt-check / vet /
test / headless-compile、UI typecheck（`tsc --noEmit`）、单测
**57 passed（12 个测试文件）**（含新增 `src/lib/chat-actions.test.ts`
6 例）、`vite build` 成功（仅既存 chunk-size 提示）。

## E2E：`just ui-e2e`

结果 **2 passed（23.2s，含用户反馈收敛后的复跑）**：

- `runtime.spec.ts` 新增断言全部通过：
  - 助手消息功能栏：复制可见、重新生成**启用**、回到这里 / 从此分叉**禁用**；
  - 用户消息（参考 ChatGPT）：操作行默认 `opacity: 0`，`hover()` 后
    `opacity: 1`（悬停浮现锁定为 e2e 契约）；编辑**禁用**，复制存在，
    回到这里 / 从此分叉**不出现**；
  - 剪贴板授权（`grantPermissions(['clipboard-read','clipboard-write'])`）
    后点击复制：按钮切换「已复制」，`navigator.clipboard.readText()`
    回读包含 `mock reply to: hello vivy`。
- `welcome-wizard.spec.ts` 回归通过。

## 真实路径冒烟：http://127.0.0.1:3015（split Vite，真实控制面）

- 功能栏在既有会话的所有消息上渲染：用户消息（复制 + 编辑占位 +
  回退 / 分叉占位）、助手消息（复制 + 重新生成启用）。
- **复制**：点击后按钮切换「已复制」。注意：IAB 内嵌浏览器的 guest
  渲染器拒绝 Clipboard API，靠 `execCommand` 兜底路径成功（该兜底正是
  为此场景引入，已在真实浏览器验证）。
- **重新生成**：点击最后一条助手消息的重新生成，走 preflight →
  turn/start → 流式回复，消息数 6 → 8（重发的用户输入 + 新回答追加，
  符合追加式 Journal 的映射语义）。
- 冒烟中发现并修正：可见 DOM 返回页面坐标而 CUA 点击用视口坐标，
  按钮在滚动区外时点击无效——滚动到视口内后全部命中。
- **第二次收敛复验**（去时间戳 + 编辑按钮移到气泡左侧）：DOM 几何
  断言按钮右缘 1150 ≤ 气泡左缘 1156（`leftOfBubble: true`），垂直中心
  111.4 vs 111.4 完全对齐；快照中仅助手消息带时间戳（16:31/16:32/03:39），
  用户消息无时间戳。

## 跳过说明

- 未单独跑 `go test`（`just ci` 已覆盖；本次无 Go 侧改动）。
