# agent-diva AGENT-LOOP 与 agent-vivy 循环对照及移植评估

> 状态：**研究报告**（分析文档，不是产品合同；不新增或改写决策）。
> 日期：2026-08-26（修订：官方压缩中间件、原生子代理、TurnLoop 三项改判"采用完整版"）
> 目的：对照 `agent-diva` 的 AGENT-LOOP（`agent-diva-agent` 的回合循环实现），
> 逐机制评估哪些值得移植到 Vivy 的循环（`internal/runtime` 背后的 Eino 接线）。
> 两个透镜：**① 每个机制是 Eino 原生就有（接线即得），还是要我们自己实现**；
> **② 反向清单——Eino 原生提供、但 agent-diva 没有且 Vivy 也没用的能力，
> 哪些值得用（§4）**。本文只做对照与结论，不做实现。
> 证据来源：两侧源码与文档的直接阅读（见附录 A 证据索引）；agent-diva 取
> `agent-diva-agent/src/agent_loop.rs`（3609 行）与 `src/agent_loop/` 模块族；
> Vivy 取 `internal/runtime/` 与 Eino v0.9.13（`github.com/cloudwego/eino@v0.9.13`，
> adk 全目录普查 + callbacks / components / flow / compose 逐包核对 +
> summarization/reduction 中间件源码精读）。
> 相关：`v1-minimal-agent-proposal.md`、`VIVY-ASSEMBLY.md`、
> `SELF-EVOLVING-GATEWAY.md`、`DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md`。

---

## 0. 摘要（TL;DR）

agent-diva 的 AGENT-LOOP 是一套**受管回合生命周期**：准入 → 上下文装配 →
迭代 → 工具步 → 终态，配以预算（迭代次数 / token）、压缩（反应式 + 微压缩）、
空输出兜底、运行时控制通道和按 plan 阶段的工具能力矩阵。Vivy 当前的循环是
Eino `ChatModelAgent` 的 ReAct 内循环，外层由 Service 治理：预算账本、审批
人闸、政策门、预检都已就位，但**循环内部**只有 `MaxToolTurns` 一个硬上限。

结论（按来源分三类）：

- **Eino 原生就有、只需接线（零工作）**：迭代上限（D3）、审批中断/恢复屏障
  （D11）。这两项 Vivy 已经接好线。
- **Eino 没有、需要我们自己实现（= DIVA 移植清单，首批）**：summary-only
  奖励轮（D4）、空输出兜底（D5）、循环可观测事件，外加白捡的
  **`UnknownToolsHandler`**（P5：幻觉工具名从 run 硬失败变一轮自我纠正）。
- **Eino 原生有、DIVA 与 Vivy 都没用（反向清单 §4，本轮拍板采用完整版）**：
  - **官方压缩中间件**（`summarization` + `reduction`）：源码精读确认配置面
    带足桥接钩子（摘要模型由调用方传入、`EmitInternalEvents`、`Callback`、
    `ClearPostProcess`）——**采用完整版 + 桥接**（P3 升格），自研
    compaction 包降级为计量/兜底；
  - **TurnLoop 抢占式对话**：**要用**，列第二批（交互模型升级：用户打断
    进行中的回合）；
  - **原生子代理（NewAgentTool）**：**做兼容可选**（service 级 child workers
    为主、原生/swarm 为进阶模式），符合热插拔原则，列第二批；
  - `ModelRetryConfig` 语义重试、`ReturnDirectly` 工具直答：之后再看（候选）。
- **Eino 没有、Vivy 已有对等实现**：上下文装配（D8）、token 预算账本（D18）、
  admission 类入口限流（D2）。
- **记忆族（MEMRULES/ACTMEM/经验日志）**：**完全不做**。Vivy 的记忆设计
  （MEM-1）比 agent-diva 的记忆机制更高级，不属于 AGENT-LOOP 范围，也不作为
  后续候选。

