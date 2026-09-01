# Verification — VC-3 切片 1

日期：2026-09-01　分支：`feat/vc1a-bash-tool`（worktree `agent-vivy-vc0`）

## Plugin module（plugins/lsp）

```
$ cd plugins/lsp
$ gofmt -l .                      # （无输出 = 干净）
$ go vet ./...                    # 通过
$ go test -race ./...             # ok  example.com/vivy/plugins/lsp  1.147s
```

## Kernel-side unit tests

```
$ go build ./...                              # 通过（全树）
$ go test ./internal/pluginhost/ ./sdk/...    # ok（pluginhost、sdk/internal；sdk/plugin 无测试文件）
```

## 五步产品路径（vivy-plugin-five）

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

$ ./vivy-sdk.exe inspect-artifact dist/gen_253ebf6fe9217736    # 与上一致
```

- pack 走的正是 D4 独立 module 路径（`-modfile` 合并 pack.mod/pack.sum +
  overlay Register），lsp 是第一个以此路径打包的插件；live go.mod/go.sum/
  `internal/generated/plugins/zz_register.go` 未被触碰（git status 仅本切片
  文件）。
- 五步成功标准达成：新 EXE + Generation manifest 命名 lsp。

## Kernel gate

```
$ just ci        # 见文末结果
```

## Smoke 例外（含原因）

1. **真实语言服务器冒烟未跑**：本机无 gopls/typescript-language-server/
   pyright/rust-analyzer（`gopls: command not found`）。替代证据：
   - `pluginhost` 真实子进程测试（echo/cd 管道与 cwd、Close 杀进程）证明
     Spawn 实现真实可用；
   - `plugins/lsp` fake-LSP 端到端测试走真实 jsonrpc 帧协议（io.Pipe +
     Content-Length 编解码），证明客户端全链路（initialize → didOpen →
     publish → 格式化 → 连接复用）。
   安装 gopls 后的人工冒烟步骤已写入 `acceptance.md`。
2. **:3015 浏览器冒烟不适用**：本切片无 UI 变更。

## 结果

- just ci：PASS（gofmt/vet/build、go test ./...、UI typecheck/build、
  embedded 冒烟；ui-e2e 预存 4 失败归 UI-E2E-STALE 行，与本切片无关，
  未在本轮重复执行）。
