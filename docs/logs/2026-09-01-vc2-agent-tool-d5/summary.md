# VC-2: `agent` 子代理工具 (D5)

日期：2026-09-01 ｜ 分支：`feat/vc1a-bash-tool`（worktree `agent-vivy-vc0`）

## 概要

落地 TODO VC-2 的 `agent` 子代理工具（D5 拍板口径：模型可见，映射既有 child run
机器；子代理 = 可戴面具、无内核、上下文干净，非 Crush 式 coordinator/命名 agent）。

主体由三部分组成：

1. **tools 层 `agent` 工具**（`internal/tools/agent.go`）
   - 参数 `task`（必填，完整自包含委托书）+ `mask`（可选人格提示）；
     task 上界 64 KiB、mask 上界 2 KiB，空/超界均拒绝。
   - `Readonly: true`——子代理复用同一自动放行规则；子代理内部不能产生
     需要审批的效果（见工具面收窄）。
   - 描述明示：只读工具、无 shell、无 MCP、看不到主对话、不能再派生子代理。
   - `AgentOperations` seam 由 app 层实现；`BuiltinWithAgent` 在传入实现时
     才注册该工具（默认 `BuiltinWithWeb` 不带）。
2. **app 层装配**（`internal/app/agenttool.go`）
   - `agentToolRef` 延迟绑定：builtin registry 先于 workerManager 构建
     （app.go 顺序约束），先建空引用、manager 就绪后 `arm()`。
   - `StartAgentTask`：run-scoped context 取父 run id → `StartChild`
     （Text=task、System=人格提示、ToolNames=只读子集）→ `WaitChild`
     同步等待终态；父 ctx 死亡（取消/关停）时
     `CancelChild(context.WithoutCancel(ctx), …)`，不留悬挂 worker 进程。
   - `readOnlyToolNames`：注册表中 `spec.Readonly` 的子集，剔除 `mcp_`
     前缀面与 `agent` 自身——子代理不可嵌套派生、不触 MCP。
   - `agentSystemPrompt`：固定基座（一次性任务、干净上下文、最终消息
     原样回传）+ 可选 `Persona hint (mask)` 行。面具是提示，不是命名
     agent——无内核、无持久身份。
3. **System prompt seam**（worker 协议贯通）
   - `ChildRequest.System`（rpc）→ `driveChild` → `worker.Spec.System` →
     `RunRequest.System` → turnLoop 首条 `system` 消息。model broker 本就
     映射 system 角色，无需改动。worker 侧对 System 施加与 Text 相同的
     `maxChildTextBytes`（64 KiB）上界。

**治理语义（复用既有 child run 机器，零新机制）**：审批 = `ApprovalKindChild`
并入父会话（Review Center 可见）；预算 = `BudgetLedger.Child` 嵌套（子代理
烧的是父 run 的配额，不能重置熔断）；成本聚合 = `legacyModelBroker` 在
`model/complete` 返回后按 `model.usage` 记账（D9 既有链路），session 级
token/费用统计自动纳入子会话用量。

**配置**：`config.example.yaml` 与 `internal/config` 默认 `tools.enabled`
列表加入 `agent`。

## 与 Crush 的关系（FSL-1.1-MIT 合规）

Crush 的 `task` 工具是行为参照（只读子代理委托）；本项目零代码拷入，
全部为既有 Vivy child-run 机器之上的自有实现。人格模型按 D5 拍板：
无 coordinator、无命名 agent 注册表，mask 只是单次提示。

## 明确未做

- 深度/并发上限数值调整（仍为 `maxChildDepth=4`、`maxChildrenPerParent=4`、
  子代理 `childMaxTurns=8`）。
- 子代理事件流的父会话实时呈现（仅终态结果回传；事件已入 Journal）。
- mask 的权限面编辑（D5 提及"参考 diva 可编辑"——属 UI 后续项）。
- agentic_fetch（WEB-1）等基于子代理的上层能力。
