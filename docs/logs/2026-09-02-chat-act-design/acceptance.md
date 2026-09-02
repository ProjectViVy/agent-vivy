# Acceptance — UI-CHAT-ACT 设计片

人如何确认生效：

1. 打开 `docs/architecture/JOURNAL-REWIND-AND-FORK.md`：§1 三动作语义表、§2 截断
   标记与折叠、§4 RPC 合同、§7 R1/R2/R3 切分、§8 三条公开问题齐备。
2. 设计判据（可追问的检查点）：
   - 为什么不物理删除？→ §1.2(1)(2)：Journal 追加式 + messages/run_events 双事实源。
   - 为什么截断能不删行还省钱？→ §2.1 标记行只存 id；§2.3 被截消息/事件留档可审计。
   - 为什么 fork 复制而非引用？→ §3.3 两会话独立演化，避免跨会话读穿透。
   - 为什么不用 child-run 表达 fork？→ §1.2(5)：重启不重执行 + 生命周期不同构。
3. `docs/TODO.md`：UI-CHAT-ACT 行显示设计片已交付 + 实现切分，行状态 OPEN（未实现）。
4. UI 不变量：MessageBubble 的编辑/回退/分叉按钮仍为禁用占位（本片零代码）。
