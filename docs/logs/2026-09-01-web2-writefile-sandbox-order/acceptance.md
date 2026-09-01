# Acceptance — WEB-2

## 人怎么看它工作了

沙箱为受限模式（workspace-write，即默认会话模式）时，让 vivy
"在 src/deep/nested/ 下新建 config.json"这类写全新嵌套目录的请求现在正常
完成：文件落盘、diff 出现在审批/Review Center。修复前这类请求一律报
`resolve parent symlinks` 失败——必须先让模型跑 bash mkdir 才能写，体验割裂。

## 边界（不该发生的事）

- read-only 模式仍然拒绝一切写（回归用例断言 ErrSandboxDenied）。
- danger-full-access 模式行为不变。
- 符号链接组件仍然被拒绝（safeWorkspacePath 逐组件 Lstat），建目录动作
  不可能借 symlink 逃出 workspace。
- 提案/审批流（PrepareWriteFile → 人工批准 → 执行复验 + 前置哈希）不变。
