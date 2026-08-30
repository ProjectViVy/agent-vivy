# CH-C3 — ChannelHost + 假插件 TCK

## 1. 身份

| | |
|---|---|
| ID | CH-C3 |
| 阶段 | C 世界入口 |
| 人日 | 3 |
| 里程碑 | M-CH1 |
| 依赖 | CH-C2 |
| 后继 | CH-C4（样板）；C6/C7* 可分树 |
| 分支 | `feat/channel-c3` |
| 合同 | §7 ChannelHost、§8 能力矩阵、C3 |

## 2. 目标

假 channel 发一条文本 → `channel.inbound` 入账 → `Message(source=channel)` → `Service.Run` → 终态 `Send` 回假适配器。空 `allow_from` 拒绝 `Start`。默认 EXE 仍无真实协议。

这是耳朵扇真正存在的第一刀。可选能力接口必须在本切片 **全部声明**（假插件可不实现）。

## 3. 现状（C1+C2 完成后）

- Message 有出处；有 `channel.inbound` 事件。
- sdk/plugin 有 Channel/Env；Adapt 跳过 channel。
- 无 `internal/channelhost`。
- `internal/app/app.go` 只 `pluginhost.Adapt(Register())` 进工具表。
- `Service.Run(ctx, sessionID, userText)` 可用。

## 4. 目标结构

```text
internal/channelhost/          # 禁止 import eino*
  host.go         StartAll/StopAll；fail-closed
  session.go      (channel, chat_id[, topic_id]) → Session
  dispatch.go     PublishInbound 实现：入账 → Run → 订阅终态 → Send
  capabilities.go 全部可选接口类型 + Discover
  fake/           测试用 Channel；不是 plugins/ 产品

internal/app
  Register() 分流：tool → pluginhost.Adapt；channel → channelhost.Host
  Host 持有 Service 的 Run 回调，不让 channelhost import 整个 runtime 循环细节过深
```

会话：本机 UI Session 与 channel Session **不合流**。新建 Session 时带 sandbox 默认值。

配置：`channels.<name>` 不在 compiled-in 集合 → 启动失败。空 allow_from → 该通道不 Start，记错误，不放行。

## 5. 文件清单

**建** `internal/channelhost/`（含测试与 fake）

**改** `internal/app/app.go` 装配；`internal/config` 启动校验（若 C2 只做了 parse）；`importlint` 确认 channelhost 无 eino。

**禁止碰** `plugins/telegram`；物种 go.mod 平台 SDK；`engine.go` 直接 NewRunner。

## 6. 步骤

1. 建包。Host 依赖接口：Journal、Messages、Sessions、`Run(sessionID, text)`、config 信封。
2. `capabilities.go` 列出演进文档 §5 全部可选接口。Discover 用类型断言。
3. fake Channel：Start 后调用 `env.PublishInbound` 一条文本；Send 记入内存。
4. TCK：
   - 空 allow_from → Start 失败。
   - 非空 allow_from + 发送者不在名单 → 不 Run。
   - 名单内 → journal 有 inbound；Message.Source=channel；Run 被调用。
   - Run 终态 → fake.Send 被调用。
   - 未知配置名 → 启动错误。
5. app 装配：无 channel 插件时 Host.StartAll 为空操作。默认 Register() nil → 行为与现在一致。
6. `just ci`。
7. log `docs/logs/YYYY-MM-DD-channel-c3/`。

## 7. 验收

- 上述 TCK 全绿。
- `internal/channelhost` 无 `github.com/cloudwego/eino` import（importlint）。
- 默认 `just run` 仍无耳朵、不发起 Telegram HTTP。
- 可选接口文件存在且被 Discover 引用，即使 fake 一个都不实现。

## 8. 禁止

- 真协议 SDK。
- Listen 挂 `:8787` `/rpc`。
- Host 认识 `parse_mode` / 飞书 encrypt。
- 空 allow_from 放行。
- `"*"` allow_from。
- 插件持有 `*runtime.Service`。

## 9. 风险与回滚

- Service.Run 与 Host 的生命周期：Run 异步。TCK 必须等终态或注入假 Run。
- 避免 channelhost → runtime → channelhost 环：app 注入 func。
- 回滚：去掉 app 装配即可让耳朵消失。

## 10. 交接

下一 AGENT 优先 [CH-C4.md](CH-C4.md)。C6 可另开 worktree 并行，但必须基于已合入的 Host ABI。从 C4 起正式做通道：先读 picoclaw 对应包（最完整 Go 样本），只读改写、禁止 import。见 `00-standing-orders.md`。
