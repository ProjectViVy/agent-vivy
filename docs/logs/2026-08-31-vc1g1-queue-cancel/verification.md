# verification — VC-1g-1

命令均在 worktree `agent-vivy-vc0`（分支 `feat/vc1a-bash-tool`）执行。

| 命令 | 结果 |
| --- | --- |
| `pnpm typecheck`（ui/） | exit 0 |
| `pnpm test`（ui/） | 24 files / **194 passed**（含新增 4 条队列测试：忙时入队、完成后派发、失败保留、切换会话清空） |
| `just ci` | **exit 0**（日志 0 个 FAIL 行；go test 26 包 + ui typecheck/test/build 全绿） |

新增测试点（`ui/src/lib/store.test.ts`）：

1. 运行中 `startRun` 不调用 `api.startTurn`，消息进入 `queuedMessages`（text/mode 正确）。
2. `run.completed` 事件后（terminal 刷新完成）自动派发队首：`api.startTurn('s1', 'second message', 'normal', undefined)`，队列清空。
3. `run.failed` 事件后队列**保留**且不派发，`runError` 正常展示。
4. `selectSession` 切换会话后队列清空。

Smoke 说明：队列/两段式取消的完整 UI 走查需要「正在运行的回合」，而运行回合需要
真实 provider key（TEST-1 之后本仓库无 mock provider）。组件与 store 行为已由上述
单元测试 + typecheck 覆盖；按 `docs/logs/2026-08-31-vc1f-ui-diff/verification.md`
记录的同一 smoke-policy 例外处理：真实 provider 环境下的手动走查留给有 key 的
运行时，验证路径为「运行中发送 → 队列 pill 出现 → 完成后自动派发；esc/停止按钮
两段式」。
