# 超级通道合同：Eino 原生 A2A 澄清 — 验收

人读合同能回答「还能按 Eino 原生来吗」：

| 句子 | 出处 | 期望 |
|---|---|---|
| 五个聊天插件不挡 ADK 循环 | §15.1、§21.7 | PASS |
| 原生 A2A 协议可复用 `eino-ext/a2a` 的 models/transport | §15.1 表 | PASS |
| `RegisterServerHandlers(adk.Agent)` 不是产品路径 | §5.2、§15.1、§19 | PASS |
| 后切执行路径是 ChannelHost → `Service.Run` | §15.1 叠法、C9 | PASS |
| 默认身体不得 import `eino-ext/a2a` | PLUGIN-SPEC 禁止项 | PASS |

无用户可见运行时行为。浏览器冒烟不适用。