一句话结论：**Vivy 不移植 agent-diva 的循环骨架；首批自研四个缺口件
（奖励轮、空输出兜底、可观测事件 + 白捡 UnknownToolsHandler），压缩直接用
Eino 官方完整版中间件加桥接；第二批用 TurnLoop 与可选原生子代理升级交互与
编排形态。**

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
- **白捡/采用完整版（§4 专用）**：Eino 原生提供、DIVA 与 Vivy 都没有、
  接线成本低或不破坏不变式（必要时加桥接层）。
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
| D7 | 工具结果微压缩 | **原生可用（官方中间件）**：`reduction`（两阶段确定性压缩）+ `summarization`（LLM 摘要），带桥接钩子（§4.3-E2） | 未接线 → **采用完整版 + 桥接**（§6.1 P3） |
| D9 | 循环可观测事件 | **无原生事件面**（仅 `CustomizedOutput` 通道 + 各中间件的 EmitInternalEvents/Callback 钩子，语义自定） | **需自行实现**：mapper 新分支 + 新事件类型（§6.1 P4）→ 移植 |
| D6 | 上下文超限反应式重试 | **部分被官方件覆盖**：`reduction` 的 Clear 阶段就是超限时的原地瘦身 | 随 P3 官方件落地后重评（见 §6.2-D6） |
| D10 | 运行时控制通道 | **部分原生**：TurnLoop 的 Stop（graceful/immediate）覆盖 StopSession 族 | 需自行实现 RPC 面 → 暂缓（依赖 P1/P3；TurnLoop 见 §4.3-E3） |
| D12 | plan 阶段工具能力矩阵 | **无原生**：工具集在装配时静态绑定（ToolsConfig）；运行时换工具面需 `TurnLoop`。有原生接缝 `BeforeModelRewriteState` + `state.ToolInfos` 可做逐轮过滤 | 已有对等（policy.go 按 profile 门控，plan→Deny）→ 暂缓 |
| D2 | admission 限流/熔断 | **无原生** | 已有对等：预算账本（budget.go）+ 预检（preflight.go）→ 暂缓（单用户形态） |
| D8 | 预算化上下文装配 | **无原生**（`Runner.Run` 收调用方消息，装配归调用方） | 已有：`buildRunContext`（context.go）→ 不移植 |
| D18 | token 预算账本 | **部分原生**：`TokenUsage` 逐次上报（`ResponseMeta`）；无跨回合账本 | 已有：`budget.go` → 不移植 |
| D15 | 媒体处理 | **部分原生**：`schema.Message` MultiContent 支持媒体部件；模型面看 provider | 文本面现状 → 独立能力切片，不随 AGENT-LOOP |
| D16 | canonical 工具结果（sha256 artifact） | **无原生**（reduction 的转存是循环内部恢复件，不改 Journal，见 §4.3-E2） | 与 Journal 逐事件回放冲突 → 拒绝 |
| D13 | MEMRULES 写门控 | **无原生** | **完全不做**：Vivy 有更高级记忆设计（MEM-1），不属于 AGENT-LOOP |
| D14 | ACTMEM 脉冲/复盘/闲置折叠 | **无原生** | **完全不做**（同上） |
| D17 | 经验日志 | **无原生** | **完全不做**（同上） |
| D1 | stage-contract turn 管线 | **部分原生**：ReAct 循环即等价骨架；阶段契约（admission/context/…）无原生 | 骨架归 Eino，不移植（§2） |

读表结论：

1. **两个"原生就有"的机制 Vivy 都已接好线**——迭代上限、审批屏障。
2. **压缩（D7）升格为"官方件 + 桥接"**——本轮源码精读后改判，不再自研
   压缩 middleware；真正必须自研的缺口收窄为奖励轮、空输出兜底、可观测事件。
3. **记忆族三项是唯一"完全不做"的类别**——不是暂缓、不是候选，是明确排除。

---

## 4. 主透镜二（反向清单）：Eino 原生有、DIVA 与 Vivy 都未用的能力

本节反过来看：**Eino v0.9.13 原生提供、且不在 §1 D1–D18 机制清单里的能力，
哪些值得 Vivy 用。** 依据：adk 全目录普查（含 `adk/middlewares/*`、
`adk/prebuilt/*`）+ callbacks / components / flow / compose 逐包核对 +
summarization/reduction 源码精读（附录 A）。

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

### 4.2 白捡并入首批

#### E1. `UnknownToolsHandler` —— 幻觉工具名的优雅回退（强烈推荐，并入首批）

| 项 | 内容 |
|---|---|
| 是什么 | `compose.ToolsNodeConfig.UnknownToolsHandler`（`compose/tool_node.go:206`，`adk.ToolsConfig` 直接内嵌，Vivy 可在 `engine.go` 一处配置） |
| 不配会怎样 | 模型调用一个不在清单里的工具名时，整个 ToolsNode 失败（`"tool %s not found in toolsNode indexes"`，`tool_node.go:819-824`），**run 硬失败** |
| 配了会怎样 | handler 的字符串返回值作为**正常工具结果**喂回模型（`tool_node.go:868`），模型下一轮自我纠正——比如回一句"该工具不存在，可用工具有 …" |
| 两侧现状 | DIVA 自研循环无此原生件；Vivy 未配置（`engine.go:94-96` 只设 `Tools: wrapped`） |
| 判定 | **白捡，并入首批（§7.1 P5）**。一处配置 + 提示文案设计 + 确定性测试；韧性收益直接 |

### 4.3 采用完整版（本轮拍板）

#### E2. 官方压缩中间件 `summarization` + `reduction` —— 完整版 + 桥接（P3 升格）

