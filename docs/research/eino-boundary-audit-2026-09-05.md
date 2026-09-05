# Vivy 自研与 Eino 原生边界审计

- 日期：2026-09-05
- 性质：当前源码事实记录；不是实施授权
- 基线：`github.com/cloudwego/eino v0.9.13`，以 `go.mod` 与本机 module cache
  为准
- 决策顺序：Vivy 架构统一性第一，Eino/EinoExt 原生复用第二，自研例外第三

## 1. 结论

影响范围是局部重构，不是推倒 Vivy runtime。

Vivy 的主循环已经原生使用 Eino `adk.ChatModelAgent`、`adk.Runner`、
checkpoint/resume、schema/stream，以及 skill、agentsmd、reduction、
summarization 中间件。当前自研代码的大部分承载 Vivy 的产品权威语义，不能由
Eino 直接替代：Journal、Policy、HITL、预算、会话、恢复、RPC、sandbox、
worker 进程隔离和产品事件投影。

需要重新评估或清理的候选面约为 **1.3–1.6k 行生产代码及其测试/装配**，
主要集中在 MCP transport、动态工具检索、Sequential Thinking、未进入生产
middleware 的 plantask 兼容面，以及流式观测与 Eino callbacks 的重叠可能。
该数字是审查上界，不等于可直接删除行数。

## 2. 当前代码规模与口径

当前非测试 Go 代码约：

| 区域 | 生产代码量 | 审计解释 |
|---|---:|---|
| `internal/runtime` | 17.1k 行 | 同时包含 Eino 接线和 Vivy 产品治理，不可整体按“自研 loop”处理 |
| `internal/provider` | 0.8k 行 | EinoExt 模型构造外加 Vivy provider catalog/secret/model metadata |
| `internal/tools` | 4.0k 行 | Vivy 稳定工具合同；经 adapter 交给 Eino |
| `internal/domain` | 1.1k 行 | Eino 隔离墙外的产品类型 |
| `internal/storage` | 8.6k 行 | Journal 与产品持久化，非 Eino 职责 |

因此，不能用 `internal/runtime` 总行数衡量“重复 Eino”的规模。

## 3. 已正确使用的 Eino 原生能力

| 能力 | 当前接线 | 判定 |
|---|---|---|
| 主 ReAct 循环 | `engine.go`: `adk.NewChatModelAgent` | 原生，保留 |
| 执行与恢复 | `adk.NewRunner` + `ResumeWithParams` | 原生，保留 |
| checkpoint 接口 | `EinoCheckpointAdapter` | 薄适配，保留 |
| OpenAI / Claude | EinoExt model components | 原生，保留 |
| Skills | `middlewares/skill.NewMiddleware` | 原生 middleware + Vivy Backend，保留 |
| AGENTS.md | `middlewares/agentsmd` | 原生 middleware + workspace Backend，保留 |
| 上下文压缩 | `reduction` + `summarization` | 原生 middleware + Vivy 事件/预算桥，保留 |
| 消息、工具、流 | Eino schema/components interfaces | 原生协议 + Vivy adapters，保留 |

## 4. Vivy 必须自己拥有的实现

### 4.1 Journal 与产品事件

Eino `AgentEvent` 是运行时事件，不是 Vivy 的耐久产品合同。Vivy 必须自己负责
persist-before-fanout、run 终态、消息投影、重启恢复、删除栅栏、事件版本、
token/cost 和 RPC 兼容。主要落点是 `service.go`、`mapper.go`、
`message_projector.go`、`internal/domain/event.go` 与 storage 实现。

判定：**正常自研，禁止外包给 Eino。**

### 4.2 Policy、HITL 与工具治理

`tooladapter.go` 在 Eino Tool 接口外执行参数安全校验、Policy profile、Plan Mode、
审批中断/恢复、hooks、proposal/precondition、脱敏、输出上限和 tool mount 审计。
Eino 提供 Tool 与 interrupt 原语，但不拥有 Vivy 的治理合同。

判定：**正常自研；可以复用 Eino 原语，不能移除治理门。**

### 4.3 checkpoint 耐久包装

Eino 提供 `CheckPointStore`；Vivy 的 `VersionedCheckpointStore` 增加 engine 版本、
checksum、BlobStore 和不兼容 fail-closed。`EinoCheckpointAdapter` 只负责接口转换。

判定：**正确的薄适配，不是重复 checkpoint。**

### 4.4 Journal → Eino 上下文投影

`context.go` 负责把耐久消息、工具调用/结果、图片和项目文件投影成 Eino
messages，并实施历史窗口与产品预算。`pairToolTurns` 会丢弃截断窗口内不完整的
调用对；Eino `patchtoolcalls` 则补占位结果，二者语义不同，不能机械替换。

判定：**正常自研；prompt template 只能减少格式代码，不能替代投影语义。**

### 4.5 workspace、文件与编码治理

`EinoFilesystemBackend` 同时服务 Eino Backend 接口和 Vivy typed operations。
workspace containment、symlink 防逃逸、sandbox、原子写入、diff、stale-read、
file version、LSP diagnostics、图片与有界搜索均是 Vivy 产品/安全语义。

判定：**正常自研 Backend；可复用 Eino 工具注册件，但安全主体仍归 Vivy。**

### 4.6 同二进制 child worker

