# Acceptance — VC-3 切片 2（人工可判）

## 怎么判断它成功了

1. `vivy-sdk pack --with lsp` 的 generation.json `tools` 列出全部四个：
   `lsp_diagnostics`、`lsp_definition`、`lsp_references`、`lsp_symbols`
   （全部 `readonly: true`）。核对命令：
   `vivy-sdk inspect-artifact dist/gen_4bb127429f3049aa`。
2. 装了 gopls 后的真实会话（pack 出的 EXE）：
   - `lsp_symbols {"path":"main.go"}` → 缩进符号树
     （`function main :1:1`…）；
   - `lsp_definition {"path":"a.go","line":10,"column":7}` →
     `b.go:3:14` 形态的落点行；
   - `lsp_references {"path":"a.go","line":10,"column":7}` →
     每个引用一行；`"include_declaration":true` 时包含声明本身；
   - 无匹配（如内置类型跳定义）→ `no matches`，不报错；
   - line/column 传 0 → 明确的 `line and column are 1-based` 错误。
3. 安全边界不变：四个工具全部 effect read，不进 write 审批路径；命令
   与路径边界与切片 1 相同（PATH 裸名/workspace 相对、Env-only spawn）。

## 回滚

revert 本切片 commit（只触 plugins/lsp 与其日志，自包含）。
