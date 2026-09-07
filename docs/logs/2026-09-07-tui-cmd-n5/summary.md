# TUI-CMD-N5 — 动态命令 P2 parity 与资源清理

## What changed

Board row `TUI-CMD-N5` 的 P2 收账（2026-09-05 审查遗留项），按当前单一
fullscreen face 架构（`internal/tui/repl.go` 已被重构吸收，plain REPL
目录仅在启动加载的缺口随之消失；palette 每次打开都刷新目录
`openCommandPalette` → `RefreshDynamicCommands`）逐项落地：

- **展开 RPC 真取消（surface + live + view）**
  - `sdk/tui/surface/surface.go`：新增 `DynamicCommandCanceller` 能力接口
    （`CancelDynamicCommand(request uint64)`），并入 `Driver`。
  - `sdk/tui/live/controller.go`：`Live` 持有
    `dynamicCommandCancels map[uint64]context.CancelFunc`（request 为
    view 私有计数器，一个 Live 只服务一个 view，故 id 唯一）；
    `ExecuteDynamicCommand` 注册 cancel 并在 RPC 结束时注销；
    `CancelDynamicCommand` 幂等，未知/已完成请求为 no-op。
  - `sdk/tui/view/model.go`：Esc 取消 pending 展开时先取消 RPC（编辑器
    立即解锁，不再等满 15s 超时）再 bump request 丢弃迟到结果；active
    session 切换时若 pending 展开属于旧 session 一并取消，取消后的迟到
    msg 走既有 discard 路径（恢复 draft + "discarded after the active
    session changed" 诊断）。
- **session mismatch 与 overlay 优先级钉死**
  - 参数表单记录 `dynamicArgumentSession`（打开 palette 时的 active
    session）；active session 变化时属于旧 session 的表单立即关闭（与
    file completion 的既有 session 守卫一致）。
  - 复核既有键路由优先级已经确定：gate > pending 展开锁 > sessions >
    shortcuts > model picker > command confirm > command overlay >
    palette > 参数表单 > 文件补全 > 编辑器；palette 与参数表单的互斥
    由路由顺序保证（表单持有键盘时 palette 无法重入），无需新增代码。
- **刷新成功清理旧错误**
  - `applyDynamicCommandsMsg` 成功路径清除遗留的
    `dynamic commands: …` 前缀 lastErr（沿用 `session sidebar:` 前缀
    清理的既有模式），目录恢复后不再残留上一次失败的错误横幅。
- **评估后不改**：escaped `<loaded_skill>` 信封。Vivy 的 skill 动态命令
  展开为等价纯文本指令是 2026-09-05 审查接受的有意等价；切到 Crush 的
  XML 信封只会改变 prompt 形状而无行为收益，保持纯文本。

## Tests

- `sdk/tui/live/controller_test.go`
  - `TestDynamicCommandRefreshSuccessClearsStaleCatalogError`：失败刷新
    置错 → 成功刷新清错且目录就位；
  - `TestCancelDynamicCommandAbortsInFlightExpansion`：fakeEnv 记录最近
    一次调用的 context（`callContext`），阻塞中的 `commands/expand` 在
    `CancelDynamicCommand` 后立即以错误返回；重复取消与未知请求为 no-op。
- `sdk/tui/view/command_test.go`
  - `TestDynamicCommandEscapeCancelsInFlightExpansion`：Esc 记录取消、
    恢复 draft，取消后的迟到 msg 被静默丢弃；
  - `TestDynamicCommandSessionChangeCancelsAndClosesArgumentForm`：
    session 切换取消旧 session 的 pending 展开并关闭其参数表单；取消
    msg 迟到时走 discard 路径（draft 恢复 + 诊断）。

## Explicitly not done

- REPL list/expand / 真实 mixed MCP server 端到端测试扩充：现测试面已
  覆盖 typed catalog、展开、能力缺失、boot/refresh 竞态与取消；真实
  MCP server 集成测试归 `MCP-TRANSPORT-1`（上游 mcp-go 组件接入）波次。
- 无 `git push`。
