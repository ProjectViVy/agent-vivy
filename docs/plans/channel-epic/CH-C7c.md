# CH-C7c — `plugins/discord` 文本（无 voice）

## 1. 身份

| | |
|---|---|
| ID | CH-C7c |
| 阶段 | G 国际补齐 / 本期关门 |
| 人日 | 2 |
| 里程碑 | M-CH4 |
| 依赖 | CH-C3；建议 C4 已合入 |
| 分支 | `feat/channel-c7c` |
| 合同 | §14.3 discord；禁止 voice.go / pion / TTS |

## 2. 目标

独立 `plugins/discord`。DM / 文本频道文本 + Message Content Intent。默认 EXE 无 discordgo。本期关门切片。

## 3. 现状

**备注：** 正式写 Discord 适配器前，先读 picoclaw——通道实现里它是**最完整**的 Go 样本。只读改写，禁止 import。对照：`.workspace/picoclaw/pkg/channels/discord` 或 `C:\Users\Administrator\Desktop\morediva\.workspace\picoclaw\pkg\channels\discord`。picoclaw 含 `voice.go` / pion——**那些文件不要移植**。详见 `00-standing-orders.md`。

## 4. 目标结构

抄 C4。grant `channel.poll` + `secret.read`。token_env。不实现 slash command 全家桶。

## 5. 文件清单

**建** `plugins/discord/**`。禁止 pion/webrtc、voice.go、TTS 探测、物种 go.mod 加 discordgo。

## 6. 步骤

1. 改写 Gateway WS 文本路径。显式不 copy voice。
2. verify 可扫 `pion` import 作为失败（建议加）。
3. pack --with discord。
4. `just ci` 默认路径。
5. log `docs/logs/YYYY-MM-DD-channel-c7c/`。TODO M-CH4 关门备注。

## 7. 验收

- 候选 DM/文本频道文本。
- 依赖图无 pion。
- 空 allow_from 拒绝。

## 8. 禁止

- `voice.go`、WebRTC、slash 全家桶、TTS。
- 公网 webhook。

## 9. 风险与回滚

- discordgo 易把 voice 当默认 example：评审 diff 盯 import。
- 回滚：配方不点名。

## 10. 交接

本期实现关门。后切读 [CH-C8.md](CH-C8.md) / [CH-C9.md](CH-C9.md)（备忘，不是开工令）。

> **DONE 2026-08-30** — 分支 `feat/channel-c7c`（基于 c7b）。本期 C1–C7c 全部落地，M-CH4 关门：默认身体 `Register()=nil` 零平台 SDK；五耳各自独立 module；pion 封禁入 verify。后切开工需用户点名（C8）或能力提案（C9）。Filing: `docs/logs/2026-08-30-channel-c7c/`。
