# Acceptance — VC-3 slice 2 (manually verifiable)

## How to tell it works

1. `vivy-sdk pack --with lsp` produces a generation.json whose `tools` lists all four:
   `lsp_diagnostics`, `lsp_definition`, `lsp_references`, `lsp_symbols`
   (all with `readonly: true`). Verify with:
   `vivy-sdk inspect-artifact dist/gen_4bb127429f3049aa`.
2. In a real session with gopls installed (using the packed EXE):
   - `lsp_symbols {"path":"main.go"}` → an indented symbol tree
     (`function main :1:1`…);
   - `lsp_definition {"path":"a.go","line":10,"column":7}` →
     a target line in the form `b.go:3:14`;
   - `lsp_references {"path":"a.go","line":10,"column":7}` →
     one line per reference; `"include_declaration":true` includes the declaration itself;
   - no matches (such as jumping to the definition of a built-in type) → `no matches`, not an error;
   - passing 0 for line/column → an explicit `line and column are 1-based` error.
3. Security boundaries are unchanged: all four tools have effect read and do not enter the
   write-approval path; command and path boundaries match slice 1 (bare PATH name/workspace-
   relative path, Env-only spawn).

## Rollback

Revert this slice's commit (it touches only plugins/lsp and its logs and is self-contained).
