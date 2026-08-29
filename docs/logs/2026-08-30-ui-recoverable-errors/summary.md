# 2026-08-30 · 前端可恢复错误

Date: 2026-08-30
Scope: Vivy UI (`ui/`), not Studio.

## What changed

对话失败原先把控制面拒绝、引擎原文和模型缺密钥混成一条红色字符串，启动失败只能整页刷新。这次把失败分类收进共享层，并接到启动页与聊天区。

- `ui/src/lib/failure.ts`：把控制面不可达、API Key 缺失、普通 run 失败分成三种，并给出恢复动作。
- `RpcClient.connect`：`/rpc/bootstrap` 网络失败（`Failed to fetch` / `ECONNREFUSED`）改成可操作文案，不再把浏览器 `TypeError` 原样甩出去。
- store：`retryInitialize` 清 RPC 客户端后重连，不必整页刷新；`run.failed` 的 `payload.message` 进入 `runError`，重开历史失败 run 也会带上。
- 启动页与聊天区用同一套 `RecoverableError`：控制面失败给重试，缺密钥给「打开模型设置」，草稿在发送失败后保留。

## Explicitly not done

- 没有改后端 `run.failed` 分类（引擎仍可能把 `API key missing` 包进 NodeRunError 文本）。
- 没有给每个设置页/演示页统一换成 `RecoverableError`。
- 没有自动重连控制面；重试仍是用户动作。
