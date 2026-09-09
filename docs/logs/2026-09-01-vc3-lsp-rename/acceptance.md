# Acceptance — VC-3 slice 3 (manually verifiable)

## How to tell it works

1. In generation.json, `lsp_rename` is the only lsp_* tool with `readonly: false`
   (`vivy-sdk inspect-artifact dist/gen_d6ddddc35f77e05f`). In a session with approval
   enabled, calling it uses the same human-confirmation surface as other write tools.
2. In a real session with gopls installed:
   - `lsp_rename {"path":"a.go","line":10,"column":7,"new_name":"newID"}`
     → outputs the affected files and edit counts (`a.go (2 edits)`…), updates the file
     contents, and a subsequent `lsp_diagnostics` call reports either a clean result or
     new diagnostics for the renamed file;
   - a rename rejected by the server (invalid identifier, etc.) → `no changes`, with no write;
   - if the server returns a URI outside the workspace → the entire call fails and writes nothing.
3. Boundaries are unchanged: every file rewrite goes through `env.OpenWrite` (inside the
   workspace, with kernel `resolve` escape prevention), and the plugin still forbids
   `os/exec` and direct file access.

## Rollback

Revert this slice's commit (it touches only plugins/lsp and its logs and is self-contained).
