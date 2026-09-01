# 2026-09-01 — VC-3 切片 3：lsp_rename（write 效果，走审批）

## What changed

`lsp_*` 工具族第三批：`lsp_rename`（effect **write**）。

- `plugins/lsp/protocol.go` — renameParams/textEdit/workspaceEdit（`changes`
  形状；`documentChanges` 需要客户端能力，本插件不声明，服务器保持简单
  形状）；`utf16Offset`（LSP UTF-16 code unit 位置 → 字节偏移，含代理对
  +2 单位、行尾/文件尾钳制）与 `applyEdits`（倒序折叠，早偏移不受影响）。
- `plugins/lsp/tools.go` — `renameTool`：
  - effect write → pluginhost 的 `Readonly=false` → 内核既有 write 审批
    路径自动把关（对应 VC-3 行"rename/replace_symbol 走 write 审批"）；
  - 流程：syncOpen → textDocument/rename → WorkspaceEdit 全部在内存应用
    （先读齐所有目标文件，任一失败即中止不落盘）→ 逐文件 env.OpenWrite
    回写 → 输出 `path (N edits)` 摘要；
  - URI 越出 workspace（或映射不成 workspace 相对路径）→ 拒绝整个 rename；
  - 插件 Grants 增加 `fs.write`，manifest 同步。
- 测试：多文件 WorkspaceEdit 端到端（main.go 替换 + util.go 插入）、
  越界 URI 拒绝、空 new_name 拒绝、UTF-16 偏移正确性（emoji 代理对后
  的位置不落进码元中间）、行尾钳制、多编辑互不位移。

不做：replace_symbol（内核已有 multiedit/patch 覆盖符号级替换场景，
Crush 亦无此 LSP 操作；按"没有的我们不擅自添加"不做）。

## Crush 对齐口径

Crush 为 FSL-1.1-MIT：LSP rename 是 Crush 已有能力的行为对齐；write 审批
对应 Vivy 自己的审批面。零代码拷贝。

## 验证命令

见 `verification.md`。

## 结果

- 插件模块 gofmt/vet/`go test -race` 全绿；
- 五步：verify ok；pack 产出 gen_d6ddddc35f77e05f，5 个工具，其中
  `lsp_rename` readonly=false（write 审批面）。
