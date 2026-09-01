# Acceptance — VC-3 切片 3（人工可判）

## 怎么判断它成功了

1. generation.json 中 `lsp_rename` 是唯一 `readonly: false` 的 lsp_* 工具
   （`vivy-sdk inspect-artifact dist/gen_d6ddddc35f77e05f`）。在审批开启的
   会话里调用它时，走与其他 write 工具相同的人工确认面。
2. 装了 gopls 的真实会话：
   - `lsp_rename {"path":"a.go","line":10,"column":7,"new_name":"newID"}`
     → 输出受影响文件与编辑数（`a.go (2 edits)`…），文件内容实际更新，
     且后续 `lsp_diagnostics` 对改名后文件给出干净或新的诊断；
   - 服务器拒绝的重命名（无效标识符等）→ `no changes`，不落盘；
   - 若服务器返回 workspace 之外的 URI → 整个调用失败，不写任何文件。
3. 边界不变：所有文件回写都经过 `env.OpenWrite`（workspace 内、内核
   resolve 防逃逸），插件自身仍禁止 `os/exec` 与直接文件访问。

## 回滚

revert 本切片 commit（只触 plugins/lsp 与其日志，自包含）。
