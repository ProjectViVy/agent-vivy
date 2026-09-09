# Acceptance — VC-3 slice 1 (manually verifiable)

## How to verify success

1. **The default EXE has no LSP (D4 policy)**
   - The mainline `just run` / everyday `vivy.exe` model tool list has no `lsp_diagnostics`
     (the committed Register installs only first-party channel plugins).
2. **The packed model gains one read-only diagnostic tool**
   - `vivy-sdk pack --with lsp` produces a new EXE + `dist/<gen>/generation.json`,
     whose `recipe.plugins` contains `lsp` and whose `tools` contains `{"name":"lsp_diagnostics",
     "readonly":true}`. Verify it with `vivy-sdk inspect-artifact dist/<gen>`.
3. **In a real session (after installing a language server)**
   - After installing gopls (or typescript-language-server / pyright-langserver /
     rust-analyzer), have the model call the tool in a session using the packed EXE:
     `lsp_diagnostics {"path":"main.go"}`:
     - type/lint errors → output lines such as
       `main.go:13:2: error: undefined: x [compiler]`;
     - a clean file → output `no diagnostics`;
     - an uninstalled server → the call fails with a readable
       `lsp: start gopls: ...` error (not silently).
   - gopls is not installed on this machine; developer smoke steps:
     1. `go install golang.org/x/tools/gopls@latest`
     2. `vivy-sdk pack --with lsp`, run the new EXE, and call the tool above in the session.
4. **Security boundaries (verifiable by code review)**
   - `os/exec` remains blocked by the verifier in plugin source (spawn may only use `Env.Spawn`);
   - The child-process cwd is fixed to the run workspace, and commands may use only a bare PATH name
     or a workspace-relative path (absolute paths/`..` escapes → ErrInvalidArgs);
   - a plugin without the `proc.spawn` grant calling Spawn → ErrDenied;
   - Declaring proc.spawn in a tool-seam manifest makes `vivy-sdk verify` fail
     (fixture `sdk/internal/testdata/bad-procspawn-seam/`).

## Rollback

Revert this slice's commit on branch `feat/vc1a-bash-tool`: all changes (kernel
capability + plugin + fixture + logs) are self-contained, with no migration or
configuration item.
