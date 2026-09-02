# Summary — UI-CHAT-ACT 设计片：Journal 截断/分叉提案

## What changed

- `docs/architecture/JOURNAL-REWIND-AND-FORK.md`（新，proposal）：消息编辑/回退/分叉
  的内核设计。核心：**逻辑截断标记**（新表 `session_truncations`，migration 020，
  不删任何行）+ 读时折叠（压缩 `foldSessionHistory` 先例同族，截断先于压缩复合）+
  `session/rewind` / `session/fork` 两 RPC + 会话忙闸（`ErrSessionBusy`）+ UI 三占位
  启用方案。编辑 = rewind + 既有 `turn/start` 组合，内核零新增。
- 盘点基础（逐 file:line 落在文档内）：Journal 追加式 + `ErrRunClosed`；messages 表
  才是模型上下文事实源（run_events 为审计/订阅轴）；压缩已建立"标记行 + 读时折叠"
  先例；会话级无 per-session 忙闸；child-run 语义与 fork 不同构（重启不重执行）。
- `docs/TODO.md`：UI-CHAT-ACT 行更新（设计片交付，实现 R1/R2/R3 切分 + 3 条公开
  问题，行保持 OPEN）。

## What was explicitly not done

- 实现（migration、Store、RPC、UI、e2e）——设计片即本片交付物；R1/R2/R3 为后续切片。
- 文件回退（RB-1 轴）与会话物理删除——显式非目标。

## Scope

docs-only；零代码。
