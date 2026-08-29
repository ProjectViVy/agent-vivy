# CH-C6 — `plugins/dingtalk` Stream 单聊文本

## 1. 身份

| | |
|---|---|
| ID | CH-C6 |
| 阶段 | F 国内过夜 |
| 人日 | 2 |
| 里程碑 | M-CH3 |
| 依赖 | CH-C3（Host ABI）；建议等 C4 形状 |
| 并行 | 可与 C4 分 worktree |
| 分支 | `feat/channel-c6` |
| 合同 | §14.3 dingtalk；Stream 不是 Octos webhook |

## 2. 目标

独立 `plugins/dingtalk`。Stream WS 单聊文本闭环。`session_webhook` 只进插件 settings / 运行时，不进内核 Config。默认 EXE 无钉钉 SDK。

## 3. 现状

C3 Host 通用信封。picoclaw `pkg/channels/dingtalk` 对照。DIVA：保留 Stream，不要倒退成 webhook 文本机器人。

## 4. 目标结构

抄 [CH-C4.md](CH-C4.md) 目录形状。`vivy-plugin.json`：`transport: poll`（出站 WS 客户端算本批 poll）。grants：`channel.poll` + `secret.read`。凭据：`client_id` / `client_secret` 的 env_key。

## 5. 文件清单

**建** `plugins/dingtalk/**`（独立 go.mod）。

**禁止** 改 Host 认识钉钉卡片；物种 go.mod 加钉钉 SDK；改成 HTTP webhook 机器人。

## 6. 步骤

1. 独立 module；改写 picoclaw Stream 客户端。
2. 入站 PublishInbound；出站 Send 用入站带来的 session webhook（存插件侧）。
3. verify + pack --with dingtalk。
4. `just ci` 默认路径无该 SDK。
5. log `docs/logs/YYYY-MM-DD-channel-c6/`。

## 7. 验收

- 候选单聊文本；默认身体无钉钉依赖。
- 空 allow_from 拒绝。
- 无公网 webhook 模式。

## 8. 禁止

- Octos 式 webhook 文本机器人当基线。
- 卡片 / 媒体（后切）。
- `channel.webhook` grant。

## 9. 风险与回滚

- Stream 协议变更：以能稳收单聊文本为准。
- 回滚：配方不点名。

## 10. 交接

[CH-C7a.md](CH-C7a.md)。不要在本切片改信封槽。