> 改判说明：上一版以"摘要调用不入账 / 全量转存与 Journal 冲突"为由拒绝。
> 本轮精读两个中间件的源码后确认：**配置面带足桥接钩子，之前的冲突可以
> 桥接解决**。按"有能力就用完整版"的原则改判采用。

源码精读结论（`adk/middlewares/summarization/summarization.go:56-151`、
`adk/middlewares/reduction/reduction.go:54-146`）：

| 件 | 做什么 | 桥接钩子（关键） |
|---|---|---|
| `reduction` | 两阶段确定性压缩：①Truncation——工具执行后超长结果（默认 50000）截断+转存；②Clear——发给模型前超预算（默认 160k token）时逐轮瘦身旧工具调用/结果，保留最近 N 轮（`ClearRetentionSuffixLimit`），`ClearAtLeastTokens` 保 prompt cache | `ClearPostProcess`（瘦身后钩子，可发事件）；`ClearMessageRewriter`（改写为 user 消息，如 system-reminder 风格）；按工具 `ToolConfig`/排除表；`TokenCounter` 可替换；`Backend` **可为 nil**（纯占位不转存） |
| `summarization` | token/条数触发（默认 160k）时用**摘要模型**压缩对话史，替换历史并留 `TranscriptFilePath` 指回全文 | `Model` **由调用方传入**（走 Vivy provider 栈）；`EmitInternalEvents`（BeforeSummarize/GenerateSummary/AfterSummarize 三种事件可观测）；`Callback`（压缩前后状态只读钩子）；`Finalize`（控制产出历史）；`TokenCounter`；`Retry`/`Failover` |

桥接方案（守住 Vivy 不变式）：

1. **模型可见 ≡ 已记录**：`ClearPostProcess` / `Callback` 钩子里发
   `context.compacted` 事件（P4），记录被瘦身的消息 id、前后规模、转存路径；
   官方件的历史改写随 checkpoint 持久化，可回放。
2. **预算入账**：summarization 的 `Model` 传 modelbroker 包装的模型（密钥/
   路由走 Vivy 栈）；`EmitInternalEvents=true` 让摘要调用以事件形式到达
   mapper，service.consume 照常记账（MaxModelCalls 不再被绕过）。
3. **转存位置**：`reduction.Backend` 用运行工作区根的 `adk/filesystem`
   backend（Vivy 已有，`filesystem_backend.go`）——转存件落在该 run 的
   workspace，占位消息配 `ReadFileToolName` 指回可读工具。Journal 仍内联
   `tool.finished` 全量结果，**不改事件模型**（D16 拒绝维持有效：这是循环
   内部恢复件，不是 Journal 的 artifact 化）。
4. **自研包降级**：`internal/runtime/compaction` 不再作为压缩引擎——
   `meter.go`（ContextBreakdown）留给 preflight/观测（上下文预算估算），
   `pruner.go` 退役或作为不启用官方件时的确定性兜底。

**判定：采用完整版 + 桥接，P3 从"自研接线"升格为"官方件 + 桥接配置"**
（§6.1 P3、§7.1）。设计注意：reduction 的 Clear 与 P1 奖励轮同用
`BeforeModelRewriteState` 钩子，官方件与自有 middleware 的执行顺序要在
装配处显式排定（先压缩、后奖励轮判定）。

#### E3. `TurnLoop` 抢占式对话 —— 要用（第二批）

| 项 | 内容 |
|---|---|
| 是什么 | 推送式回合循环（`turn_loop.go`）：`Run/Push/Stop/Wait` + `WithPreempt(SafePoint)` 抢占（用户在回合进行中插话/改指令，在 ChatModel 后/工具批后安全点让位）、graceful/immediate Stop、断点续跑（`Store+CheckpointID`） |
| 两侧现状 | DIVA 无对应件；Vivy 未用——现在用户只能在 run 之间操作，无法打断进行中的回合 |
| 价值 | "用户打断进行中的回合"是交互模型升级，且 Stop 原生覆盖 D10 控制通道的 StopSession 族 |
| 接入代价 | engine 层：交互会话的 Runner 换成/包一层 TurnLoop；事件面从 `AgentEvent` 迭代器改为 `TurnLoopConfig.OnAgentEvents` 回调 → mapper 加一条适配路径；与审批中断并存（两层 checkpoint，TurnLoop 是外层会话循环、审批是内层图中断） |
| 判定 | **要用（用户拍板），列第二批（§7.2）**。依赖首批 P1–P5 落地；TurnLoop 是较新 API，实现批先做与现有 checkpoint/resume 的兼容 PoC |

#### E4. 原生子代理 `NewAgentTool` —— 兼容可选进阶（第二批）

