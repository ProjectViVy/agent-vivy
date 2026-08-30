# CH-C3 — verification

日期：2026-08-30。工作区：worktree `agent-vivy-channel-c2`（顺序复用），分支 `feat/channel-c3`，基线 edf24c8（含 C1+C2）。

## GOAL 运行方式（子代理分工）

- explore（只读）：扫 runtime Run/终态可观测面、events bus、journal 约束（空 run_id 非法 + 一 run 一终态）、app 装配序列、config 信封、importlint 覆盖、fake 模式。
- executor（写盘）：实现 internal/channelhost 五文件 + fake + TCK、runtime Provenance 出缝、app 装配、能力接口方法集。
- reviewer（只读独立评审）：结论 **PASS**，无 blocker；2 条 should-fix（TODO 对账 + SDK 注释 deferral 表述）已在落地时处理。

## 命令与结果

| 命令 | 结果 |
|---|---|
| `gofmt -l ./internal ./sdk` / `go build ./...` / `go vet ./...` | 全干净（executor） |
| `go test ./... -count=1`（executor） | 全部包 ok：app 8.3s、channelhost 9.3s、runtime 35.6s |
| TCK 逐项 -v | TestStartAllRefusesEmptyAllowFrom / TestPublishInboundDropsSenderNotInAllowFrom / TestPublishInboundAllowedSenderJournalsAndRuns / TestChannelSessionIDDeterministic / TestOnRunEventDeliversAssistantReply / TestOnRunEventFailedDeliversNothing / TestStartAllIgnoresConfigWithoutPlugin / TestDiscoverReportsOnlyImplementedCapabilities 全 PASS |
| Provenance 回归 | TestRunWithChannelProvenance / TestRunWithoutProvenanceKeepUISource / TestRunWithEmptyProvenanceSourceRejected PASS |
| importlint 三测（eino 检疫 / 插件窗 / 参考料） | PASS（internal/channelhost 自动入网；测试文件 eino-free 按约定人工把关 + reviewer grep 确认） |
| `just ci`（分支切换后） | **exit 0**：Go 全部包 ok（channelhost 12.9s、runtime 61.8s、sqlite 44.5s、rpc 29.0s），UI 21 文件 / 175 测试，vite build 绿 |
| reviewer 复核 `go test ./internal/channelhost/... ./internal/app/ ./internal/runtime/` + 全仓 | 全 PASS；`git diff edf24c8 --stat` 仅 4 个已跟踪文件改动 + 新增集与清单一致；engine.go/plugins//go.mod/go.sum/routeTree.gen.ts 零触碰 |

## 验收清单（CH-C3.md §7）

- TCK 全绿 ✅
- `internal/channelhost` 无 eino import（含测试文件人工确认）✅
- 默认 `just run` 无耳朵、零真实协议 HTTP（Register()=nil → channels 空 → StartAll no-op；app 既有测试全绿）✅
- 可选接口文件存在且被 Discover 引用；fake 一个都不实现 ✅
- 空 allow_from 拒 Start ✅；未知 `channels.<name>` 启动失败 ✅

## 诚实声明

- 终态投递的内存跟踪窗口（崩溃丢回复）、`chanin_*` 事件保留、Secret 未钉 token_env：实现上已知、有意留待，登记 §0.1（CH-C3-N1/N2），C4 硬化。
- EnsureSession 建会话竞态的重读收敛路径只有串行测试覆盖（reviewer note #7）。
- CH-C3 PLAN 说「importlint 确认」——检疫由既有 `internal/app/importlint_test.go` 自动覆盖（无需新工具）；已由测试与 grep 双证。
- 未 push。
