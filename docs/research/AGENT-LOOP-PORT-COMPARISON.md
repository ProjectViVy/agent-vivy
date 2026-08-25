# agent-diva AGENT-LOOP 与 agent-vivy 循环对照及移植评估

> 状态：**研究报告**（分析文档，不是产品合同；不新增或改写决策）。
> 日期：2026-08-26
> 目的：对照 `agent-diva` 的 AGENT-LOOP（`agent-diva-agent` 的回合循环实现），
> 逐机制评估哪些值得移植到 Vivy 的循环（`internal/runtime` 背后的 Eino 接线）。
> 两个透镜：**① 每个机制是 Eino 原生就有（接线即得），还是要我们自己实现**；
> **② 反向清单——Eino 原生提供、但 agent-diva 没有且 Vivy 也没用的能力，
> 哪些值得白捡（§4）**。本文只做对照与结论，不做实现。
> 证据来源：两侧源码与文档的直接阅读（见附录 A 证据索引）；agent-diva 取
> `agent-diva-agent/src/agent_loop.rs`（3609 行）与 `src/agent_loop/` 模块族；
> Vivy 取 `internal/runtime/` 与 Eino v0.9.13（`github.com/cloudwego/eino@v0.9.13`，
> adk 全目录普查 + callbacks / components / flow / compose 逐包核对）。
> 相关：`v1-minimal-agent-proposal.md`、`VIVY-ASSEMBLY.md`、
> `SELF-EVOLVING-GATEWAY.md`、`DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md`。

---

## 0. 摘要（TL;DR）

agent-diva 的 AGENT-LOOP 是一套**受管回合生命周期**：准入 → 上下文装配 →
迭代 → 工具步 → 终态，配以预算（迭代次数 / token）、压缩（反应式 + 微压缩）、
空输出兜底、运行时控制通道和按 plan 阶段的工具能力矩阵。Vivy 当前的循环是
Eino `ChatModelAgent` 的 ReAct 内循环，外层由 Service 治理：预算账本、审批
人闸、政策门、预检都已就位，但**循环内部**只有 `MaxToolTurns` 一个硬上限。

三个透镜的结论：

- **Eino 原生就有、只需接线（零工作）**：迭代上限（D3）、审批中断/恢复屏障
  （D11）。这两项 Vivy 已经接好线。
- **Eino 没有、需要我们自己实现（= DIVA 移植清单）**：summary-only 奖励轮
  （D4）、空输出兜底（D5）、循环内微压缩（D7）、循环可观测事件。四个构成
  首批（§7.1）。
- **Eino 原生有、但 DIVA 与 Vivy 都没用（反向清单 §4）**：真正值得白捡的是
  **`UnknownToolsHandler`**——现在模型幻觉一个不存在的工具名会让整个 run
  硬失败，配一个 handler 就能变成可自我纠正的工具结果（并入首批 P5）。
  其次是 `ModelRetryConfig` 语义重试、`ReturnDirectly` 工具直答、
  `WithCallbacks` 组件级遥测（候选/按需）。Eino 自带的
  `summarization`/`reduction` 压缩中间件经评估**不采纳为主路径**：摘要调用
  不经 Journal/预算账本，违反"模型可见 ≡ 已记录"。
- **Eino 没有、Vivy 已有对等实现**：上下文装配（D8）、token 预算账本（D18）、
  admission 类入口限流（D2）。
- **记忆族（MEMRULES/ACTMEM/经验日志）**：**完全不做**。Vivy 的记忆设计
  （MEM-1）比 agent-diva 的记忆机制更高级，不属于 AGENT-LOOP 范围，也不作为
  后续候选。

一句话结论：**Vivy 不移植 agent-diva 的循环骨架（stage-contract turn 管线），
它自行实现 agent-diva 在循环内部积累的四个治理机制，外加白捡 Eino 的
UnknownToolsHandler。** 骨架不同（Eino ReAct vs 自研 stage），机制相同。

---

## 1. agent-diva AGENT-LOOP 架构总览

agent-diva 的回合循环是 **facade → turn 编排 → turn 阶段 → 控制面 → 工具面**
五层结构（`agent_loop.rs` 是 facade，`agent_loop/` 模块族承载阶段）：

```text
agent_loop.rs (facade, 3609 行)
  ├─ turn 编排        loop_turn.rs / loop_runtime_control.rs
  ├─ turn 阶段        turn/{admission, context, iteration, tool_step, finalize}.rs
  │                    stage-contract：每个阶段有 StageContract{ShouldRun, Run}
  ├─ turn 策略        turn/policy.rs（TurnMode: Agent | Plan | Ask）
  ├─ turn 提示        turn/prompt.rs
  └─ 工具面           loop_tools.rs
```

关键机制清单（编号后文引用）：