| 项 | 内容 |
|---|---|
| 是什么 | adk 的 in-run 子代理委托（`agent_tool.go`）：把一个 Agent 包成父 agent 工具面的工具；`ToolsConfig.EmitInternalEvents` 让子代理事件上流；审批中断经 `tool.CompositeInterrupt` 传播；`WithAgentToolRunOptions`/`DesignateAgent` 逐子代理配参 |
| 两侧现状 | DIVA 无对应原生件；Vivy 有 service 级 child workers（预算父子作用域、child.* 事件、审批权威、`rpc/control.go` child 面）——治理完整但每次委托是一个独立 run，偏重 |
| 用户方向 | 兼容可选进阶：swarm 或原生，符合热插拔原则 |
| 评估 | **同意做兼容可选**。设计为一个可切换的委托后端：`children.mode: service（默认，现状全治理）| native（in-run，轻量快，适合 swarm 式 fan-out——父工具面挂多个 AgentTool，模型驱动并行委托）**。约束：native 模式下子 agent 的工具必须仍走同一个 tooladapter 门（政策/钩子/审批不旁路）；子代理事件映射进 child.* 词汇并入预算账本。两个模式共用 child.* 事件与 RPC 面，对上层无感 |
| 判定 | **采纳为设计目标，列第二批（§7.2）**。动 engine 装配 + mapper + 预算，不是首批 |

### 4.4 候选不急（之后再看）

#### E5. `ModelRetryConfig` / `ModelFailoverConfig` —— 语义重试与模型切换

`adk.ModelRetryConfig`（`retry_chatmodel.go:222`）：`ShouldRetry` 收
`RetryContext`，返回 `RetryDecision`——可改写给模型看的错误、修改输入消息、
按尝试次数附加模型选项、自定义退避；`ModelFailoverConfig` 失败切模型。
Vivy 现在只**消费** `WillRetryError` 事件（provider.retry），engine 层没配置
重试策略——重试藏在 provider 实现里，治理面改不了。**候选（provider 韧性
切片），之后再看。**

#### E6. `ToolsConfig.ReturnDirectly` —— 工具直答

按工具声明"此结果即最终答案"（`react.go:520-553`），省一次模型合成。对
查询型工具（读笔记、查状态）可降时延；需逐工具产品决策。**候选，之后再看。**

### 4.5 评估后不采纳

#### E7. `patchtoolcalls` 中间件 —— 未应答 tool call 的占位消息

Vivy 在 feed 装配层已有对等处理（`buildRunContext` 的 `pairToolTurns` 丢弃
未配对工具行），中断/恢复路径由 checkpoint 保证配对。**与现状部分重复，
不采纳**；若未来发现 Eino state 内畸变案例再评估。

#### E8. 整装 prebuilt 与多 agent 编排件 —— 产品形状不符

- `prebuilt/deep`（DeepAgent）、`prebuilt/planexecute`、`prebuilt/supervisor`：
  整装 agent 产品，整体引入与 Vivy 自我物种定位冲突（D-001/D-005 反克隆）。
  **不整体采纳**（个别思想如 write_todos 已由 plantask 覆盖）。
- transfer 族（`SetSubAgents`/`transfer_to_agent`/deterministic_transfer）与
  `SequentialAgent`/`ParallelAgent`/`LoopAgent`：Eino 自己标注 NOT RECOMMENDED，
  且超出 Vivy 单 agent 形状。**不采纳。**（E4 的原生委托用 `NewAgentTool`，
  不用 transfer 族。）

#### E9. 小件与重复件

| 件 | 结论 |
|---|---|
| `WithChatModelOptions`（`WithModel` 名切/`WithToolChoice`+allowedToolNames） | 按 run 切模型不重建 agent。与 modelbroker 的选型职责重叠；若做按会话切模型再评估 |
| `WithCallbacks` + `utils/callbacks.NewHandlerHelper` | 组件级时延遥测 seam。Vivy 已有自己的事件流遥测；需要更细组件时延再接 |
| `ExitTool` / `SendToolGenAction`+`NewExitAction` | 模型显式收尾/工具触发循环动作。与 P1 奖励轮语义重叠，P1 优先，观望 |
| `agentsmd` / `dynamictool`（toolsearch）中间件 | 与现有 preamble notes digest、`tools.Selector` 职责重复，不采纳 |
| prompt 模板（FString/GoTemplate/Jinja2 + MessagesPlaceholder） | Vivy 的 Go 侧 composer（prompt.go）已够用，不引入模板层 |

### 4.6 确认不存在的能力（避免重复寻找）

本轮普查确认 Eino v0.9.13 **没有**以下件（不要再花时间找）：

- 独立 tokenizer / token 计数包——只有 summarization/reduction 的
  `TokenCounter` 钩子，默认实现是"~4 字符/token"估算，与 Vivy 自研
  `compaction/meter.go` 的启发式同款。P3 的估算口径用钩子注入即可。
- cache 辅助包（无任何缓存层）。
- `WithJSONResponse` / ResponseFormat 通用选项（响应格式控制在 eino-ext 各
  provider 实现里，不在核心包）。

---

## 5. Eino v0.9.13 接缝可行性（移植前提）

本轮已直接阅读 Eino v0.9.13 源码验证以下接缝（均为移植/采用候选的落点）：

1. **`BeforeModelRewriteState` 每次模型生成前运行**（`adk/chatmodel.go`），
   可改写 `state.Messages` / `state.ToolInfos`，且改写随 checkpoint 持久化。
   对 `state.ToolInfos = nil` 等价于给该次生成一个空工具列表——这是
   **summary-only 奖励轮**（D4）的落点，也是官方 reduction Clear 阶段与
   summarization 的挂点（两者同为 middleware，顺序在装配处排定）。
2. **`SetRunLocalValue` / `GetRunLocalValue`**（`adk/react.go`）存 gob 可序列化
   值，跨中断/恢复存活——"跨迭代记账"（奖励轮触发判定）的落点。
3. **`AgentOutput.CustomizedOutput` + `adk.SendEvent`**（`adk/interface.go`）
   允许 middleware 注入自定义事件；消费端（`internal/runtime/mapper.go:94-96`）
   目前对 `ev.Output.MessageOutput == nil` 的事件直接忽略——**循环可观测
   事件**需要在 mapper 里新增一类自定义事件分支。官方 summarization 的
   `EmitInternalEvents` 也走事件面，映射在同一分支处理。
4. **`RemainingIterations` 在内部 State 中逐次递减**，`<= 0` 时报
   `ErrExceedMaxIterations`（`adk/react.go`）——奖励轮必须**早于**该错误
   触发（在倒数第二次生成后接管）。
5. **`UnknownToolsHandler`**（`compose/tool_node.go`）：不配则未知工具名使
   ToolsNode 整体失败（§4.2-E1）；配则 handler 返回值作为工具结果回模型。
6. **官方中间件桥接钩子**：summarization 的 `Model`（调用方传入）/
   `EmitInternalEvents`/`Callback`/`Finalize`；reduction 的
   `ClearPostProcess`/`ClearMessageRewriter`/按工具 `ToolConfig`/可 nil 的
   `Backend`（§4.3-E2 表）。
7. **middleware 链可叠加**：`engine.go:93` 现有 `newToolSelectionMiddleware()`
   只做 BeforeAgent 整轮工具过滤，新 middleware 可与它共存；官方中间件
   （filesystem/skill/plantask）已验证可与自有链共存（todo/skills backend）。
8. **TurnLoop 事件面**：`TurnLoopConfig.OnAgentEvents` 回调（非迭代器），
   Store+CheckpointID 断点续跑；抢占 `WithPreempt(SafePoint)`（§4.3-E3）。
9. **原生子代理**：`NewAgentTool` + `ToolsConfig.EmitInternalEvents` +
   `CompositeInterrupt` 审批传播 + `WithAgentToolRunOptions`/`DesignateAgent`
   （§4.3-E4）。

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
| 可行性 | 已验证：`BeforeModelRewriteState` 里检查 run-local 计数器（`SetRunLocalValue`），倒数第二轮把 `state.ToolInfos = nil` 并改写 system 提示为"只总结"，随后模型只输出最终消息，正常走 END。注意与官方压缩 middleware 的钩子顺序（先压缩、后奖励轮判定） |
| 判定 | **移植（自研）**。直接补上"循环可能空转失败"的最大缺口，且与 MA-4 语义兼容（保留硬上限，仅在临限时给一次软收尾） |

#### P2. 空输出兜底（agent-diva D5）

| 项 | 内容 |
|---|---|
| agent-diva | `turn/finalize.rs`：终态消息为空时分类为 OutputTruncated / InputPressure / EmptyStop，分别处理（重试 / 提示用户 / 干净停止） |
| Eino 原生? | **无原生**。终态消息语义归消费方 → **需自行实现** |
| Vivy 现状 | `model.completed` 可能带空 Content（`mapper.go:184-189`），Service 照常落 `run.completed`；空回复直接呈现给用户，无分类无兜底 |
| 缺口 | 模型偶发空回复（截断、输入压力、空停）被当成正常完成，用户侧不可区分 |
| 可行性 | 已验证：空输出判定在 `mapper.onMessageEvent` 最终消息分支与 `onTurnEnd`（`mapper.go:234-241`）即可完成；分类与兜底动作在 Service 层（`service.go` consume）实现，不碰 Eino |
| 判定 | **移植（自研）**。纯消费端改动、无 Eino 风险，且是用户体验级的可观察差异 |

#### P3. 循环内压缩（agent-diva D6/D7 合并）—— 官方件 + 桥接（升格）

| 项 | 内容 |
|---|---|
| agent-diva | `turn/context.rs`：长工具结果按比例裁剪（D7）；上下文超限时重建一次（D6） |
| Eino 原生? | **原生可用（官方中间件）**：`reduction` 两阶段确定性压缩（单结果截断 + 历史瘦身，等价 D7 并大部分覆盖 D6）+ `summarization` LLM 摘要（完整版）。桥接钩子齐备（§4.3-E2） |
| Vivy 现状 | 工具结果只有**单次** head/tail/tombstone 压缩（`tooladapter.go` 的 `compactToolResult`，受 `MaxToolResultBytes` 约束），无跨迭代累积压缩；`buildRunContext` 只按字节从新到旧截断（`context.go`），可能把早期有用的工具结果整段丢掉；自研 `internal/runtime/compaction` 包已写好已测试但零引用 |
| 方案 | **官方件 + 桥接**（§4.3-E2 四条）：①`ClearPostProcess`/`Callback` 发 `context.compacted` 事件（P4）入 Journal；②summarization 的 `Model` 走 modelbroker、`EmitInternalEvents=true` 让摘要调用入预算账本；③`reduction.Backend` 用运行工作区（复用 adk/filesystem backend），Journal 事件模型不变；④自研包降级——meter 留给 preflight/观测，pruner 退役或兜底 |
| 判定 | **采用完整版 + 桥接**（本轮改判，原"自研接线"方案作废）。阈值（MaxLengthForTrunc/MaxTokensForClear/Trigger/保留轮数）接 `config.yaml` runtime.loop.* |

#### P4. 循环可观测事件（agent-diva D10 的事件面 / D9 的可见性）

| 项 | 内容 |
|---|---|
| agent-diva | 循环状态（预算、压缩、控制动作）在会话流中可见，用户可感知"循环在做什么" |
| Eino 原生? | **无原生事件面**（`CustomizedOutput` 通道 + 各中间件的 EmitInternalEvents/Callback 钩子，语义自定）→ **需自行实现** |
| Vivy 现状 | 34 个 RunEvent（`internal/domain/event.go`）覆盖模型/工具/审批/子代理，但**没有任何"循环自身动作"事件** |
| 缺口 | 循环内部治理动作不可观测；排障时无法区分"模型在空转"与"循环在收尾" |
| 可行性 | 已验证：`adk.SendEvent` + `AgentOutput.CustomizedOutput` 注入自定义事件（P1 奖励轮触发时发 `loop.summary_pass`）；官方件的事件/钩子映射 `context.compacted` 与摘要调用（P3 桥接）。`mapper.go:94-96` 新增自定义事件分支，domain 新增事件类型，RPC/UI 订阅面随事件流自动获得 |
| 判定 | **移植（自研）**。事件源来自 P1（自有 middleware）与 P3（官方件桥接）两路 |

#### P5. 幻觉工具名优雅回退（Eino 原生白捡，无 DIVA 对应）

| 项 | 内容 |
|---|---|
| 是什么 | §4.2-E1 的 `UnknownToolsHandler`：未知工具名从"run 硬失败"变为"回一句纠正性工具结果，模型自我纠正" |
| Vivy 现状 | `engine.go:94-96` 未配置 handler；模型调错工具名直接 `run.failed` |
| 判定 | **白捡并入首批**。与 P1–P4 同批交付，实现量最小 |

### 6.2 暂缓（有方向，先不做）

#### D6. context-overflow 反应式重试（重建一次）

agent-diva 在上下文超限时重建一次再跑。**注意：P3 采用官方 reduction 后，
其 Clear 阶段（超预算时逐轮瘦身 + `ClearAtLeastTokens` 门槛）已经把"超限
原地缓解"做掉了**——DIVA 式"整段重建一次"与 reduction 的渐进瘦身相比更粗
暴。本项**降级为：P3 落地后看残留缺口**，若 reduction+summarization 仍不够
（例如触发频率过高），再评估重建式重试。

#### D2. admission 限流 / 熔断 / 执行冲突

agent-diva 是 100 次/小时限流 + 拒绝熔断 + 执行冲突检查。**Eino 无原生**。
Vivy 是**单用户个人网关**，天然低频，且已有预算账本（`budget.go`）与预检
（`preflight.go`）在入口处限制。**暂缓理由**：限流阈值对个人网关是伪参数；
若未来出现多租户或 Studio 批量评测，再以 `budget.go` 为底座扩展。

#### D10. 运行时控制通道（StopSession/ResetSession/CompactSession/…）

agent-diva 有循环内控制命令。Vivy 的控制面在 **RPC 层**（`internal/rpc/control.go`：
run/interrupt、approval、question、review、child），已覆盖人闸需求，缺的是
"运行中压缩/改预算/停止会话"这类命令。**暂缓理由**：独立切片（新 RPC 方法 +
运行中干预通道 + 恢复语义）；TurnLoop（§4.3-E3）原生覆盖 StopSession 族，
CompactSession 随 P3 官方件的 Trigger/阈值配置部分覆盖，剩余命令等两批落地
后再设计。**列入第二批之后的候选。**

#### D12. plan 阶段工具能力矩阵

agent-diva 按 TurnMode 给 fail-closed 工具白名单。**Eino 无原生**。Vivy 已有
**按 profile 的政策引擎**（`policy.go`：default→Prompt、plan→Deny、
read_only→Deny、full_auto→Allow），plan 阶段整体 Deny 比"按工具白名单"更严。
**暂缓理由**：安全目标已达成；细化到 per-phase 白名单需要产品证据，属策略级
改动。列换代候选。

### 6.3 拒绝 / 完全不做

| # | 机制 | 判定理由 |
|---|---|---|
| D1 | stage-contract turn 管线 | 骨架归 Eino 所有；重写=放弃 D-007 隔离与 `loop: eino` 出厂（§2） |
| D13 | MEMRULES 写门控 | **完全不做**。记忆相关，Vivy 有更高级的记忆设计（MEM-1），不属于 AGENT-LOOP 范围 |
| D14 | ACTMEM 脉冲/复盘 + 闲置折叠 | **完全不做**（同上） |
| D17 | 经验日志 | **完全不做**（同上） |
| D15 | 媒体处理 | 介质能力（图片等），Vivy 当前模型面为文本；属独立能力切片，不随 AGENT-LOOP |
| D16 | canonical 工具结果（sha256 artifact 引用） | 与 Journal 逐事件回放 / 工具结果内联于 `tool.finished` 的事件模型冲突。注意与 E2 的区别：reduction 的转存是循环内部恢复件（Journal 不变），D16 是改 Journal 事件模型——后者维持拒绝 |
| D18 | token 预算账本 | **不是拒绝**：Vivy 已有对等 `budget.go`，方向一致，无需移植 |
| D8 | 预算化上下文装配（DEFERRED 分层） | `buildRunContext` 已按字节/条数截断并配对工具消息；P3 采用官方件后分层装配由 reduction/summarization 在循环内接管。不移植分层 |
| D11 | plan 批准生命周期屏障（stop_after_AwaitingApproval） | 已有对等：Eino 原生 interrupt + `ResumeWithParams` 已接线 |
| D9 | 自动压缩 + 手动 /compact | 自动压缩 = P3 官方件本体；手动 /compact 依赖 D10 控制通道（暂缓），随第二批再看 |

---

## 7. 建议批次与优先级

### 7.1 首批（turn 生命周期治理批）——本次调研之后唯一建议立即做的切片

| 项 | 内容 | 落点 | 性质 |
|---|---|---|---|
| P1 | summary-only 奖励轮 | 新自有 middleware（`BeforeModelRewriteState` + run-local 计数器 + `ToolInfos=nil`） | 自行实现 |
| P2 | 空输出兜底 | `mapper.go` 最终消息分支 + `service.go` consume 分类 | 自行实现 |
| P3 | 循环内压缩（D6+D7） | **官方 `reduction` + `summarization` + 桥接**（事件入账/预算入账/工作区 Backend/阈值接配置）；自研 compaction 包降级为 meter 观测 + pruner 兜底 | **Eino 原生完整版 + 桥接**（§4.3-E2） |
| P4 | 循环可观测事件 | `mapper.go` 自定义事件分支 + domain 新增 `loop.summary_pass`/`context.compacted`（含官方件事件映射） | 自行实现 |
| P5 | 幻觉工具名优雅回退 | `engine.go` 配 `UnknownToolsHandler` + 纠正文案 + 测试 | **Eino 原生白捡** |
| 配套 | `config.yaml` runtime 新增 `loop.*`（奖励轮开关/阈值、reduction/summarization 触发阈值与保留轮数） | `internal/config/config.go` + `config.example.yaml` | 自行实现 |

配置新增即需解析/校验测试；全部机制需要确定性脚本化模型测试（失败路径 +
正常路径；摘要模型用 scripted model 固定输出），验证走 `just ci`，UI 冒烟走
`http://127.0.0.1:3015`（split Vite）。

