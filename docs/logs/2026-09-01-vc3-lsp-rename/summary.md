# 2026-09-01 — VC-3 slice 3: lsp_rename (write effect, through approval)

## What changed

The third batch of `lsp_*` tools: `lsp_rename` (effect **write**).

- `plugins/lsp/protocol.go` — renameParams/textEdit/workspaceEdit (`changes`
  shape; `documentChanges` requires client capabilities, which this plugin does not
  declare, so the server keeps the simpler shape); `utf16Offset` (LSP UTF-16 code-unit
  position → byte offset, including +2 units for surrogate pairs and end-of-line/end-of-file
  clamping) and `applyEdits` (folded in reverse order so earlier offsets are unaffected).
- `plugins/lsp/tools.go` — `renameTool`:
  - effect write → pluginhost's `Readonly=false` → the kernel's existing write-approval
    path enforces the boundary automatically (matching the VC-3 rule that
    "rename/replace_symbol goes through write approval");
  - flow: syncOpen → textDocument/rename → apply the entire WorkspaceEdit in memory
    (read all target files first; any failure aborts without writing) → rewrite each file
    through env.OpenWrite → output a `path (N edits)` summary;
  - a URI outside the workspace (or one that cannot map to a workspace-relative path) →
    reject the entire rename;
  - the plugin Grants add `fs.write`, and the manifest is updated.
- Tests: multi-file WorkspaceEdit end to end (replacement in main.go + insertion in util.go),
  out-of-bounds URI rejection, empty new_name rejection, UTF-16 offset correctness (a
  position after an emoji surrogate pair does not land in the middle of a code point),
  end-of-line clamping, and multiple edits that do not shift one another.

Not done: replace_symbol (the kernel's existing multiedit/patch covers symbol-level replacement,
Crush has no such LSP operation either, and we do not add capabilities that are absent).

## Crush alignment

Crush is FSL-1.1-MIT: LSP rename is behavior alignment with an existing Crush capability;
write approval corresponds to Vivy's own approval surface. Zero code copied.

## Verification command

See `verification.md`.

## Results

- Plugin-module gofmt/vet/`go test -race` all passed;
- Five-step path: verify passed; pack produced gen_d6ddddc35f77e05f with 5 tools, including
  `lsp_rename` readonly=false (write-approval surface).
