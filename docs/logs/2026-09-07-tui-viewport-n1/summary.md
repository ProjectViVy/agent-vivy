# TUI-VIEWPORT-N1-OPEN — 打开态视口收口（锚定 + 渲染缓存 + 加载态）

日期：2026-09-07 · 范围：`sdk/tui/surface`、`sdk/tui/live`、`sdk/tui/view`

## 变更内容

三个正交收口，全部落在打开态（session 已激活）的聊天视口：

1. **暂停视口锚定（消息锚点）**
   - 停止跟随（PgUp / 滚轮 / PgDn）时，视口顶部的位置不再记为纯数字偏移，
     而是捕获为 `(segment, line)` 锚点 + 当时的 assembly 指纹
     （`chatAnchorSeg / chatAnchorOff / chatAnchorStamp`，model.go）。
   - `clampChatScroll` 在 assembly 指纹变化（内容变更）时把锚点重新映射为
     当前平面偏移：锚点上方（如工具结果卡片展开）增删行不再让视口漂移；
     指纹未变时（纯 clamp）不覆盖显式写入的数字偏移
     （保住 `TestChromeScrollHintHiddenNearBottom` 的直接定位语义）。
   - 锚点随 `Home` / `End` / 底部跟随 / 会话切换 正确置位与清除。

2. **指纹化分段渲染缓存（chatAssembly）**
   - `chatSegments`（render.go）把每条消息渲染为独立 segment（分隔行内建在
     除最后一段外的所有段中），整体以 maphash 指纹（ID/Role/Content/Tool 全
     字段/Streaming/Reasoning/Attachments/FileContexts + 消息数）判定复用；
     `renderChat` 只把可见窗口 slice 出来 join，不再每帧重渲染 + 全量 flatten
     整个历史。mdCache（TUI-MD-STREAM-CACHE）在重建路径上继续兜住单消息渲染。
   - Model 为值接收者：assembly 指针在 `New()` 分配一次、原地更新，值拷贝共享缓存。

3. **历史加载态（Meta.Loading）**
   - `surface.Meta` 新增 `Loading bool`，Live 在 delete→switch→load 窗口期
     （`loadPending`）置位；空会话视图此时渲染“正在加载会话历史…”行，
     不再冒充空对话 hero（“寻找真心之旅”）。

## 明确未做

- 未改投影顺序/来源列（TUI-PROJECTION-ORDER 仍在板上，故意最后做）。
- 未动侧栏 MCP/LSP 状态行（TUI-SIDEBAR-N1-OPEN 另行处理）。
- 未新增增量渲染（脏段重渲染）；指纹全量比对已是 O(历史) 而非 O(渲染)，足够。

## Eino 能力核对

本切片为纯 TUI 表现层（surface/live/view），不涉及 LLM runtime、编排、
提示词、流式管线或上下文管理，无 Eino 复用点，亦不触及 import 隔离边界。
