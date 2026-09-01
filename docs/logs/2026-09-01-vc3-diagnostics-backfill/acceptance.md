# Acceptance

## How a human can tell it worked

1. Install a language server (`go install golang.org/x/tools/gopls@latest`
   or have `typescript-language-server` on PATH).
2. Build an EXE with the plugin: `vivy-sdk pack --with lsp`
   (gen_c569df92d8fbfc1f or later) and run it.
3. In a session with a run workspace, ask the model to introduce a type
   error into a `.go` file and approve the `patch`/`write_file`/`multiedit`
   tool call.
4. The tool result now contains a `diagnostics` field, e.g.:
   `"diagnostics": "main.go:5:2: error: undefined: foo [gopls]"` — the
   model sees the finding in the same turn and can fix it without a
   separate `lsp_diagnostics` call.
5. Editing a clean file (or one with no language server) yields no
   `diagnostics` field at all — results stay identical to before.

## Boundaries

- Default EXE (no lsp plugin): zero behavior change — the bridge has no
  observers and `diagnostics` never appears.
- `bash`-side file changes are not backfilled (call `lsp_diagnostics`
  explicitly).
- One file contributes at most 30 diagnostic lines; the wait per file is
  ~2s bounded.

## Rollback

Revert this slice's commit. The plugin is optional and the kernel contract
is additive (`omitempty` field + optional interfaces), so nothing else
depends on it.
