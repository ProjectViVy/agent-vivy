# 2026-09-01 — VC-3 切片 2：lsp_definition / lsp_references / lsp_symbols

## What changed

VC-3 `lsp_*` 工具族第二批（全部 effect read，无需内核改动）：

- `plugins/lsp/protocol.go` — LSP Location/definitionParams/referenceParams
  类型；documentSymbol（层级）与 symbolInformation（扁平）两种回包形状，
  `parseSymbols` 按"是否含 location 键"启发式兼容两者。
- `plugins/lsp/tools.go`（新）— 共享 `syncOpen`/`syncFile` 前奏（校验
  workspace 相对路径 + 1-based line/column → 0-based LSP position、
  env.OpenRead 读盘 → didOpen/didChange 同步、复用 manager 连接），三个工具：
  - `lsp_definition` — textDocument/definition（Location | Location[] | null）
  - `lsp_references` — textDocument/references（include_declaration 可选，
    默认 false）
  - `lsp_symbols` — textDocument/documentSymbol（层级缩进渲染，含 SymbolKind
    1..26 名称表；扁平形状按 `kind name path:line:col` 渲染）
- `plugins/lsp/vivy-plugin.json` — tools 增至 4 个。
- `plugins/lsp/plugin_test.go` — 假语言服务器新增三个方法的回包；端到端
  （definition 单跳、references 多行、symbols 层级缩进）、扁平形状解析、
  null/单 Location 渲染、0 行号拒绝。

不做：lsp_rename/replace_symbol（下一批，effect write 走审批）；诊断回填；
文件版本 history（等 O1..O6）；UI。

## Crush 对齐口径

Crush 为 FSL-1.1-MIT：定义跳转/引用查找/符号列表均为 Crush 已有 LSP 能力的
行为对齐，零代码拷贝，未添加 Crush 没有的功能面。

## 验证命令

见 `verification.md`。

## 结果

- 插件模块 gofmt/vet/`go test -race` 全绿；
- 五步：`vivy-sdk verify plugins/lsp` ok；`pack --with lsp` 产出
  gen_4bb127429f3049aa，generation.json tools 含全部四个 lsp_* 工具。
