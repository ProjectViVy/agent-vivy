# Verification — FACE-TUI-1 F1

## New tests (`internal/app/facehost_test.go`)

```
go test ./internal/app -run 'TestLoopbackControlCompletesApprovedConversation|TestGatewaylessRunWithoutFaceCancelsDurably' -count=1 -v
--- PASS: TestLoopbackControlCompletesApprovedConversation (2.28s)
--- PASS: TestGatewaylessRunWithoutFaceCancelsDurably (1.27s)

go test ./internal/app -run 'TestLoopbackControlCompletesApprovedConversation|TestGatewaylessRunWithoutFaceCancelsDurably' -count=3 -race
ok  agent-vivy/internal/app  11.682s
```

第一测覆盖 F1 成功判据：无 embed/无 listener 组合内，进程内 JSON-RPC 客户端走 web face 同款方法链（initialize → session/create → turn/start → approval/list → approval/respond(approved) → run/get completed），journal `tool.finished` 零错误，终文落 session/messages。第二测覆盖无 UI 审批失败路径：审批挂起无人应答 → run/cancel 收到 cancelled → gateway-less `App.Run` 干净返回。

## Static + package

```
gofmt -l internal/app/          # clean after gofmt -w facehost_test.go
go vet ./...                    # clean
go build ./...                  # clean
go test ./internal/app -count=1 # ok
```

## just ci

```
( just ci > /tmp/ci-facehost-f1.log 2>&1; echo "CI-EXIT:$?" >> /tmp/ci-facehost-f1.log )
CI-EXIT:0   # fmt-check, ui-ci, vet, test, headless-compile, plugin-ci — 0 FAIL lines
```

headless-compile 步骤同时证明 `vivy_headless` 组合在 `WithoutGateway()` 下仍编译。

## Real-path note (smoke)

- F1 是组合层能力，无可视 UI 变更——`:3015` 浏览器冒烟不适用（web face 路径未动，`just ci` 的 embedded-UI 测试与 `TestRPCBootstrapRoutePrecedesUIShell` 回归钉住默认 gateway 组合）。
- 可执行面冒烟 = 组合测试本身：`DialControl` 走的是真 `controlHandler` + 真 journal + 真 sqlite 存储 + 脚本化模型端点（frozen ENV 会话经 `VIVY_API_BASE` 指向本地 SSE 服务器），即 headless face 将来的真实驱动方式。
- 开发中发现并当场修复的回归：`appOptions.gateway` 零值错误导致默认组合 `httpServer == nil`，被既有 `TestRPCBootstrapRoutePrecedesUIShell`（整包测试）抓住——初版仅跑新测未见此问题，整包跑法保留为流程依据。
