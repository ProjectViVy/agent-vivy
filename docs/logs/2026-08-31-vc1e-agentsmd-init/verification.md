# Verification — VC-1e

日期：2026-08-31 ｜ 分支：`feat/vc1a-bash-tool`（vc0 worktree）

## Commands and results

| Command | Result |
| --- | --- |
| `go build ./...` | exit 0 |
| `go vet ./internal/runtime ./internal/app ./cmd/vivy` | exit 0（修复 1 处：`domain.RunEvent` 无 `ID` 字段，改用 `Seq`） |
| `go test ./internal/runtime -run "TestEngineAgentsMD\|TestServiceAgentsMD" -count=1 -v` | 5/5 PASS |
| `go test ./cmd/vivy -run TestInit -count=1 -v` | 4/4 PASS |
| `go run ./cmd/vivy init`（`data/tmp-init-demo/` 冒烟，含 main.go） | exit 0，生成 AGENTS.md，输出"只记非显而易见知识"提示；二次执行 exit 1 拒绝覆盖，原文件未动 |
| `just ci`（完整产品门） | exit 0（见下） |

## just ci

- 2026-08-31 背景 task `b6mh8nnq4`：exit 0，全部包测试 + UI 构建通过。

## Test coverage notes

`internal/runtime/agentsmd_test.go`：

1. `TestEngineAgentsMDInjection` — 真实 `EinoFilesystemBackend` + 真实 workspace：两轮模型调用（echo 工具 + 收尾）输入各含且仅含一条注入消息，且紧邻首条真实 user 消息之前（幂等）。
2. `TestEngineAgentsMDMissingFileInjectsNothing` — 无 AGENTS.md 的 run 完全不受影响（回归覆盖本轮修复的 ErrNotExist sentinel）。
3. `TestEngineAgentsMDRunWorkspaceScoping` — run A 的 AGENTS.md 不会泄漏进 run B（逐 run workspace 隔离）。
4. `TestServiceAgentsMDInjectionIsTransient` — 全栈 service e2e：模型确实看到注入内容；同时 **Journal replay 与消息存储均不含 marker**（D6 瞬态性证据）。
5. `TestServiceAgentsMDInjectionSurvivesApprovalResume` — 审批挂起→恢复流：注入消息过 checkpoint 往返后仍恰好一条（eino Extra 标记经序列化保留，幂等防重注成立），Journal 仍无 marker。

`cmd/vivy/init_test.go`：模板创建、空目录拒绝（仅 .git 视为空）、拒绝覆盖（原文件 byte 级不变）、规则文件探测（`.cursorrules` + copilot 命中且进入模板）。

## Smoke policy note

- `vivy init` 是真实 CLI 路径冒烟（`go run` 在 workspace 内 scratch 目录执行，事后清理）。
- AGENTS.md 注入本身无法在本 lane 做 3015 浏览器冒烟：注入发生在模型调用时，需真实 provider（本机无 provider 凭据，D-010 禁止把密钥放进 fixture）。service e2e（#4/#5）驱动的是与生产完全相同的 engine→adapter→backend→journal 栈，是本环境可达的最强证据。

## Deferred

- stale-read 防护 / file_versions：等 O1..O6（同 VC-1d 记录）。