### 7.2 第二批（采用完整版，本轮拍板）—— 首批落地后启动

| 项 | 内容 | 要点 |
|---|---|---|
| E3 | TurnLoop 抢占式对话 | engine 层交互会话换 TurnLoop；`OnAgentEvents`→mapper 适配；`WithPreempt` 安全点抢占 = 用户打断进行中回合；graceful/immediate Stop 覆盖 D10 的 StopSession 族；先做 checkpoint/resume 兼容 PoC |
| E4 | 原生子代理兼容可选 | `children.mode: service（默认）| native`；native = `NewAgentTool` + `EmitInternalEvents` + `CompositeInterrupt`，子工具面仍走 tooladapter 门；swarm = 父工具面挂多个 AgentTool；两模式共用 child.* 事件与 RPC 面 |

### 7.3 后续候选（之后再看）

- **E5 语义重试/模型切换**（`ModelRetryConfig`/`ModelFailoverConfig`）：把
  重试决策权收进 Vivy 治理层（provider 韧性切片）。
- **E6 工具直答**（`ReturnDirectly`）：查询型工具省一跳模型合成，需逐工具
  产品决策。
- **D10 运行时控制通道**剩余命令（改预算/压缩指令）：随 TurnLoop 与 P3 落地
  后再设计。
- **D12 plan 阶段能力矩阵**：策略级，列换代候选。
- **D2 admission 限流**：仅在多租户 / 批量评测出现时以 `budget.go` 为底座
  扩展。

