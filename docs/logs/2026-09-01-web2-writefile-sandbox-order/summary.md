# WEB-2: WriteFile sandbox 校验先于 MkdirAll 的受限模式误拒

日期：2026-09-01 ｜ 分支：`feat/vc1a-bash-tool` ｜ worktree：`agent-vivy-vc0`

## What changed

`internal/runtime/filesystem_backend.go` 的 `EinoFilesystemBackend.WriteFile`
在 MkdirAll 之前就跑 `ValidatePathWithMode`。该校验对"目标文件不存在"已有
回退（用父目录 EvalSymlinks），但父目录本身不存在（全新嵌套目录）时
`EvalSymlinks(parent)` 失败 → `"resolve parent symlinks"`——workspace-write
模式下 `write_file` 到任何新嵌套目录都被误拒（danger 模式短路校验，所以
既有测试全绿没暴露）。

修复 = 采用 download.go（`internal/runtime/download.go:100`）已自修并注释过
的同款顺序「resolve → MkdirAll → Validate」：

1. `resolve()` 先行：`safeWorkspacePath` 逐组件 Lstat，拒绝符号链接组件并
   把路径钳制在 workspace 内——先于任何建目录动作（download.go 注释同款
   论证）。
2. `CreateParents` 时 MkdirAll + 复跑 resolve。
3. sandbox 校验移到父目录存在之后、atomicWrite 之前。

行为边界（有意为之）：

- 同内容 no-op 写（`bytes.Equal` 早退）不再过 sandbox 校验——零字节落盘、
  零变更，读-only 模式拦一个 no-op 不是安全属性。
- `CreateParents=false` 且父目录缺失时错误仍为 sandbox 解析错误（与修复前
  一致，随后 atomicWrite 也会失败）。
- symlink 逃逸防护不变：resolve() 的逐组件 Lstat 在建目录前就拒绝 symlink
  组件，MkdirAll 无法借道逃逸；`ValidatePathWithMode` 的 EvalSymlinks 仍作为
  第二道网。

## Deliberately not done

- 不给 `ValidatePathWithMode` 增加"缺失父目录也放行"的语义开关：改动面会
  波及所有调用方，且 download.go 先例已证明调整调用点顺序即可。
- PrepareWriteFile（提案构建）本就无 sandbox 校验（提案零变更，执行时复验），
  维持原状。

## 验收口径

- workspace-write 模式下 `write_file` 到全新嵌套目录成功落盘（修复前误拒）。
- read-only 模式写仍然响亮拒绝（ErrSandboxDenied）。
- danger 模式行为不变（短路）。
