# TUI-WORKSPACE-OWNERSHIP — workspace RPC 所有权校验

## Summary

`/files` 的 run workspace RPC（`workspace/list`、`workspace/read`）此前对格式合法但
未知的 run id 直接调用 `WorkspaceFiles.Ensure`，会在磁盘上创建目录——控制面从未证明
该 run 属于本实例 Journal。本次把该面收口为 fail closed：

- **控制面预检（`internal/rpc/control.go`）**：新增 `workspaceRunOwner`，两个 handler
  在触碰文件系统前先经 `Runs.GetRun` 证明 run 存在于本 Journal；未知 run 返回
  `run not found`（404 语义，与 `run/get` 一致）；Run store 未接线时同样拒绝。
- **运行时零副作用（`internal/runtime/workspace_files.go`）**：`WorkspaceFiles` 从
  `manager.Ensure`（会建目录）切换为 `manager.Existing`（从不创建）；workspace 尚不
  存在的 run 返回新哨兵错误 `ErrWorkspaceNotFound`，控制面映射为
  `run workspace not found` 404。UI 访问器现在的契约是"绝不创建文件系统状态"。
- **Read 竞态硬化**：`Read` 改为 open 后对句柄 `Stat` 再从同一句柄读内容——即使
  Lstat 与 open 之间路径被换成 symlink，离开函数的字节也来自已校验的常规文件句柄。
  读取改用 `io.LimitReader` 按 byte cap 有界进行（旧实现先整读进内存再截断）。
- **测试**：RPC 层新增 unknown-run fail closed（断言 accessor 零调用）与
  workspace-not-found→404 映射两测试；runtime 层新增未知/畸形 run id
  （控制字符 `\x1b`/`\x00`、`..`、路径分隔符）fail closed 且零目录创建、
  final-component symlink 拒绝两测试。

控制面为 loopback 信任传输、无 per-client 身份，因此"run 属于本 Journal 存在性"
即归属事实；未引入新鉴权概念（避免范围扩张）。

## Explicitly not done

- 未做 workspace/list 的 secret 文件名过滤（project-context 面已有该模式，属另一面）。
- 未引入 per-client RPC 鉴权（协议层无此概念，loopback 信任模型不变）。
- Read 打开瞬间的 symlink-follow 残窗在 POSIX 语义下无法用可移植 API 完全闭合；
  以 Lstat + 句柄 Stat + workspace symlink-free 不变量缓解。

## Filing

- 看板：`docs/TODO.md` §0.1 TUI-WORKSPACE-OWNERSHIP 行 → §10 完成日志。
- 关联：`docs/plans/2026-09-07-mcp-stdio-upstream.md` 无关；本项独立交付。
