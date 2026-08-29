# CH-C9 — A2A / NeuroLink（DEFERRED 备忘，不是开工令）

## 1. 身份

| | |
|---|---|
| ID | CH-C9 |
| 状态 | **DEFERRED** — 各需独立能力提案 |
| 合同 | §15、§15.1；演进阶段 H |
| Eino | 偷 `eino-ext/a2a` 的 models/transport；禁止 `RegisterServerHandlers` |

未提案授权前 **不要领取实现**。

## 2. 目标（将来）

两者都是 ChannelHost 上的重量级插件，不是新内核，不是 Face，不是 ACP。

**NeuroLink：** 本机 WS server；grant `channel.listen`；Host 拥有 bind，默认 loopback；不进 telegram 式必填字段卡。

**A2A：** 北向互操作；grant `channel.a2a`；HTTP+JSON 默认关；`taskId` = `run_id`；请求进 `Service.Run`。

叠法：

```text
A2A JSON-RPC     ← eino-ext/a2a models + transport
    ↓
plugins/a2a      ← 独立 go.mod；只做编解码
    ↓
ChannelHost      ← 已有
    ↓
Service.Run      ← 已有 ADK Runner
```

## 3. 现状

信封槽已在 C2 定形。Listen 面 C3 已声明。五个聊天插件不得占用 `channel.a2a` / `channel.listen` grant。

## 4–8. 禁止（即使将来开工）

- `RegisterServerHandlers(adk.Agent)` 当 Vivy 网关。
- 第二套 TaskStore。
- Listen 挂 `:8787` `/rpc`。
- 默认身体 import `eino-ext/a2a`。
- 设置页在插件未编进身体时列出「添加 NeuroLink」。
- 远程返回当可信工具输出。

## 9. 风险

eino-ext/a2a 仍是 alpha，且声明的 eino 版本与 Vivy pin 不对齐。只能当 codec 对照，不能 drop-in 示例服务器。

## 10. 交接

各写一份能力提案后再开切片 PLAN。不要从本备忘直接开写代码。
