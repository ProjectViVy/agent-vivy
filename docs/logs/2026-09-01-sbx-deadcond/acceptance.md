# Acceptance — SBX-DEADCOND

## 人怎么看它工作了

配置 `sandbox.mode: danger-full-access`（完全信任模式）时，即使白名单放开，
以下类命令仍被拒绝并返回 "command pattern is too dangerous even in
full-access mode"：

- `rm -rf /`、`rm -fr /*`、`rm -r /`、`rm --recursive --force /`（任何旗标组合的
  系统根递归删除）
- `rm -rf .` / `..`（当前/上级目录整体递归删除）
- `rm -rf ~`、`rm -rf C:\`、`rm -rf C:\*`（home、盘符根）
- 带 `--no-preserve-root` 的任何 rm
- Windows：`del /f /s /q *`、`del /s C:\`、`rd /s /q .`

同时不被误伤的正常用法保持可用：`rm -rf build`、`rm -rf node_modules`、
`rm -f file.txt`、`del /q file.txt`、`rm -rf c:/temp/x`（具体子目录）等。

## 边界

- 该防线只覆盖 execute/commandline 路径的 danger 模式；bash 工具路径由其
  既有 deny 表覆盖（未变更）。
- confined（workspace-write/read-only）模式不受影响——本就不靠这条防线。

## 修复前对照

原实现的单参数交叉条件永假，`rm -rf /` 可通过 danger 模式校验直接执行——
本切片后由测试钉死（`TestIsDangerousCommandRootDeletion`）。
