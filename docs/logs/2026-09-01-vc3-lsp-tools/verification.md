# Verification — VC-3 切片 2

日期：2026-09-01　分支：`feat/vc1a-bash-tool`（worktree `agent-vivy-vc0`）

```
$ cd plugins/lsp
$ gofmt -l .                       # 无输出 = 干净
$ go vet ./...                     # 通过
$ go test -race ./...              # ok  example.com/vivy/plugins/lsp  1.130s

$ cd ..
$ ./vivy-sdk.exe verify plugins/lsp
ok C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-vc0\plugins\lsp

$ ./vivy-sdk.exe pack --with lsp
gen_4bb127429f3049aa
recipe.plugins = ["lsp"]
tools = ["lsp_diagnostics", "lsp_definition", "lsp_references", "lsp_symbols"]
```

## Smoke 例外（含原因）

1. **真实语言服务器冒烟未跑**：本机未安装
   gopls/typescript-language-server/pyright/rust-analyzer（同切片 1）。
   替代证据：假服务器端到端走真实 jsonrpc 帧协议，覆盖 definition/
   references/symbols 全链路与形状兼容（层级 + 扁平 + null + 单 Location）。
2. **just ci 未跑**：本切片零内核/UI/架构文档改动（plugins/lsp 为嵌套
   module，不在 just ci 的 `./...` 扫描面内）；插件门禁按
   vivy-plugin-five 五步执行（verify/pack/inspect + 模块内 go test -race）。
   pack 过程即完成了 `go build ./cmd/vivy`（含插件链接）的编译验证。
3. **:3015 浏览器冒烟不适用**：无 UI 变更。
