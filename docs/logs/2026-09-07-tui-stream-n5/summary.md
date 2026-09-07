# TUI-STREAM-N5 — durable stream 防御性契约收紧

## Summary

2026-09-04 lifecycle 审计遗留的 P3 五项逐一收口：

1. **`stream.Decode` 身份必填**（`sdk/tui/stream/events.go`）：`subscription_id` 与
   `run_id` 现为必填，缺失即拒绝。durable 服务端恒发两字段；生产唯一消费点
   `sdk/tui/live/events.go` 不受影响。防止外域/半损通知推进本 face 的 cursor。
2. **空 replay 终态订阅即释放**（`internal/rpc/control.go` `streamRun`）：resubscribe
   的 `after_seq` 已覆盖终态记录时，先前的空 replay 会挂死在 bus 上直到 peer Close。
   现在在初始 replay 后检查 `Runs.GetRun`：run 已终态 → 立即返回；run 不存在 →
   立即返回；active 或查询错误 → 保留 live-wait 路径。正确性依据
   `emitTerminal` 的提交顺序不变量：终态事件先落 Journal、后翻转 run 状态，
   因此 terminal 行 ⇒ 终态记录必已可 replay。
3. **移除 `Inbox.Take()`**（`sdk/tui/stream/inbox.go`）：公开 Take 丢弃 replay
   fence 是一个 footgun；生产代码只用 `TakeWithOverflow`/`TakeBatch`，测试改用
   `TakeWithOverflow`，API 不再暴露隐藏 overflow 的入口。
4. **REPL `streamLost` run epoch / 迁入共享 Inbox**：已被后续重构吸收——现
   `Live`（`sdk/tui/live/controller.go`）的 `recordStreamError` 按 subscription
   键隔离并用 `retiredSubscriptions` 挡住陈旧失败，通知统一经共享
   Inbox/Projection。无剩余代码动作。
5. **`Live.Close` 所有权边界**：维持原设计并复核——`Close` 置位 `closed`、
   `subscriptionRequest++`、2s 重试 unsubscribe、取消 ctx、关闭 inbox；
   完全未交付的 subscribe response 依赖外层 peer Close 收尾，`enqueueNotice`
   在 ctx.Done 后直接返回，无泄漏路径。属设计确认而非缺陷。

## Explicitly not done

- 不改变 JSON-RPC 协议形状（订阅提前结束对客户端表现为流终止，durable 客户端
  以终态事件为准，不依赖连接生命周期）。
- `streamFailures` map 的 LRU 上限维持 4 不变。

## Filing

- 看板：`docs/TODO.md` §0.1 TUI-STREAM-N5 行 → §10 完成日志（注明 4/5 两项
  已被重构吸收/确认为设计）。