| # | 机制 | 位置 | 一句话 |
|---|---|---|---|
| D1 | stage-contract turn 管线 | `agent_loop/` 各阶段 | 每回合五阶段，每阶段可独立 should-run 判定 |
| D2 | admission 准入控制 | `turn/admission.rs` | 拒绝熔断 + 100 次/小时限流 + 执行冲突检查 |
| D3 | IterationBudget（默认 20） | `turn/iteration.rs` | 回合内迭代上限，可跨回合记账 |
| D4 | summary-only 奖励轮 | `turn/iteration.rs` | 预算耗尽时追加一轮"只总结不调工具" |
| D5 | 空输出兜底分类 | `turn/finalize.rs` | OutputTruncated / InputPressure / EmptyStop |
| D6 | 反应式压缩（重建一次） | `turn/context.rs` | 上下文超限时重建，仅一次 |
| D7 | 工具结果微压缩 | `turn/context.rs` | 长工具结果按比例裁剪 |
| D8 | 预算化上下文装配 | `turn/context.rs` | History/CanonicalCheckpoint/ToolResultInline/ActiveTail/DEFERRED |
| D9 | 自动压缩 + 手动 /compact | `loop_tools.rs` | 工具面暴露压缩命令 |
| D10 | 运行时控制通道 | `loop_runtime_control.rs` | StopSession/ResetSession/SetThinking/SetApprovalPolicy/CompactSession/UpdateNetwork/UpdateMcp |
| D11 | plan 批准生命周期屏障 | `loop_turn.rs` | stop_after_AwaitingApproval，批准前停住 |
| D12 | plan 阶段工具能力矩阵 | `turn/policy.rs` | 每阶段 fail-closed 的工具白名单 |
| D13 | MEMRULES 写门控 | 记忆子系统 | 记忆写入门控 |
| D14 | ACTMEM 脉冲/复盘 + 闲置折叠 | 记忆子系统 | 定期经验沉淀 |
| D15 | 媒体处理 | 消息面 | 图片等媒体的模型可见性 |
| D16 | canonical 工具结果（sha256 artifact 引用） | 工具面 | 结果不内联，引用 artifact |
| D17 | 经验日志 | 记忆子系统 | 会话后经验沉淀 |
| D18 | token 预算账本 | 循环内部 | 回合 token 记账 |

Vivy 侧对应现状（详细对照见 §6）：

- 循环骨架：Eino `adk.ChatModelAgent` + `adk.Runner` 的 ReAct 内循环
  （`internal/runtime/engine.go:88-116`），由 Service 驱动
  （`internal/runtime/service.go`）。
- 回合预算：`MaxToolTurns` → `MaxIterations` 单一硬上限
  （`engine.go:98-103`），超限 → `run.failed`（MA-4）。
- 上下文装配：`buildRunContext` 按字节/条数截断（`internal/runtime/context.go`），
  另有未接线的 `internal/runtime/compaction/` 包（meter + pruner，纯函数、已测试）。
- 工具面：`tooladapter.go` 的 InvokableRun 门（选择 → 校验 → 政策 → 钩子 →
  审批/提问中断 → 执行 → 脱敏 → 头注 → 结果压缩）。
- 控制面：RPC 层（`internal/rpc/control.go`），run/session/approval/question/
  review/child/generations/evals 面，无上下文/预算/压缩面。

---

## 2. 对照方法：为什么"骨架不搬、机制搬"

agent-diva 的 AGENT-LOOP 是自研 Rust 实现，Vivy 的循环是 Eino 库内 ReAct。
两者的差异不是实现语言，而是**骨架的拥有者**：

- agent-diva 拥有整条 turn 管线（stage-contract 是它的产品代码），因此它能把
  admission、预算、压缩、控制通道直接编进管线。
- Vivy 的 turn 管线归 Eino 所有（`adk/react.go` 的 ChatModel 循环），Vivy 通过
  三层接缝介入：middleware（`adk.ChatModelAgentMiddleware`）、run-local 值、
  事件消费（`internal/runtime/mapper.go`）。

结论：**把 stage-contract turn 管线移植过来等于重写 Eino 的 ReAct 循环**
（需换用 `adk.TurnLoop` 甚至自建 Runner），代价与收益不成比例，且违背
`v1-minimal-agent-proposal.md` 的 D-007 隔离（"Vivy's loop stays Eino's
run-internal loop behind the quarantine"）与 `VIVY-ASSEMBLY.md` 的
"loop 出厂 = eino"。所以移植单位是**机制**（agent-diva 在循环内部解决的问题），
不是**骨架**。

判定标准（后文统一使用）：

- **移植**：缺口真实（agent-diva 有、Vivy 无且有害）、Eino 接缝已验证可行、
  切片可独立交付、不违反 Journal / 政策不变式。
- **白捡（§4 专用）**：Eino 原生提供、DIVA 与 Vivy 都没有、接线成本低且
  不违反不变式。
- **暂缓**：机制本身有价值，但受接缝约束、收益证据不足、或依赖独立切片
  （换代 / 多租户）先行。
- **拒绝 / 完全不做**：属于其他切片（记忆族，Vivy 有更高级设计）、与 Vivy
  不变式冲突、或纯属 agent-diva 形态不需要。

---

## 3. 主透镜一：Eino 原生有 vs 需要我们自行实现（DIVA 机制分类）

对照的核心问题是：**agent-diva 的每个机制，Eino v0.9.13 原生就有（接线即得），
还是必须我们自己写代码？** 下表是全部机制的归类（依据见 §5 接缝验证与
附录 A 证据）。

