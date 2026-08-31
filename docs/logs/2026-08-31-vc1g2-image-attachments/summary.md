# VC-1g-2 图片附件链路

日期：2026-08-31　分支：`feat/vc1a-bash-tool`（worktree `agent-vivy-vc0`）　任务：VC-1g 第二段

## 范围

用户消息可以携带图片附件，从 UI 到内核多模态输入全链路打通：

- **UI**：ChatInput 回形针选择图片 / 文本框直接粘贴截图（对照 Crush 的贴图交互），
  待发送缩略图可逐个移除；客户端门禁与服务端一致（png/jpeg/gif/webp、单张 5MB、
  每条最多 4 张），越界给出行内提示；运行中发送走 VC-1g-1 队列并携带附件。
- **RPC**：`turn/start` 新增 `attachments: [{name?, mime_type, data(base64)}]`；
  服务端为权威门禁（mime 白名单 / base64 解码 / 5 MiB / 4 张，越界回
  InvalidParams 并带 1 起始序号）。`session/messages` 对用户消息回传
  `attachments: [{name?, mime_type, data_url}]`（服务端拼好 data URL）。
- **存储**：新表 `message_attachments`（message_id / position / name / mime_type /
  data BLOB），SQLite 迁移 18、Postgres schemaV17（全新启动走 bootstrap V15+
  upgrade 到 17）；AppendMessage 事务化写入消息 + 附件；ListMessages JOIN 回填；
  DeleteSession 显式清理附件行。
- **内核**：`runtime.RunOptions.Attachments` 透传到 Journal；`buildRunContext`
  把带附件的用户消息投影为 eino 规范多模态输入
  （`schema.UserInputMultiContent`：文本 part + `image_url` part 的
  Base64Data/MIMEType），无附件路径保持 `schema.UserMessage` 不变。
- **压缩安全**：compaction 摘要的 transcript 保持纯文本，附件渲染为
  `[image attachment: name]` 占位行，摘要器永不接触二进制。
- **字节预算**：附件字节刻意不计入 `ContextPolicy.MaxBytes`（图片按视觉 token
  计费而非文本字节，单张 5MB 图 base64 后 ~6.7MB，计入会误触发
  `ErrContextBudgetExceeded`）。已在 context.go 注释中写明理由。
- **UI 渲染**：用户气泡内缩略图（服务端 data_url 与本地乐观行统一）。

## 明确不做（对照 Crush 边界，不擅自添加）

- **SupportsImages 模型门控**：Crush 用模型元数据在不支持图片的模型上拒绝贴图。
  Vivy 目前没有模型元数据管线，门控归属 VC-2（模型元数据）一并做；
  当前行为：请求会走到 provider，由 provider/上游自行报错。
- 非 image 附件（PDF、文本文件）不在本片（Crush 亦无）。
- tool result 携图 workaround（Crush 的 CLI 无头截图路径）不做。
- 通道（feishu 等）媒体入口不做，本片只覆盖 web UI。

## 法律与对齐

Crush 为 FSL-1.1-MIT：本片只做行为/协议对齐（附件上限、类型白名单、贴图交互、
队列携附件），零代码复制；全部实现基于 eino 原生多模态 API 与自有存储设计。

## 验证

见 `verification.md`。

## 已知限制

- 模型不支持视觉时错误来自 provider（VC-2 落 SupportsImages 门控后前置拦截）。
- 队列 pill 的 chip 只显示文本，附件数量不单独标注（文本必填，chip 不为空）。
