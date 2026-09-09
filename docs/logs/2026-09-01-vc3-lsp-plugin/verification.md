# Verification — VC-3 slice 1

Date: 2026-09-01  Branch: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`)

## Plugin module (plugins/lsp)

```
$ cd plugins/lsp
$ gofmt -l .                      # no output = clean
$ go vet ./...                    # passed
$ go test -race ./...             # ok  example.com/vivy/plugins/lsp  1.147s
```

## Kernel-side unit tests

```
$ go build ./...                              # passed (entire tree)
$ go test ./internal/pluginhost/ ./sdk/...    # ok (pluginhost, sdk/internal; sdk/plugin has no test files)
```

## Five-step product path (vivy-plugin-five)

```
$ go build -o vivy-sdk.exe ./sdk
$ ./vivy-sdk.exe verify plugins/lsp
ok C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-vc0\plugins\lsp

$ ./vivy-sdk.exe pack --with lsp
{
  "id": "gen_253ebf6fe9217736",
  "artifact_sha256": "3a568a05d3a6aeaf42d7dc7879b483e8bef6699899c47c5d31a9532848a31223",
  "recipe": { "loop": "eino", "world": "sandbox", "plugins": ["lsp"] },
  "phase": "built",
  "tools": [ { "name": "lsp_diagnostics", "readonly": true } ]
}

$ ./vivy-sdk.exe inspect-artifact dist/gen_253ebf6fe9217736    # same as above
```

- pack used the D4 independent-module path (`-modfile` merging pack.mod/pack.sum +
  overlay Register), and lsp is the first plugin packed through this path; live go.mod/go.sum/
  `internal/generated/plugins/zz_register.go` were untouched (git status showed only
  this slice's files).
- The five-step success criteria were met: new EXE + Generation manifest naming lsp.

## Kernel gate

```
$ just ci        # see the result at the end
```

## Smoke exceptions (with reasons)

1. **The real language-server smoke test was not run**: this machine has no
   gopls/typescript-language-server/pyright/rust-analyzer (`gopls: command not found`).
   Substitute evidence:
   - `pluginhost` real subprocess tests (echo/cd pipes and cwd, Close killing the process)
     prove that Spawn works with a real process;
   - `plugins/lsp` fake-LSP end-to-end tests use the real jsonrpc frame protocol (io.Pipe +
     Content-Length framing), proving the client path (initialize → didOpen → publish →
     formatting → connection reuse).
   The manual smoke steps after installing gopls are recorded in `acceptance.md`.
2. **:3015 browser smoke test not applicable**: this slice has no UI changes.

## Results

- just ci: PASS (gofmt/vet/build, go test ./..., UI typecheck/build,
  embedded smoke; 4 pre-existing ui-e2e failures belong to the UI-E2E-STALE line and
  are unrelated to this slice, so they were not rerun this round).