| # | 机制 | Eino v0.9.13 原生? | Vivy 现状 / 结论 |
|---|---|---|---|
| D3 | 迭代上限 | **原生**：`MaxIterations`、内部 State 的 `RemainingIterations`、超限 `ErrExceedMaxIterations` | 已接线（`MaxToolTurns`→`MaxIterations`，`engine.go:98-103`）。**零工作** |
| D11 | 审批中断/恢复屏障 | **原生**：interrupt（InterruptCtx/AwaitingApproval）+ `ResumeWithParams` + CheckPointStore | 已接线（mapper extractInterrupt、service handleInterrupt、engine Resume）。**零工作** |
| D4 | summary-only 奖励轮 | **无原生**（迭代上限只有硬失败，没有"奖励轮"概念） | **需自行实现**：middleware（§6.1 P1）→ 移植 |
| D5 | 空输出兜底分类 | **无原生**（终态消息语义归消费方） | **需自行实现**：mapper/service（§6.1 P2）→ 移植 |
| D7 | 工具结果微压缩 | **无原生**（工具结果原样进 state.Messages，无裁剪；官方 `summarization`/`reduction` 中间件评估见 §4.3-E3） | **需自行实现**：复用未接线的 `compaction` 包（§6.1 P3）→ 移植 |
| D9 | 循环可观测事件 | **无原生事件面**（仅 `CustomizedOutput` 通道，语义自定） | **需自行实现**：mapper 新分支 + 新事件类型（§6.1 P4）→ 移植 |
| D6 | 上下文超限反应式重试 | **无原生** | **需自行实现**（middleware 改写历史）→ 暂缓（收益证据不足） |
| D10 | 运行时控制通道 | **无原生** | **需自行实现**（RPC + 运行中干预）→ 暂缓（依赖 P1/P3 落地） |
| D12 | plan 阶段能力矩阵 | **无原生**：工具集在装配时静态绑定（ToolsConfig）；运行时换工具面需 `TurnLoop`（未启用）。有原生接缝 `BeforeModelRewriteState` + `state.ToolInfos` 可做逐轮过滤 | 已有对等（policy.go 按 profile 门控，plan→Deny）→ 暂缓（现状更严、白名单价值待证） |
| D2 | admission 限流/熔断 | **无原生** | 已有对等：预算账本（budget.go）+ 预检（preflight.go）→ 暂缓（单用户形态） |
| D8 | 预算化上下文装配 | **无原生**（`Runner.Run` 收调用方消息，装配归调用方） | 已有：`buildRunContext`（context.go）→ 不移植 |
| D18 | token 预算账本 | **部分原生**：`TokenUsage` 逐次上报（`ResponseMeta`）；无跨回合账本 | 已有：`budget.go`（MaxModelCalls/MaxToolCalls/MaxEvents）→ 不移植 |
| D15 | 媒体处理 | **部分原生**：`schema.Message` MultiContent 支持媒体部件；模型面看 provider | 文本面现状 → 独立能力切片，不随 AGENT-LOOP |
| D16 | canonical 工具结果（sha256 artifact） | **无原生** | 与 Journal 逐事件回放冲突 → 拒绝 |
| D13 | MEMRULES 写门控 | **无原生** | **完全不做**：Vivy 有更高级记忆设计（MEM-1），不属于 AGENT-LOOP |
| D14 | ACTMEM 脉冲/复盘/闲置折叠 | **无原生** | **完全不做**（同上） |
| D17 | 经验日志 | **无原生** | **完全不做**（同上） |
| D1 | stage-contract turn 管线 | **部分原生**：ReAct 循环即等价骨架；阶段契约（admission/context/…）无原生 | 骨架归 Eino，不移植（§2） |

读表结论：

1. **两个"原生就有"的机制 Vivy 都已接好线**——迭代上限、审批屏障。说明
   agent-diva 的核心回合骨架里，Vivy 靠 Eino + Service 已拿到等价能力，这块
   没有移植负担。
2. **四个"需要自行实现"的机制构成移植清单**——奖励轮、空输出兜底、微压缩、
   可观测事件。它们恰好是"Eino 原生缺失、Vivy 现状为空"的空白区。
3. **记忆族三项是唯一"完全不做"的类别**——不是暂缓、不是候选，是明确排除。

---

## 4. 主透镜二（反向清单）：Eino 原生有、DIVA 与 Vivy 都未用的能力

本节反过来看：**Eino v0.9.13 原生提供、且不在 §1 D1–D18 机制清单里的能力，
哪些值得 Vivy 白捡。** 依据：adk 全目录普查（含 `adk/middlewares/*`、
`adk/prebuilt/*`）+ callbacks / components / flow / compose 逐包核对（附录 A）。

### 4.1 基线：Vivy 已经在用的 Eino 面

为避免误判"没用"，先列 Vivy 的实际用量（grep 全仓非测试源）：

- adk 核心：`NewChatModelAgent`/`NewRunner`/`ToolsConfig`/`WithCheckPointID`/
  `ResumeParams`/`AgentEvent` 流消费/`CancelError` 分类
  （`engine.go`、`service.go`、`mapper.go`、`checkpointadapter.go`）。
- adk 官方中间件已用三个：`middlewares/filesystem` + `middlewares/plantask`
  （`todo_backend.go`）、`middlewares/skill`（`skills_backend.go`）；
  `adk/filesystem` Backend（`filesystem_backend.go`）。
- 自有中间件一个：toolSelection（BeforeAgent 整轮工具过滤，
  `toolselection_middleware.go`）。

其余 adk 面（下述全部）当前未用。

### 4.2 值得白捡（推荐）

#### E1. `UnknownToolsHandler` —— 幻觉工具名的优雅回退（强烈推荐，并入首批）

| 项 | 内容 |
|---|---|
| 是什么 | `compose.ToolsNodeConfig.UnknownToolsHandler`（`compose/tool_node.go:206`，`adk.ToolsConfig` 直接内嵌，Vivy 可在 `engine.go` 一处配置） |
| 不配会怎样 | 模型调用一个不在清单里的工具名时，整个 ToolsNode 失败（`"tool %s not found in toolsNode indexes"`，`tool_node.go:819-824`），**run 硬失败** |
| 配了会怎样 | handler 的字符串返回值作为**正常工具结果**喂回模型（`tool_node.go:868`），模型下一轮自我纠正——比如回一句"该工具不存在，可用工具有 …" |
| 两侧现状 | DIVA 自研循环无此原生件；Vivy 未配置（`engine.go:94-96` 只设 `Tools: wrapped`） |
| 判定 | **白捡，并入首批（§7.1 P5）**。一处配置 + 提示文案设计 + 确定性测试；韧性收益直接（模型幻觉工具名从"run 失败"变成"一轮自我纠正"） |

