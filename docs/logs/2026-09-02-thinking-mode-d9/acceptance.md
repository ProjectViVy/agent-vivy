# Acceptance — 思考模式端到端接线

## 怎么向人证明它工作

1. **门控（无 provider 的默认环境）**：打开 `http://127.0.0.1:3015` 聊天页，
   新建会话 —— 工具栏**没有**思考模式灯泡按钮（D9 门控隐藏），附件按钮仍在。
   `ui/e2e/thinking-gate.spec.ts` 常驻回归此行为。
2. **门控打开（Anthropic 思考代际模型）**：Settings → Model 选 anthropic
   bundle、模型切到 `claude-sonnet-4-5`（或任一 3.7+/4+ 代际），回到聊天页 ——
   工具栏出现思考模式选择器；选「开启」后发送消息，模型可返回思考内容
   （`model.reasoning_delta` 事件，TUI/气泡已有渲染）。
3. **参数真的发出去**：内核级证明见
   `internal/provider/resolving_thinking_test.go` —— 本地 Anthropic 形状服务器
   断言 outbound JSON 携带 `thinking: {"type":"enabled","budget_tokens":4096}`
   （仅 on + 支持代际；auto/off/未知模型一律无 thinking 键）。
4. **无效值拒绝**：`turn/start` 带 `thinking:"execute"` → InvalidParams
   （`TestTurnStartThinkingRoute`）；运行前拒绝，Journal 无残留 run。

## 边界行为

- 排队消息保留各自的思考偏好，轮到时按入队时选择发送。
- 旧客户端不传 thinking → auto → 行为与之前逐字节一致。
