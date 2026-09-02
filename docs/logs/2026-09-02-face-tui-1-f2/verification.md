# verification — FACE-TUI-1 F2

日期：2026-09-02。全部命令在仓库根 `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy` 执行（除冒烟在 %TEMP%）。

## 单元 / 组件测试

| 命令 | 结果 |
| --- | --- |
| `cd faces/headless && go vet ./... && go test -race ./...` | `ok example.com/vivy/faces/headless 1.508s`（7 测试：completed 流式/非流式、approval 块取消、question 块取消、--continue 取既有会话、failed 终点、空 prompt 拒拨号） |
| `go test ./sdk/...` | `ok agent-vivy/sdk/internal 28.105s`（含 checkFaceManifest/hasNewFace/seam 分派既有用例全绿） |
| `go test -run 'TestRunFace\|TestLoopback\|TestGatewayless' ./internal/app/` | `ok agent-vivy/internal/app 3.860s`（新增 TestRunFaceWithoutOrganFails、TestRunFaceServesGatewaylessControlPlane；F1 两测不回归） |
| `go build ./...`（含 cmd/vivy face.Register 分支） | 通过 |
| `gofmt -l sdk internal cmd faces` | 无输出（零未格式化） |

## pack 五步（真实产物）

| 命令 | 结果 |
| --- | --- |
| `go build -o vivy-sdk.exe ./sdk` | 通过 |
| `./vivy-sdk.exe verify faces/headless` | `ok ...\faces\headless`（seam-face 校验路径生效） |
| `./vivy-sdk.exe pack --face headless` | `gen_7dbd0d2922738a21`，EXE built；generation.json：`recipe.face: "headless"` + `face{name: headless, kind: headless, grants: [tty argv rpc.client], tree_hash: ca80bb…}` |

## 真实 EXE 冒烟（air-gap 安全：全部在 `%TEMP%\f2-smoke` 独立目录，config/storage/workspace 均临时；未触碰仓库 data/）

1. `ANTHROPIC_API_KEY=dummy vivy.exe run --continue "smoke probe"` →
   `vivy run: headless: no sessions to continue; run without --continue to start one`，EXIT:1。
   **分支证明**：错误前缀 `headless:` 来自器官（若 face 分支未接线，内核 RunHeadless 会打 `app:` 前缀）。
2. `ANTHROPIC_API_KEY=dummy vivy.exe run "smoke probe"` →
   `vivy: run failed (internal_error): The model run could not be completed...`，EXIT:1。
   **全流证明**：器官完成 initialize → session/create → turn/start → run/subscribe → run.failed 事件渲染 → 终点状态 → 进程退出码映射。dummy key 下 provider 失败是预期（无真实网络依赖）。

冒烟后 scratch 已删除。

## `just ci`

- 第一轮：`CI-EXIT:1`——`test` 步骤两枚既有负载敏感 flake（`TestCronAtJobDeletesAfterSuccessfulRun` 30.96s 顶到金丝雀预算、`TestServiceGrepToolEndToEnd` 14.98s，即 TFLAKE-CRON 与 TEST-2 两行所记）；本切片零文件触碰 `internal/runtime`。
- 隔离重跑：`go test -run 'TestCronAtJobDeletesAfterSuccessfulRun|TestServiceGrepToolEndToEnd' -count=1 ./internal/runtime/` → `ok 2.656s`（全量负载下才复现，与板上记录一致）。
- 第二轮全量：`CI-EXIT:0`（fmt-check[含 faces]、ui-ci、vet、test、headless-compile、plugin-ci[plugins+faces 两 module 根]）。

## 过程教训

- faces/headless 首轮测试失败暴露 fake env 与真实控制面语义偏差：run/cancel 之后服务器会回推 `run.cancelled` 终点，fake env 现在照实模拟（organ 等终点而非取消调用本身）。
- pack.go 编辑中一次误删 for 行被立即发现并修复，未进入任何测试/提交。