#### E2. `ModelRetryConfig` / `ModelFailoverConfig` —— 语义重试与模型切换（候选切片）

| 项 | 内容 |
|---|---|
| 是什么 | `adk.ModelRetryConfig`（`retry_chatmodel.go:222`）：`ShouldRetry` 收 `RetryContext`，返回 `RetryDecision`——可改写给模型看的错误、修改输入消息、按尝试次数附加模型选项（如加 MaxTokens）、自定义退避；`ModelFailoverConfig`（`failover_chatmodel.go:128`）：失败时切换模型并改写输入 |
| 两侧现状 | DIVA 无对应原生件；Vivy 只**消费** `WillRetryError` 事件（provider.retry，`mapper.go:78-81`），engine 层没配置任何重试策略——现有重试来自 provider 实现内部，Vivy 治理面看不见、改不了 |
| 判定 | **候选（provider 韧性切片）**。把重试决策挪进 Vivy 可治理的层：哪些错可重试、重试时对模型说什么、几次退避——都是 Vivy 该有的决定权。不是首批（现有 WillRetryError 通路已可用），列为后续候选 |

#### E3. `ToolsConfig.ReturnDirectly` —— 工具直答（按需评估）

| 项 | 内容 |
|---|---|
| 是什么 | 按工具声明"此结果即最终答案"（`ToolsConfig.ReturnDirectly`，`react.go:520-553`）：工具返回后循环直接走 END，省一次模型合成 |
| 两侧现状 | DIVA 无对应原生件；Vivy 未用——所有工具结果都回模型合成最终回复 |
| 判定 | **按需评估**。对查询型工具（读一条笔记、查一个状态）可省一跳模型调用、降时延；但 Vivy 的人设是"模型合成最终回复"，逐工具放开需要产品决策。适合作为配置项设计，不急 |

### 4.3 评估后不采纳的 Eino 原生件（含理由）

#### E4. `adk/middlewares/summarization` 与 `reduction` —— 官方压缩中间件

Eino 自带两个上下文压缩中间件（`adk/middlewares/summarization`、`reduction`），
与首批 P3（微压缩）直接相关，**评估后不作为主路径**：

- **summarization**：token 触发的对话压缩，需要一个**摘要模型**再调用。问题：
  摘要调用发生在 Eino 内部 state，**不经过 Vivy 的 Journal 与预算账本**——
  违反"模型可见 ≡ 已记录"与 MaxModelCalls 记账。桥接成本（把内部摘要调用
  映射成 Vivy 事件）高于自研接线。**不采纳。**
- **reduction**：工具结果确定性截断 + 全量转存 Backend。确定性没问题，但
  "全量内容离开 Journal 存到别处"与 D16 被拒的理由同款（Journal 逐事件回放
  要求结果内联）。**不采纳主路径**，P3 仍用 Vivy 自己的 `compaction` 包
  （`domain.Message` 原生、纯函数、可发 `context.compacted` 事件入账）。

这条评估结论已并入 §6.1 P3 的"Eino 原生件评估"行。

#### E5. `patchtoolcalls` 中间件 —— 未应答 tool call 的占位消息

为没被执行的 tool call 插占位 Tool 消息，防止消息历史畸变。Vivy 在 **feed 装配层**
已有对等处理（`buildRunContext` 的 `pairToolTurns` 丢弃未配对工具行，
`context.go`），且中断/恢复路径由 checkpoint 保证配对。**与现状部分重复，
不采纳**；若未来发现 Eino state 内畸变案例再评估。

#### E6. `NewAgentTool` 原生子代理 —— 与 service 级 child workers 重叠

adk 的子代理委托（`agent_tool.go`，含 `EmitInternalEvents` 子代理事件上流）。
Vivy 已有自己的子代理体系：service 级 child workers（预算账本父子作用域、
child.* 事件、审批权威、`rpc/control.go` 的 child/start|get|wait|cancel）。
原生件没有 Vivy 的治理面（Journal/预算/审批），**用它会绕开治理，不采纳**；
Vivy 的子代理继续走 service 层。

#### E7. 整装 prebuilt 与多 agent 编排件 —— 产品形状不符

- `prebuilt/deep`（DeepAgent：agent+文件工具+todos+task 子代理）、
  `prebuilt/planexecute`、`prebuilt/supervisor`：整装 agent 产品，整体引入与
  Vivy 自我物种定位冲突（D-001/D-005 反克隆）。**不整体采纳**（个别思想如
  write_todos 已由 plantask 覆盖）。
- transfer 族（`SetSubAgents`/`transfer_to_agent`/deterministic_transfer）与
  `SequentialAgent`/`ParallelAgent`/`LoopAgent`：Eino 自己标注 NOT RECOMMENDED，
  且超出 Vivy 单 agent 形状。**不采纳。**

#### E8. `TurnLoop` —— 推送式对话与抢占（换代候选）

推送式回合循环（`Push` + `WithPreempt` 抢占、优雅/立即 Stop、断点续跑），
是最接近"用户打断进行中的回合"的原生件。DIVA 无对应件，Vivy 未用。这是
**交互模型升级**（对话中插话/改指令），不是缺口修补，与 D10（运行时控制
通道）同属换代候选，**暂缓**。

#### E9. 小件与重复件（不采纳或低优先）

