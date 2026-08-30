# CH-C7b — `plugins/qq` 官方 Bot 文本

## 1. 身份

| | |
|---|---|
| ID | CH-C7b |
| 阶段 | F |
| 人日 | 2 |
| 里程碑 | M-CH3 |
| 依赖 | CH-C3；建议 C4 已合入 |
| 分支 | `feat/channel-c7b` |
| 合同 | §14.3 qq；官方开放平台机器人，不是个人号，不是 OneBot |

## 2. 目标

独立 `plugins/qq`。官方 Bot WS 能稳收的单聊/频道文本。默认 EXE 无 botgo。

## 3. 现状

picoclaw `pkg/channels/qq`。DIVA：不要个人号 / NapCat 外挂。

## 4. 目标结构

抄 C4。凭据 env：`app_id` / `app_secret`。transport poll（出站 WS）。

## 5. 文件清单

**建** `plugins/qq/**`。禁止 OneBot 桥、个人号、把 botgo 写入物种 go.mod。

## 6. 步骤

1. 改写官方 WS；入站 PublishInbound。
2. 范围以「官方 API 能稳收的文本」为准，不发明 Guild 全家桶。
3. verify + pack --with qq。
4. `just ci` 默认路径。
5. log `docs/logs/YYYY-MM-DD-channel-c7b/`。

## 7. 验收

- 候选文本闭环；默认无 botgo。
- 空 allow_from 拒绝。
- 代码与文档均声明：非个人号、非 OneBot。

## 8. 禁止

- 个人号、OneBot、NapCat、大文件 base64、语音。
- 第二种身体（外挂 qq.exe）。

## 9. 风险与回滚

- 官方 API 事件覆盖不齐：缩小范围，不要改 Host。
- 回滚：配方不点名。

## 10. 交接

[CH-C7c.md](CH-C7c.md)。

> **DONE 2026-08-30** — 分支 `feat/channel-c7b`（基于 c7a）。样板要点：botgo 的 ChanManager/token 自启协程不可用（源码核实），自驱 `websocket.ClientImpl` + resume + supervised redial；被动回复靠入站 msg_id（内存窗）；群事件在 botgo v0.2.1 解不出地址，勿尝试。Filing: `docs/logs/2026-08-30-channel-c7b/`。
