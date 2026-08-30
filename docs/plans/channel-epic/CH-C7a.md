# CH-C7a — `plugins/feishu` 单聊文本 WS

## 1. 身份

| | |
|---|---|
| ID | CH-C7a |
| 阶段 | F |
| 人日 | 2 |
| 里程碑 | M-CH3 |
| 依赖 | CH-C3；**建议 CH-C4 已合入**（ABI 样板） |
| 分支 | `feat/channel-c7a` |
| 合同 | §14.3 feishu；实现是 WS，文档里的公网 webhook 不做 |

## 2. 目标

独立 `plugins/feishu`。飞书/Lark 单聊文本，出站 SDK WS。`is_lark` 域名开关放 settings。64-bit only；32-bit 必须编译失败且错误明确。默认 EXE 无 lark SDK。

## 3. 现状

picoclaw `pkg/channels/feishu`。合同明确不做 32-bit stub、表情、公网 webhook 模式。

## 4. 目标结构

抄 C4 目录。`encrypt_key` 放插件 settings，Host 不解码。grants：`channel.poll` + `secret.read`。凭据 env：`app_id` / `app_secret`。

## 5. 文件清单

**建** `plugins/feishu/**`。禁止物种 go.mod 加 lark；禁止 Host 认识 encrypt 算法。

## 6. 步骤

1. 独立 module；改写 WS 事件循环。
2. `is_lark` settings。
3. verify + pack --with feishu。
4. 文档/测试注明 64-bit。
5. `just ci` 默认路径。
6. log `docs/logs/YYYY-MM-DD-channel-c7a/`。

## 7. 验收

- 候选单聊文本；默认无 lark。
- 空 allow_from 拒绝。
- 无公网 webhook 实现。

## 8. 禁止

- 32-bit 兼容层。
- `channel.webhook` grant。
- 表情 / 媒体（后切）。

## 9. 风险与回滚

- 飞书 SDK 肥：必须独立 go.mod，这是硬验收。
- 回滚：配方不点名。

## 10. 交接

[CH-C7b.md](CH-C7b.md)。

> **DONE 2026-08-30** — commit 12a2a70 落于 feat/channel-c6 线（未单独切分支）。SDK 选型样板：飞书 WS 必须 oapi-sdk-go/v3 ≥ v3.11.0（旧版生命周期破损）；`*_env` 双密钥 + `encrypt_key`/`is_lark` 走插件 settings。Filing: `docs/logs/2026-08-30-channel-c7a/`。