| 件 | 结论 |
|---|---|
| `WithChatModelOptions`（`WithModel` 名切/`WithToolChoice`+allowedToolNames） | 按 run 切模型不重建 agent。与 modelbroker 的选型职责重叠；若做按会话切模型再评估 |
| `WithCallbacks` + `utils/callbacks.NewHandlerHelper` | 组件级时延遥测 seam（每组件 OnStart/OnEnd）。Vivy 已有自己的事件流遥测（provider.stall、model.usage）；需要更细的组件级时延时再接 |
| `ExitTool` / `SendToolGenAction`+`NewExitAction` | 模型显式收尾/工具触发循环动作。与 P1 奖励轮语义重叠（P1 是治理驱动的收尾，ExitTool 是模型自主收尾），P1 优先，此件观望 |
| `agentsmd` / `dynamictool`（toolsearch）中间件 | 与现有 preamble notes digest、`tools.Selector` 职责重复，不采纳 |
| prompt 模板（FString/GoTemplate/Jinja2 + MessagesPlaceholder） | Vivy 的 Go 侧 composer（prompt.go）已够用，不引入模板层 |

### 4.4 确认不存在的能力（避免重复寻找）

本轮普查确认 Eino v0.9.13 **没有**以下件（不要再花时间找）：

- 独立 tokenizer / token 计数包——只有 summarization 中间件的 `TokenCounterFunc`
  钩子，其默认实现就是"~4 字符/token"估算，与 Vivy 自研 `compaction/meter.go`
  的启发式同款。P3 的估算口径自研即可。
- cache 辅助包（无任何缓存层）。
- `WithJSONResponse` / ResponseFormat 通用选项（响应格式控制在 eino-ext 各
  provider 实现里，不在核心包）。

---

## 5. Eino v0.9.13 接缝可行性（移植前提）

本轮已直接阅读 Eino v0.9.13 源码验证以下接缝（均为移植候选的落点）：

1. **`BeforeModelRewriteState` 每次模型生成前运行**（`adk/chatmodel.go`），
   可改写 `state.Messages` / `state.ToolInfos`，且改写随 checkpoint 持久化。
   对 `state.ToolInfos = nil` 等价于给该次生成一个空工具列表
   （`model.WithTools(nil)` 后加、胜出）——这是 **summary-only 奖励轮**
   （D4）与 **微压缩**（D7）的落点。
2. **`SetRunLocalValue` / `GetRunLocalValue`**（`adk/react.go`）存 gob 可序列化
   值，跨中断/恢复存活，工具执行上下文可读——这是"跨迭代记账"（奖励轮触发
   判定）与"压缩已发生"标记的落点。
3. **`AgentOutput.CustomizedOutput` + `adk.SendEvent`**（`adk/interface.go`）
   允许 middleware 注入自定义事件；消费端
   （`internal/runtime/mapper.go:94-96`）目前对
   `ev.Output.MessageOutput == nil` 的事件直接忽略（返回 `nil, nil`）——这是
   **循环可观测事件**的落点，需要在 mapper 里新增一类自定义事件分支。
4. **`RemainingIterations` 在内部 State 中逐次递减**，`<= 0` 时报
   `ErrExceedMaxIterations`（`adk/react.go`），当前被 Service 归类为
   `run.failed`（MA-4）——奖励轮必须**早于**该错误触发（在倒数第二次生成后
   接管），否则预算耗尽即失败、没有奖励轮机会。
5. **`UnknownToolsHandler`**（`compose/tool_node.go`）：不配则未知工具名使
   ToolsNode 整体失败（§4.2-E1）；配则 handler 返回值作为工具结果回模型。
6. **middleware 链可叠加**：`engine.go:93` 现有 `newToolSelectionMiddleware()`
   只做 BeforeAgent 整轮工具过滤，新 middleware 可与它共存；官方中间件
   （filesystem/skill/plantask）已验证可与自有链共存（todo/skills backend）。

这些验证结论记录在 §6 各"可行性"列，供实现批直接引用；本文不做实现。

---

## 6. 逐机制对照与判定

### 6.1 移植（首批：turn 生命周期治理批）

#### P1. summary-only 奖励轮（agent-diva D4）

| 项 | 内容 |
|---|---|
| agent-diva | `turn/iteration.rs`：IterationBudget 耗尽前保留一个奖励轮，模型仅被要求产出最终总结，不再给工具（预算 + 提示双约束） |
| Eino 原生? | **无原生**。`MaxIterations` 用尽即 `ErrExceedMaxIterations`，无"奖励轮"概念 → **需自行实现** |
| Vivy 现状 | `MaxToolTurns` 用尽即 `run.failed`（`engine.go:98-103`，`service.go` terminalEvent）。预算到了，模型连"我已经做了这么多，这是结论"都说不出口 |
| 缺口 | 长任务在迭代上限处**硬失败**而非**软收尾**；用户只拿到"limit of tool-call turns"失败，拿不到部分结果 |
| 可行性 | 已验证：`BeforeModelRewriteState` 里检查 run-local 计数器（`SetRunLocalValue`），倒数第二轮把 `state.ToolInfos = nil` 并改写 system 提示为"只总结"，随后模型只输出最终消息，正常走 END |
| 判定 | **移植**。直接补上"循环可能空转失败"的最大缺口，且与 MA-4 语义兼容（保留硬上限，仅在临限时给一次软收尾） |

#### P2. 空输出兜底（agent-diva D5）

| 项 | 内容 |
|---|---|
| agent-diva | `turn/finalize.rs`：终态消息为空时分类为 OutputTruncated / InputPressure / EmptyStop，分别处理（重试 / 提示用户 / 干净停止） |
| Eino 原生? | **无原生**。终态消息语义归消费方 → **需自行实现** |
| Vivy 现状 | `model.completed` 可能带空 Content（`mapper.go:184-189`），Service 照常落 `run.completed`；空回复直接呈现给用户，无分类无兜底 |
| 缺口 | 模型偶发空回复（截断、输入压力、空停）被当成正常完成，用户侧不可区分 |
| 可行性 | 已验证：空输出判定在 `mapper.onMessageEvent` 最终消息分支与 `onTurnEnd`（`mapper.go:234-241`）即可完成；分类与兜底动作在 Service 层（`service.go` consume）实现，不碰 Eino |
| 判定 | **移植**。纯消费端改动、无 Eino 风险，且是用户体验级的可观察差异 |

