# CH-C7c — `plugins/discord` 文本，无 voice（summary）· 本期关门切片

日期：2026-08-30。分支 `feat/channel-c7c`（自 `feat/channel-c7b` d5f4506 切出；顺序切片复用同一 worktree）。
PLAN：`docs/plans/channel-epic/CH-C7c.md`。合同：`VIVY-CHANNEL-PACK.md` §14.1（discord 行）。对照 picoclaw discord（**voice.go 未移植**）。

## 做了什么

第四只国际耳朵，本期 M-CH4 关门切片：Discord Gateway WS 的 DM + 文本频道纯文本。

1. **独立 module** `plugins/discord`（**上游** discordgo v0.29.0——picoclaw 用 fork replace，我们不用 fork，README 注明）。
2. **session 接口隔离**：discordgo 无干净的端点注入（`Session.gateway` 未导出、endpoint 是进程级全局）→ 细 `session` 接口（Open/Close/ChannelMessageSend）+ 工厂缝，测试全 fake/回环。settings 仅 `token_env`（单凭据，信封钉名语义与 telegram 同）。
3. **生命周期（discordgo v0.29 源码核实）**：`Open()` 同步走完 READY（坏 token / 4014 intent 拒绝首连即败）；`reconnect()` 无视 Close 无限循环 → `ShouldReconnectOnError=false` + 每次重拨换全新 session；死亡信号 = 合成 DISCONNECT 事件（双重触发真实存在 → `sync.OnceFunc`）；** ear/api 分离**——专用永不开机的 session 走纯 REST `ChannelMessageSendComplex`（回复在耳朵重拨期间仍可用）。Stop 是 latch+cancel+有界等待、从不从 Stop 路径 Close（Open 持锁全握手，Stop 侧 Close 会互斥卡死——qq 同型）。
4. **入站**：DM + guild 文本频道；bot 自环 / 空内容 / 系统与 interaction 类型丢弃；`ReplyTo` 入站捕获（出站本刀忽略）；`Sender = "discord:<id>"`。无去重栅栏（Discord 网关不重投已派发事件——源码核实，README 记录）。类型别名即处理函数（discordgo 的 `handlerForInterface` 类型开关不识别具名函数类型——测试抓出的真 bug，已注释）。
5. **pion 封禁入 SDK verify（CH-C7c §6.2 要求）**：`bannedImportPrefixes` 增加 `github.com/pion/` **全前缀**封禁，对全部 seam 生效（tool 插件也不许）；夹具 `bad-pion-import` 被拒实测。voice.go / webrtc / TTS / slash 全家桶零移植（grep 全零；`ShouldReconnectVoiceOnSessionError=false` 有测试钉住）。
6. **密钥**：`LogLevel` 显式钉 `LogError`（discordgo 会在 LogDebug 打含 token 的 Identify 包——不依赖库默认值，评审 note 落实）。`Message Content Intent` 是特权 intent，README 写明开通前置。

## 明确没做（不做声明）

- voice / WebRTC / TTS / 打字指示 / 反应 / 编辑 / embeds / 媒体 / 论坛与线程管理 / slash 与 interaction 处理——合同禁止或后切。
- RESUME 续传（v0.29 session id/seq 未导出）→ 每次重拨重新 IDENTIFY，重拨间隙事件有界丢失（防卡死优先于零丢失，记录为有意偏离）。
- 群触发策略（可读文本全走 Host allow_from 闸门）。
- 真实 Discord 冒烟未做（无凭据；Intent 前置需开发者面板开通；回滚 = 配方不点名）。

## 门禁与证据

`just ci` exit 0（Go 24 包 + UI 172 测试）；`vivy-sdk verify plugins/discord` ok；`bad-pion-import` 被拒；`pack --with discord` 候选 EXE 链接 discordgo v0.29.0 且 **0 pion**；物种 `go.mod`/`go.sum` 字节不变；`go list -deps ./cmd/vivy` 零 discordgo；插件测试 + `-race` 绿。

## 结构性缺口（登记 §0.1）

`just ci` 不编译/不测 `plugins/*`（fmt-check 的 rg glob 只扫 cmd internal sdk ui；`go test ./...` 不过独立 module 边界）——五只耳朵的 gofmt/vet/test 全靠切片内人工执行。插件数量已到五，建议后继加 `plugin-ci` 配方逐 module 跑（记 CH-C7c-N1）。