`internal/worker` 的小型模型—工具循环与 Eino `NewAgentTool`/prebuilt agents 有
表面重叠，但它实现的是独立进程故障域：父进程代理模型、工具、审批、预算和
PolicySnapshot；child 不拥有 Journal，也不能放宽父权限。Eino prebuilt agents
是进程内编排，直接替换会形成平行治理轨道。

判定：**经过架构权衡的正常自研，当前不列入删除范围。** 将来若立项轻量
in-process child，可把 `NewAgentTool` 作为可选后端，不替换现有 worker。

### 4.7 RPC、Faces、Channels、Plugins 与 UI

这些是 Vivy 产品表面和宿主边界，不属于 Eino orchestration 职责。

判定：**正常自研。**

## 5. 需要复审的重叠或概念债务

### 5.1 手写 MCP transport — 最大候选

`internal/runtime/mcp_backend.go`（881 行）自行实现 HTTP JSON-RPC、initialize、
session、SSE、retry、tools/resources/prompts、分页与 payload bounding，未接
`eino-ext/components/tool/mcp`。`EinoMCPBackend` 这个名称因此会误导维护者。

建议边界：评估以 EinoExt/mcp 或其底层客户端替换协议 transport；保留 Vivy 的
配置热更、provenance、proposal/HITL、untrusted 标记、大小限制和 RPC/TUI 投影。

影响：**中等、局部；不应修改 Journal 主架构。**

### 5.2 自研 `tool_search` 与可见面 middleware

`internal/tools/toolsearch.go`、`toolselection_middleware.go` 和
`toolselection.go` 合计约 260 行。Eino v0.9.13 已有
`middlewares/dynamictool/toolsearch`，同样负责初始隐藏、搜索和后续显露动态
工具。

建议边界：用 Eino 管模型可见性；Vivy adapter 继续执行 allowlist、Skill mount、
Policy 和实际调用二次校验。先做行为对照，不能只删除执行门。

影响：**小到中等。**

### 5.3 Sequential Thinking

`EinoSequentialThinkingBackend` 没有 Eino import，是约 160 行的内存状态工具，
重启即丢失。应与 EinoExt Sequential Thinking 做 capability check；满足约束则
替换，不满足则保留最小 Vivy wrapper 并改掉误导命名、记录例外。

影响：**小。**

### 5.4 Todo / plantask 的“兼容不等于已采用”

`EinoTodoBackend` 实现了 `plantask.Backend`，但生产工具走 Vivy 自己的
`task_create/get/update/list` typed operations；当前生产装配未调用
`plantask.New(...)`。既有调研把“实现兼容接口”写成“已使用原生 plantask”，
表述不准确。

不能为了原生而直接挂 plantask middleware：middleware 注入的写工具若不经过
Vivy `toolAdapter`，会旁路 Policy/HITL。候选动作应是删除无消费方的兼容半边，
或证明原生工具可以完整穿过治理门后再接入。

影响：**小，主要是概念和死兼容代码清理。**

### 5.5 stream observer 与 Eino callbacks

`model_stream_observer.go` 包装 ChatModel 并用 `schema.Pipe` tee 原始 chunk，解决
Eino materialize 之前的实时 delta。Eino callbacks 提供
`OnEndWithStreamOutput` 的流副本，理论上可能替代包装器。

该路径直接影响 reasoning/text 连续性、usage、背压和 interrupt/resume，只能用
PoC 证明 chunk 边界、顺序、关闭、无重复和恢复行为后再决定。

影响：**中等风险；暂不判定为可删除。**

### 5.6 `Eino*Backend` 命名债务

名副其实或确有 Eino interface 消费：checkpoint、filesystem、skill；todo 仅部分。
没有 Eino import 的实现包括 command、HTTP、MCP、Sequential Thinking、
WebFetch、Download。后者并不自动等于实现错误，例如 SSRF、sandbox 和审批约束
可能要求 Vivy 自有实现，但名称不能再作为“已接 Eino”的证据。

## 6. 对既有调研的修正

既有 `eino-reuse-inventory-2026-08-31.md` 的总体判断仍成立：治理、Journal、
Policy、worker 应由 Vivy 拥有，约六成候选能力可以通过 Eino/上游减量。

需要修正三点：

1. “有等价能力”不等于“当前已使用”：自研 `tool_search` 仍是生产实现。
2. “实现 Eino Backend”不等于“middleware 已接线”：plantask 当前未注册为生产
   middleware。
3. `EinoMCPBackend` 不是 Eino MCP 客户端；它是 Vivy 手写 MCP transport。

## 7. 影响范围判定

- 约 **85%** 的 runtime/product 代码：必要自研，保留。
- 约 **5–8%**：明确值得迁移、减量或清理，集中在 MCP、tool_search、
  Sequential Thinking 和 plantask 兼容面。
- 约 **2–3%**：先 PoC 再定，例如 stream observer/callbacks。
- child worker 虽然代码较多，但属于产品架构选择，不计入立即删除范围。

这些百分比是审计量级，不是精确删除承诺。

## 8. 后续纪律

本记录不授权实现。后续逐项执行时必须：

1. 先核对 pinned Eino/EinoExt 的实际 API 和行为；
2. 写明 Vivy 不变式以及原生件能否满足；
3. 优先替换协议/编排机械层，保留 Vivy 治理壳；
4. 保持 domain firewall，Eino import 不越过 runtime/provider；
5. 每项独立测试、独立交付，不做整片 runtime 重写。
