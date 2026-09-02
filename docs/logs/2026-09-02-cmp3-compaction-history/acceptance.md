# Acceptance — CMP-3

人工如何确认本切片生效（开发环境 `just run` + `cd ui; pnpm dev` →
http://127.0.0.1:3015）：

1. 打开/创建一个会话，跑到触发压缩（或直接在 设置 → 通用 → 上下文压缩卡点
   「立即压缩」，前提是会话有足够消息量；无压缩需求时反馈「暂不需要压缩」）。
2. 压缩成功后，卡片下方「压缩历史」块出现条目：运行徽标 + 时间 + 「折叠 N 条消息」
   + 摘要文本（最多三行折叠）。无需手动刷新页面。
3. 空会话场景：「压缩历史」块显示空态文案（无记录）；未打开任何会话时显示
   引导文案。中文/英文切换后全部文案跟随语言，不出现 `settings.compaction.*`
   原始键。
4. 接口面：`POST /rpc` `session/compactions` 带 `{"session_id": "..."}`
   返回 `{"compactions": [...]}` newest-first；未知 session 返回 CodeNotFound；
   不带 session_id 返回 InvalidParams。
5. 回归：既有压缩卡配置保存、「立即压缩」、「刷新占用」行为不变；
   `ui/e2e/compaction-setting.spec.ts` 原规格与新规格都绿。
