# Verification — VC-3 slice 2

Date: 2026-09-01  Branch: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`)

```
$ cd plugins/lsp
$ gofmt -l .                       # no output = clean
$ go vet ./...                     # passed
$ go test -race ./...              # ok  example.com/vivy/plugins/lsp  1.130s

$ cd ..
$ ./vivy-sdk.exe verify plugins/lsp
ok C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-vc0\plugins\lsp

$ ./vivy-sdk.exe pack --with lsp
gen_4bb127429f3049aa
recipe.plugins = ["lsp"]
tools = ["lsp_diagnostics", "lsp_definition", "lsp_references", "lsp_symbols"]
```

## Smoke exceptions (with reasons)

1. **The real language-server smoke test was not run**: this machine does not have
   gopls/typescript-language-server/pyright/rust-analyzer installed (same as slice 1).
   Substitute evidence: the fake-server end-to-end path uses the real jsonrpc frame protocol,
   covering definition/references/symbols end to end and shape compatibility (hierarchical +
   flat + null + single Location).
2. **just ci was not run**: this slice made no kernel/UI/architecture-document changes
   (plugins/lsp is a nested module outside just ci's `./...` scan); the plugin gate followed
   the five vivy-plugin-five steps (verify/pack/inspect + module-local go test -race).
   The pack process itself completed `go build ./cmd/vivy` (including plugin linking).
3. **:3015 browser smoke test not applicable**: no UI changes.
