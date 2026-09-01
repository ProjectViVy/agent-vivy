# Verification — VC-3 切片 3

日期：2026-09-01　分支：`feat/vc1a-bash-tool`（worktree `agent-vivy-vc0`）

```
$ cd plugins/lsp
$ gofmt -l .                       # 无输出 = 干净
$ go vet ./...                     # 通过
$ go test -race ./...              # ok  example.com/vivy/plugins/lsp  1.1s

$ cd ..
$ ./vivy-sdk.exe verify plugins/lsp
ok C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-vc0\plugins\lsp

$ ./vivy-sdk.exe pack --with lsp
gen_d6ddddc35f77e05f
tools = [lsp_diagnostics(readonly), lsp_definition(readonly),
         lsp_references(readonly), lsp_symbols(readonly),
         lsp_rename(writable)]
```

## Smoke 例外（含原因）

1. **真实语言服务器冒烟未跑**：本机未安装 gopls 等 LSP 服务器（同切片
   1/2）。替代证据：假服务器端到端覆盖 rename 全链路（WorkspaceEdit 解码
   → 内存应用 → 双文件回写）与 UTF-16 偏移正确性。
2. **write 审批联动未在运行会话中演示**：审批判定在内核 pluginhost
   （`Effect() != read → Readonly=false`），该路径由内核既有审批机制
   与其测试覆盖；本切片未改内核，不重跑 just ci（同切片 2 理由）。
3. **:3015 浏览器冒烟不适用**：无 UI 变更。
