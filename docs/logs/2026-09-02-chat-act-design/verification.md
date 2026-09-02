# Verification — UI-CHAT-ACT 设计片

日期：2026-09-02。

```text
（地形盘点由只读探索代理完成，36 次工具调用；关键 file:line 引用均落入设计文档）
确认事实源：
  internal/storage/contracts.go:57-64   Journal Append/Replay + ErrRunClosed
  internal/storage/sqlite/sessions.go:107-114  DeleteSession 唯一成块删除
  internal/runtime/service.go:1151-1179 runMessages 读 messages 表（模型上下文）
  internal/runtime/compaction_service.go:168-178 SessionCompaction 标记行先例
  internal/rpc/control.go:475-658      dispatch switch + 相邻方法面
  internal/runtime/service.go:629-631  child run 重启不重执行（fork 不复用的依据）

just ci   （docs-only 片）
  → CI-EXIT:0
```