### 7.4 明确不做

- **记忆族（D13/D14/D17）→ 完全不做**：Vivy 有更高级的记忆设计（MEM-1），
  记忆机制不属于 AGENT-LOOP 范围，也不作为后续候选。
- **E8 整装 prebuilt 与多 agent 编排件**（DeepAgent/planexecute/supervisor/
  transfer 族/Sequential-Parallel-LoopAgent）：产品形状不符（§4.5-E8）。
- **E7 patchtoolcalls**：与 `pairToolTurns` 部分重复（§4.5-E7）。
- **D16 Journal artifact 化**：Journal 事件模型不变（与 E2 转存的区别见
  §6.3）。
- 媒体（D15）→ 独立能力切片，需产品决策。
- stage-contract 骨架（D1）→ 保持 `loop: eino` 出厂。

---

## 8. 交付纪律

- 本文档为调研记录，落入 `docs/logs/2026-08-26-agent-loop-port-comparison/`
  对应迭代日志时以 `summary.md` 记录结论、以本文件为附件。
- 实现批（§7.1 首批、§7.2 第二批）各自单独开迭代日志，遵循 `just ci` 门与
  UI 冒烟门。
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
- `compaction/meter.go`、`compaction/pruner.go` — 未接线的压缩包（纯函数、已测试；P3 改判后降级为 meter 观测 + pruner 兜底）
- `toolselection_middleware.go` — 现有自有 middleware
- `todo_backend.go`（middlewares/filesystem + plantask）、`skills_backend.go`
  （middlewares/skill）、`filesystem_backend.go`（adk/filesystem）— 已用的官方件
