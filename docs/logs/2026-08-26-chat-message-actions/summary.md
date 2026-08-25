# 聊天消息功能栏移植总结

Date: 2026-08-26
Status: complete

## Outcome

对照 `agent-diva-gui` 的 `ChatView.vue` `msg-actions`，把 Agent-DIVA 的
聊天消息功能栏移植到 Vivy。按用户反馈收敛为：
**用户消息（蓝色气泡）仅保留时间戳 + 编辑（禁用占位）**；
助手消息为时间戳 + 复制（启用，带「已复制」反馈）+ 重新生成（启用）+
回到这里 / 从此分叉（禁用占位）。

## Delivered

- `ui/src/lib/chat-actions.ts` — `regeneratePrompt` 纯函数：目标助手消息
  之前最近一条 user 消息内容；`chat-actions.test.ts` 6 个单测（邻近选择、
  跳过 tool/system、无前置 user、非助手目标、id 不存在、id 复用）。
- `ui/src/components/chat/MessageBubble.tsx` — 功能栏 UI：
  - 用户消息：气泡下方复制 + 编辑（禁用占位，title 带「待实现」）
    按钮，`opacity-0 group-hover:opacity-100` 悬停浮现（参考 ChatGPT），
    无时间戳；
  - 助手消息：时间戳（`dateTimeLocale()` 本地化 HH:mm）+ 复制
    （Clipboard API 优先，受限 webview 拒绝时退回隐藏 textarea +
    `execCommand('copy')`，成功后按钮切换「已复制」1.5 秒）+ 重新生成
    （`actionsDisabled`（运行/预检中）或无前置用户消息时禁用）+
    回到这里 / 从此分叉（`disabled` 占位，与 Agent-DIVA 一致）；
  - 流式气泡（streaming）不渲染功能栏，落盘后再出现。
- `ui/src/components/chat/ChatView.tsx` — 接线：`regenerate` 用
  `regeneratePrompt` 取输入并复用既有 `submit`（preflight → startRun），
  不绕过预检产品契约。
- `ui/src/i18n/zh.ts` / `en.ts` — `chat.copy / copied / edit /
  regenerate / rewind / fork / pending` 词条（结构一致）。
- `ui/e2e/runtime.spec.ts` — 首条回复后断言功能栏存在性与禁用态（用户
  消息只余编辑、无复制/回退/分叉），并授权剪贴板后点击复制、断言
  「已复制」与剪贴板回读内容。

## 用户反馈收敛（同日）

首版把五个操作同时放在用户与助手消息上；经用户三轮反馈收敛（用户侧
交互参考 ChatGPT）：

1. 蓝色用户气泡只保留「编辑」+「复制」，回到这里 / 从此分叉从用户
   消息移除；
2. 去掉时间戳；
3. 按钮放在**气泡下方**、平时 `opacity-0` 隐藏，悬停消息行才浮现
   （保留布局位避免悬停跳动），与 ChatGPT 行为一致。

助手消息保持完整功能栏（时间戳 + 复制 + 重新生成 + 回退 / 分叉占位，
`opacity-60` → 悬停全显）。

## 语义映射（与 Agent-DIVA 的差异）

Agent-DIVA 的重新生成会**截断**目标助手消息之后的历史并就地覆盖；
Vivy 的 Journal 是追加式事实源，控制面没有消息截断 / 分支 RPC，因此
「重新生成」映射为：用目标助手消息之前最近一条用户输入**重新发起一轮
对话**（旧回答保留，新回答追加）。这是当前 RPC 面下最诚实的映射；
真正的回退 / 分叉 / 就地编辑需要内核 Journal 能力，已在
`docs/TODO.md` §0.1 登记（UI-CHAT-ACT）。

Agent-DIVA 的流式重试 / 停滞徽标（`chat.retrying` / `chat.stalled`，
依赖 `msg.retryStatus` 字段）未移植：Vivy 的 run 事件契约不暴露该字段，
等内核提供后再接。

## Explicitly not done

- 未新增任何后端 RPC；未触碰 Journal 语义。
- 未实现消息编辑、回退、分叉（占位禁用，同 Agent-DIVA）。
- 未移植 artifact 引用复制（`copyArtifactReference`）——Vivy 工具消息
  渲染不含 artifact 引用结构。