#### P3. 循环内微压缩（agent-diva D7）

| 项 | 内容 |
|---|---|
| agent-diva | `turn/context.rs`：长工具结果按比例裁剪后进下一轮上下文 |
| Eino 原生? | **无原生裁剪**；官方 `summarization`/`reduction` 中间件经评估不采纳主路径（§4.3-E4：摘要调用不入 Journal/预算；全量转存 Backend 与 D16 同款冲突）→ **需自行实现** |
| Vivy 现状 | 工具结果已做**单次** head/tail/tombstone 压缩（`tooladapter.go` 的 `compactToolResult`，受 `MaxToolResultBytes` 约束），但**没有跨迭代的累积压缩**：长会话中历史工具结果总量持续增长，`buildRunContext` 只按字节从新到旧截断（`context.go`），可能把早期有用的工具结果整段丢掉 |
| 缺口 | 上下文超限的缓解手段只有"整体截断"，没有"对已内联的工具结果做瘦身保留"；`internal/runtime/compaction/` 包（meter.go + pruner.go，纯函数、已测试）**已存在但未接线**（grep 无任何 import） |
| 可行性 | 已验证：`BeforeModelRewriteState` 里对 `state.Messages` 中的工具消息调用 compaction 包的 `CompactToolResult` / `Pruner`，改写随 checkpoint 持久化；现有包直接复用。**设计约束**：压缩必须可审计——发 `context.compacted` 事件（P4）记录被裁消息 id 与前后规模，保住"模型可见 ≡ 已记录"的可回放口径 |
| 判定 | **移植**。把已写好、已测试、零引用的 compaction 包接进循环，是低风险高收益的接线项 |

#### P4. 循环可观测事件（agent-diva D10 的事件面 / D9 的可见性）

| 项 | 内容 |
|---|---|
| agent-diva | 循环状态（预算、压缩、控制动作）在会话流中可见，用户可感知"循环在做什么" |
| Eino 原生? | **无原生事件面**。只有 `CustomizedOutput` 通道，事件语义要自己定 → **需自行实现** |
| Vivy 现状 | 34 个 RunEvent（`internal/domain/event.go`）覆盖模型/工具/审批/子代理，但**没有任何"循环自身动作"事件**：压缩发生、奖励轮发生、预算逼近，前端与日志都看不见 |
| 缺口 | 循环内部治理动作不可观测；排障时无法区分"模型在空转"与"循环在收尾" |
| 可行性 | 已验证：`adk.SendEvent` + `AgentOutput.CustomizedOutput` 注入自定义事件；`mapper.go:94-96` 目前忽略这类事件，新增分支映射为 `loop.summary_pass` / `context.compacted`（domain 新增事件类型 + payload，RPC/UI 订阅面随事件流自动获得） |
| 判定 | **移植**。与 P1/P3 同批落地（没有 P1/P3 就没有事件源），事件类型增量小 |

#### P5. 幻觉工具名优雅回退（Eino 原生白捡，无 DIVA 对应）

| 项 | 内容 |
|---|---|
| 是什么 | §4.2-E1 的 `UnknownToolsHandler`：未知工具名从"run 硬失败"变为"回一句纠正性工具结果，模型自我纠正" |
| Vivy 现状 | `engine.go:94-96` 未配置 handler；模型调错工具名直接 `run.failed` |
| 判定 | **白捡并入首批**。与 P1–P4 同批交付（同一份配置与测试载体），实现量最小 |

### 6.2 暂缓（有方向，先不做）

#### D6. context-overflow 反应式重试（重建一次）

agent-diva 在上下文超限时重建一次再跑。**Eino 无原生**，需自行实现。Vivy 的
`buildRunContext` 已在**回合入口**按预算截断（`context.go`），回合内超限的概率
比 agent-diva 低（agent-diva 是长会话连续循环，Vivy 每回合重装配）。真正的
回合内超限（一次工具返回巨大结果）已被 `MaxToolResultBytes` 单点拦截。
**暂缓理由**：收益证据不足，且"重建一次"在 Eino 里意味着在
`BeforeModelRewriteState` 里改写整段历史（可行但需验证对 checkpoint/恢复的
副作用），应等 P3 微压缩上线后看残留缺口再决定。

#### D2. admission 限流 / 熔断 / 执行冲突

agent-diva 是 100 次/小时限流 + 拒绝熔断 + 执行冲突检查。**Eino 无原生**，需
自行实现。Vivy 是**单用户个人网关**，天然低频，且已有预算账本（`budget.go`：
MaxEvents/MaxModelCalls/MaxToolCalls/MaxRetries）与预检（`preflight.go`）在入口
处限制。**暂缓理由**：限流阈值对个人网关是伪参数；若未来出现多租户或 Studio
批量评测，再以 `budget.go` 为底座扩展，不单独移植。

#### D10. 运行时控制通道（StopSession/ResetSession/CompactSession/…）

