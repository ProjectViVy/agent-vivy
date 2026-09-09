# Verification — VC-3 slice 3

Date: 2026-09-01  Branch: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`)

```
$ cd plugins/lsp
$ gofmt -l .                       # no output = clean
$ go vet ./...                     # passed
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

## Smoke exceptions (with reasons)

1. **The real language-server smoke test was not run**: this machine has no gopls or
   other LSP servers installed (same as slices 1/2). Substitute evidence: the fake-server
   end-to-end test covers the full rename path (WorkspaceEdit decoding → in-memory application
   → rewriting two files) and UTF-16 offset correctness.
2. **Write-approval integration was not demonstrated in a running session**: approval is
   decided in kernel pluginhost (`Effect() != read → Readonly=false`), and that path is covered
   by the kernel's existing approval mechanism and tests; this slice did not change the kernel,
   so just ci was not rerun (same reason as slice 2).
3. **:3015 browser smoke test not applicable**: no UI changes.
