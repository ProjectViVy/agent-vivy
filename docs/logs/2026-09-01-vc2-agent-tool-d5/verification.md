# 验证记录

日期：2026-09-01 ｜ 环境：Windows（worktree `agent-vivy-vc0`，分支 `feat/vc1a-bash-tool`）

## 命令与结果

| 命令 | 结果 |
|---|---|
| `gofmt -l internal/tools internal/app internal/worker` | 干净（agent_test.go 一次格式修正后） |
| `go vet ./internal/tools/... ./internal/app/... ./internal/worker/...` | 通过 |
| `go test ./internal/tools/ ./internal/worker/ ./internal/app/ -run 'Agent\|ReadOnly\|SystemPrompt\|Oversize' -count=1` | 三个包全部 `ok` |
| `go build ./...` | 通过（实现在此窗口前完成并验证） |
| `just ci` | **通过**（Go 全量测试 + vet + UI vitest 24 文件 195 用例 + vite build） |

## 新增测试

- `internal/tools/agent_test.go`
  - `TestAgentToolDelegatesTaskAndMask`：task/mask 去空格后传给 seam，结果原样返回
  - `TestAgentToolValidatesArguments`：缺 task / 空 task / 非 JSON 三条拒绝路径
  - `TestAgentToolBoundsTaskAndMask`：task >64 KiB、mask >2 KiB 拒绝
  - `TestAgentToolWithoutOperationsFails`：seam 未装配时失败
  - `TestAgentToolSpec`：Readonly、task 必填、mask 存在
  - `TestRegistryRegistersAgentWithOperations`：有 seam 才注册、无 seam 不注册
- `internal/app/agenttool_test.go`
  - `TestReadOnlyToolNamesFiltersSurface`：只读子集过滤（write_file/bash 剔除、
    mcp_call 即使标记 readonly 也剔除、agent 自身剔除、顺序保持）
  - `TestAgentSystemPromptIncludesMask`：基座无 mask 行；有 mask 追加提示且是基座前缀扩展
  - `TestStartAgentTaskRequiresWiredManager`：未 arm 报 "not wired"
  - `TestStartAgentTaskRequiresRunScopedParent`：无 run context 报 "run-scoped"
  - `TestStartAgentTaskRejectsBeforeSpawning`：父并发满时 StartChild 在
    spawn 之前拒绝（复用既有 harness，不产生真实子进程）
- `internal/worker/server_test.go`
  - `TestRunTurnLoopThreadsSystemPrompt`：`System` 经 turnLoop 成为首条
    system 消息、其后为 user 消息（net.Pipe + 假父端捕获 ModelRequest）
  - `TestRunRejectsOversizeSystemPrompt`：System >64 KiB 被
    "bounded harness limits" 拒绝

## Smoke 例外

按 `smoke-for-user-visible-change` 规则，agent 工具的端到端真实路径需要
真实 provider key（子代理走 model broker 调真实模型）。本环境无 key，
以两层真实协议测试替代并在此记录：

1. worker 协议层：真实 `worker.Run` server + 真实 JSONL RPC peer
   （net.Pipe），断言系统消息贯穿与上界拒绝——子代理 harness 的真实行为。
2. app 装配层：`agentToolRef` 守卫路径 + `StartChild` 拒绝路径走真实
   service/policy/storage 组合（`newChildBrokerTest` harness，与既有
   child-run 测试同一构造）。

浏览器 3015 smoke 不适用：本片为内核工具面，无 UI 变更（UI 零改动）。