agent-diva 有循环内控制命令。**Eino 无原生**，需自行实现。Vivy 的控制面在
**RPC 层**（`internal/rpc/control.go`：run/interrupt、approval、question、
review、child），实现的是"回合外控制"（中断、审批、恢复），不是"循环内命令"。
两者职责不同：Vivy 的 RPC 控制面已覆盖人闸需求（approval/question/review），
缺的是"运行中压缩/改预算/停止会话"这类命令。**暂缓理由**：这是一个独立切片
（新 RPC 方法 + 运行中干预通道 + 恢复语义），且依赖 P3/P1 先把循环内治理做实，
否则控制通道没有可控制的对象。列入换代候选（与 §4.3-E8 TurnLoop 同族）。

#### D12. plan 阶段工具能力矩阵

agent-diva 按 TurnMode（Agent/Plan/Ask）给 fail-closed 工具白名单。**Eino 无
原生**（工具集静态绑定；换工具面需 `TurnLoop`，逐轮过滤可用
`BeforeModelRewriteState` + `state.ToolInfos`）。Vivy 已有**按 profile 的政策
引擎**（`policy.go`：default→Prompt、plan→Deny、read_only→Deny、
full_auto→Allow），工具面在 InvokableRun 门处逐工具 Evaluate
（`tooladapter.go`），plan 阶段整体 Deny 而非按工具白名单。**暂缓理由**：
Vivy 的"plan 阶段不给工具"是比"plan 阶段给指定工具"更严格的现状，安全目标已
达成；细化到 per-phase 白名单需要产品证据（什么时候 plan 阶段该保留某工具），
属骨架级/策略级改动。列入换代候选，先不做。

### 6.3 拒绝 / 完全不做

| # | 机制 | 判定理由 |
|---|---|---|
| D1 | stage-contract turn 管线 | 骨架归 Eino 所有；重写=放弃 D-007 隔离与 `loop: eino` 出厂（§2） |
| D13 | MEMRULES 写门控 | **完全不做**。记忆相关，Vivy 有更高级的记忆设计（MEM-1），不属于 AGENT-LOOP 范围 |
| D14 | ACTMEM 脉冲/复盘 + 闲置折叠 | **完全不做**（同上） |
| D17 | 经验日志 | **完全不做**（同上） |
| D15 | 媒体处理 | 介质能力（图片等），Vivy 当前模型面为文本；属独立能力切片，不随 AGENT-LOOP |
| D16 | canonical 工具结果（sha256 artifact 引用） | 与 Journal 逐事件回放 / 工具结果内联于 `tool.finished` 的事件模型冲突；改动横跨 domain/storage/RPC/UI，收益对个人网关不足 |
| D18 | token 预算账本 | **不是拒绝**：Vivy 已有对等 `budget.go`，方向一致，无需移植 |
| D8 | 预算化上下文装配（DEFERRED 分层） | Vivy 的 `buildRunContext` 已按字节/条数截断并配对工具消息（`context.go`），目标已达成；不移植分层，仅吸收"预算进装配"的思想（P3 依赖） |
| D11 | plan 批准生命周期屏障（stop_after_AwaitingApproval） | 已有对等：Eino 原生 interrupt + `ResumeWithParams` 已接线（`mapper.go` extractInterrupt、`service.go` handleInterrupt），目标已达成 |
| D9 | 自动压缩 + 手动 /compact | 手动 /compact 依赖 D10 控制通道（暂缓）；自动压缩 = P3 本体。随 P3 落地，不单独移植 |

---

## 7. 建议批次与优先级

### 7.1 首批（turn 生命周期治理批）——本次调研之后唯一建议立即做的切片

| 项 | 内容 | 落点 | 性质 |
|---|---|---|---|
| P1 | summary-only 奖励轮 | 新 middleware（`BeforeModelRewriteState` + run-local 计数器 + `ToolInfos=nil`） | 自行实现 |
| P2 | 空输出兜底 | `mapper.go` 最终消息分支 + `service.go` consume 分类 | 自行实现 |
| P3 | 循环内微压缩 | 新 middleware 复用 `internal/runtime/compaction`（`CompactToolResult`/`Pruner`） | 自行实现（复用已有包；Eino 官方中间件已评估排除，§4.3-E4） |
| P4 | 循环可观测事件 | `mapper.go` 自定义事件分支 + domain 新增 `loop.summary_pass`/`context.compacted` | 自行实现 |
| P5 | 幻觉工具名优雅回退 | `engine.go` 配 `UnknownToolsHandler` + 纠正文案 + 测试 | **Eino 原生白捡** |
| 配套 | `config.yaml` runtime 新增 `loop.*` 开关（summary 奖励轮开关/阈值、微压缩开关） | `internal/config/config.go` + `config.example.yaml` | 自行实现 |

配置新增即需解析/校验测试；全部机制需要确定性脚本化模型测试（失败路径 +
正常路径），验证走 `just ci`，UI 冒烟走 `http://127.0.0.1:3015`（split Vite）。

### 7.2 后续候选（换代 / 独立切片）

- **E2 语义重试/模型切换**（`ModelRetryConfig`/`ModelFailoverConfig`）：把重试
  决策权收进 Vivy 治理层（provider 韧性切片）。
- **E3 工具直答**（`ReturnDirectly`）：查询型工具省一跳模型合成，需逐工具
  产品决策。
- context-overflow 反应式重试（D6）：P3 上线后评估残留缺口。
- 运行时控制通道（D10）+ **E8 TurnLoop 抢占式对话**：同族换代候选，依赖
  P1/P3 落地。
- plan 阶段工具能力矩阵（D12）：骨架级或策略级，列换代候选。
- admission 限流（D2）：仅在多租户 / 批量评测出现时以 `budget.go` 为底座扩展。

### 7.3 明确不做

- **记忆族（D13/D14/D17）→ 完全不做**：Vivy 有更高级的记忆设计（MEM-1），
  记忆机制不属于 AGENT-LOOP 范围，也不作为后续候选。
