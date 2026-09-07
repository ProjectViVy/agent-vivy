# Acceptance — TUI-STREAM-N5

## 一个人如何确认它生效

1. 正常会话流式输出、审批、取消行为不变（回归面；全部 TUI 测试 + just ci）。
2. 对一个已完成 run 的 `run/subscribe` 携带足够大的 `after_seq`：订阅立即结束
   （服务端 subscriptions map 计数归零），不再悬挂连接。
3. 对不存在 run id 的 `run/subscribe`：订阅立即结束，无悬挂。
4. 行为无用户可见 UI 变化——本切片是协议/资源收口，纯防御性。

## 回归风险

- 订阅提前终止对客户端表现为通知流静默结束；durable TUI 面以 `run.completed`
  等终态事件驱动，不依赖连接保持，两套 Live 均不在终态后重订阅。