- `domain/event.go` — 34 个 RunEvent 类型
- `config/config.go`、`config.example.yaml` — runtime 配置段（max_tool_turns 等）
- `rpc/control.go`、`rpc/protocol.go` — 控制面方法清单

### Eino v0.9.13（`github.com/cloudwego/eino@v0.9.13/adk/` 等）

- `adk/chatmodel.go` — ChatModelAgentMiddleware 钩子、ToolsConfig（内嵌
  ToolsNodeConfig：UnknownToolsHandler/ToolAliases/ToolCallMiddlewares 等、
  EmitInternalEvents/ReturnDirectly）、WithChatModelOptions、
  ModelRetryConfig/ModelFailoverConfig 挂点
- `adk/react.go` — ReAct 循环、RemainingIterations、ErrExceedMaxIterations、
  SetRunLocalValue、ReturnDirectly 分支、SendToolGenAction
- `adk/handler.go` — AgentOutput.CustomizedOutput、adk.SendEvent
- `adk/retry_chatmodel.go` / `failover_chatmodel.go` — RetryDecision/FailoverContext
- `adk/agent_tool.go` — NewAgentTool 子代理委托、CompositeInterrupt 传播
- `adk/turn_loop.go` — TurnLoop 推送/抢占（Run/Push/Stop/Wait、WithPreempt、
  TurnLoopConfig.OnAgentEvents、Store+CheckpointID）
- `adk/middlewares/summarization/summarization.go:56-253` — TypedConfig（Model
  调用方传入/EmitInternalEvents/Callback/Finalize/Trigger/TokenCounter/
  Retry/Failover）、TriggerCondition
- `adk/middlewares/reduction/reduction.go:54-221` — TypedConfig（两阶段：
  MaxLengthForTrunc 截断 + MaxTokensForClear 清理；ClearPostProcess/
  ClearMessageRewriter/ClearRetentionSuffixLimit/ClearAtLeastTokens/按工具
  ToolConfig/Backend 可 nil）
- `adk/middlewares/{plantask,skill,filesystem,agentsmd,dynamictool,patchtoolcalls}` — 官方中间件族其余件
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
