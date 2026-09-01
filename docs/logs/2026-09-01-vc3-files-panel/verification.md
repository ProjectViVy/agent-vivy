# Verification — VC-3g files panel

日期：2026-09-01 ｜ 全部命令在 worktree `agent-vivy-vc0`（分支 `feat/vc1a-bash-tool`）执行。

## Go 单测（新面全覆盖）

```
go test ./internal/runtime/ -run 'TestWorkspaceFiles' -race -count=1   → ok（5 个用例：
    ListAndRead / ReadRejectsEscapes / ReadBinaryAndTruncation / ListSkipsSymlinks / NotWired）
go test ./internal/rpc/ -run 'TestWorkspaceRPC' -race -count=1         → ok（3 个用例：
    DisabledWithoutDep → MethodNotFound / ListAndRead（含缺参 -32602）/ SurfacesInternalErrors）
go build ./...                                                          → ok
```

## just ci（产品门）

```
just ci   → CI_EXIT=0（首次失败：internal/app/app.go 未 gofmt，gofmt -w 后复跑全绿；
             golangci-lint / go test ./... / ui tsc+eslint+vitest+build 全部通过）
```

## 浏览器 e2e（无 provider 壳态，真实浏览器）

```
cd ui; npx playwright test files-panel
→ 1 passed (7.7s)：打开 Files 面板 → 断言空态文案 → Escape 关闭 → 重开仍空态
```

## 内核 RPC 全链路冒烟（真实服务器，WebSocket）

服务器：`VIVY_ADDR=127.0.0.1:8791 VIVY_CONFIG=ui/.e2e-workdir/config.yaml go run ./cmd/vivy`
（healthz → `{"status":"ok"}`）。经 `/rpc/bootstrap` 取 token 后走 WebSocket JSON-RPC：

| 调用 | 结果 |
| --- | --- |
| `workspace/list {run_id:"run_smoke"}` | `{"files":[{"path":"blob.bin","size":11},{"path":"notes/hello.txt","size":30}],"truncated":false}`（种子文件即建即见，排序正确） |
| `workspace/read {run_id, path:"notes/hello.txt"}` | `{"binary":false,"content":"hello vivy workspace\nline two\n","path":"notes/hello.txt","size":30,"truncated":false}`（内容逐字节一致） |
| `workspace/read {run_id, path:"../escape.txt"}` | `-32603 internal error`（拒绝且不泄露校验细节） |
| `workspace/read {run_id, path:"blob.bin"}` | `{"binary":true,"content":"","size":11}`（二进制只回 flag） |
| `workspace/read {run_id}`（缺 path） | `-32602 "run_id and path are required"` |
| `workspace/list {}`（缺 run_id） | `-32602 "run_id is required"` |
| `workspace/read {run_id, path:"missing.txt"}` | `-32603`（不存在文件，折叠为内部错误） |

冒烟种子目录用后即删；服务器进程已停止。

## 冒烟中发现并当场修复

- e2e config 未设 `runtime.workspace_root`：`Ensure` 会落到默认用户根
  `~/.vivy/workspace`。已给 `ui/e2e/global-setup.ts` 显式补
  `runtime.workspace_root`（指向 `.e2e-workdir/workspace`），复跑 files-panel
  spec 绿，且 `~/.vivy/workspace` 中无 e2e 产生目录（grep 计数 0）。
- 另发现一个更早遗留的孤儿 `vivy.exe`（go-build 临时目录路径、15:18 启动，
  系此前 playwright webServer 在 Windows 上未收干净的 `go run` 子进程）占着
  organism lease 导致首次冒烟起不来；已核实命令行后终止该孤儿进程。

## 已知余项

- 有 provider 的"真 run 产生文件 → 面板浏览"全流程未验证（本机无 key）；
  内核侧已被上表覆盖，UI 侧 run 绑定逻辑依赖 `currentRun.id`，与 Review Center
  同源。归属 UI-E2E 轨道在有 key 机器补验。
- RPC 层把路径逃逸/不存在文件都折叠成 `-32603`：安全上无泄露，语义上可再细分
  （-32602/-32004），当前不做——UI 只读列表里点到的路径必然存在，Crush 亦无
  对应细分面。