- **E4 官方压缩中间件**（summarization/reduction）：摘要调用不入账 / 全量
  转存与 Journal 冲突（§4.3-E4）。
- **E6 原生子代理**（NewAgentTool）：绕开 Vivy 的 child worker 治理面
  （§4.3-E6）。
- **E7 整装 prebuilt 与多 agent 编排件**（DeepAgent/planexecute/supervisor/
  transfer 族/Sequential-Parallel-LoopAgent）：产品形状不符（§4.3-E7）。
- 媒体（D15）、artifact 结果（D16）→ 独立能力切片，需产品决策。
- stage-contract 骨架（D1）→ 保持 `loop: eino` 出厂。

---

## 8. 交付纪律

- 本文档为调研记录，落入 `docs/logs/2026-08-26-agent-loop-port-comparison/`
  对应迭代日志时以 `summary.md` 记录结论、以本文件为附件。
- 实现批（§7.1）单独开迭代日志，遵循 `just ci` 门与 UI 冒烟门。
- 不触碰 `data/vivy.db`、`data/demo/`、`data/workspaces/`（air-gap）。
- 本文不新增决策，`docs/TODO.md` §0.1 维持现状；若实现批被接受，届时按
  待办项登记。

---

## 附录 A：证据索引

### agent-diva（只读参考，`agent-diva-agent/src/`）

- `agent_loop.rs` — facade 与回合编排（3609 行）
- `agent_loop/loop_runtime_control.rs` — D10 控制命令
- `agent_loop/loop_tools.rs` — D9 压缩工具
- `agent_loop/loop_turn.rs` — D11 plan 屏障
- `agent_loop/turn/admission.rs` — D2 准入
- `agent_loop/turn/context.rs` — D6/D7/D8 上下文装配与压缩
- `agent_loop/turn/finalize.rs` — D5 空输出分类
- `agent_loop/turn/iteration.rs` — D3/D4 迭代预算与奖励轮
- `agent_loop/turn/policy.rs` — D12 能力矩阵
- `agent_loop/turn/prompt.rs` — 回合提示
- `agent_loop/turn/tool_step.rs` — 工具步
- `agent_loop/turn/mod.rs` — 阶段契约

### Vivy（`internal/runtime/`）

- `engine.go:88-116` — Eino ChatModelAgent/Runner 装配；`98-103` MaxToolTurns→MaxIterations；`94-96` ToolsConfig（未配 UnknownToolsHandler）
- `service.go` — 回合编排、预算账本、terminalEvent 分类、恢复、中断、child workers
- `mapper.go:94-96` — 自定义事件当前被忽略；`184-189`/`234-241` 空输出分支
- `context.go` — buildRunContext 按字节/条数截断、工具消息配对（pairToolTurns）
- `tooladapter.go` — InvokableRun 门与 compactToolResult 单点压缩
- `policy.go` — 按 profile 的政策引擎（plan→Deny）
- `budget.go` — 预算账本（MaxEvents/MaxModelCalls/MaxToolCalls/MaxRetries）
- `preflight.go` — 回合前 ready/warning/blocked
- `compaction/meter.go`、`compaction/pruner.go` — 未接线的压缩包（纯函数、已测试）
- `toolselection_middleware.go` — 现有自有 middleware
- `todo_backend.go`（middlewares/filesystem + plantask）、`skills_backend.go`
  （middlewares/skill）、`filesystem_backend.go`（adk/filesystem）— 已用的官方件
- `domain/event.go` — 34 个 RunEvent 类型
- `config/config.go`、`config.example.yaml` — runtime 配置段（max_tool_turns 等）
- `rpc/control.go`、`rpc/protocol.go` — 控制面方法清单

### Eino v0.9.13（`github.com/cloudwego/eino@v0.9.13/adk/` 等）

- `adk/chatmodel.go` — ChatModelAgentMiddleware 钩子、ToolsConfig（内嵌
  ToolsNodeConfig：UnknownToolsHandler/ToolAliases/ToolCallMiddlewares 等）、
  WithChatModelOptions、ModelRetryConfig/ModelFailoverConfig 挂点
- `adk/react.go` — ReAct 循环、RemainingIterations、ErrExceedMaxIterations、
  SetRunLocalValue、ReturnDirectly 分支、SendToolGenAction
- `adk/handler.go` — AgentOutput.CustomizedOutput、adk.SendEvent
- `adk/retry_chatmodel.go` / `failover_chatmodel.go` — RetryDecision/FailoverContext
- `adk/agent_tool.go` — NewAgentTool 子代理委托
- `adk/turn_loop.go` — TurnLoop 推送/抢占
- `adk/middlewares/{summarization,reduction,plantask,skill,filesystem,agentsmd,dynamictool,patchtoolcalls}` — 官方中间件族
- `adk/prebuilt/{deep,planexecute,supervisor}` — 整装 agent
- `compose/tool_node.go:206,819-824,868` — UnknownToolsHandler 语义
- `callbacks/` + `utils/callbacks/template.go` — 回调面与类型化 handler builder
- `components/model/option.go` — WithToolChoice/WithModel/WithMaxTokens 等（无
  WithJSONResponse）
- `flow/` — agent/react（legacy）、retriever/indexer 辅助（无 agentops、无 rag）

### 决策锚

- `docs/v1-minimal-agent-proposal.md` — MA-1..MA-4、D-007 隔离
- `docs/architecture/VIVY-ASSEMBLY.md` — loop 为可装配一等单元，出厂 eino
- `docs/architecture/SELF-EVOLVING-GATEWAY.md` — loop 方向（builtin | generation）
- `docs/research/DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md` — 上一轮对照的文体与证据惯例
