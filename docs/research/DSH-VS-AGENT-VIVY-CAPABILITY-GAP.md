# DeepSeek Harness 与 agent-vivy 能力对比与差距分析

> 状态：**研究报告**（分析文档，不是产品合同；不新增或改写决策）。
> 日期：2026-08-16
> 目的：逐维对比 DeepSeek Harness（DSH）与 agent-vivy（物种 `vivy.exe`）的
> 实际能力，标注差距的方向、性质与来源，供能力提案和架构讨论引用。
> 证据来源：两侧源码与文档的直接阅读（见附录 B 证据索引），DSH 取
> `.workspace/deepseek-harness/deepseek-harness`（HEAD `47f9438`，`0.1.0-rc.5`）。
> 相关：`AGENT-VIVY-DIRECTION.md`、`SELF-EVOLVING-GATEWAY.md`、
> `VIVY-GATEWAY-AND-STUDIO.md`、`VIVY-STUDIO.md`、`prd-agent-vivy-v0.md`。

---

## 0. 摘要（TL;DR）

DSH 和 agent-vivy 不是同一类产品，能力差距首先是**身份差距**，其次才是
清单差距：

- **DSH** 是一个开发者预览期的 **TypeScript / Cordis 插件 OS**：一切皆插件，
  连 agent loop 本身都可卸可换，配 profile/bundle/overlay 在启动时叠加，
  模型工具面 30+，有 fail-closed 的三平台进程沙箱、上下文 compaction、
  spill、SQLite FTS 会话检索、6 种以上子代理传输（spawn/fork/ACP/dsh-sdk/
  Codex/Claude Code）、模型自写 workflow、自我
  挂载插件（`tool-cordis`）等。它把"组合性"做成产品。
- **agent-vivy** 是一个 **Go 单 EXE 个人网关**：Journal 一等公民、六态
  run 状态机、恰好一次终态、审批与 Ask User 的 first-writer-wins、重启
  恢复、环境变量密钥、编译期插件（`vivy-sdk pack`）、独立 Studio 管理
  开发与分发。它把"可回放、可审、可恢复、人闸"做成产品。

两侧共享同一批工程纪律：**事件溯源的本地会话历史、模型可见 ≡ 已记录、
能力 seam、政策/钩子/Plan Mode 挂在工具管线上、JSON-RPC 控制面、薄 UI**。
差距集中在 DSH 拥有而 Vivy 有意或暂时不做的能力上：**上下文压缩与检索、
OS 级沙箱、热/动态插件装载、子代理/工作流编排、浏览器自动化、LSP、
多接口面（ACP/CLI/Python SDK/Codex/Claude Code hooks）、遥测**。反过来，
agent-vivy 在**终态唯一性、人闸审批、重启恢复、代际账本与气隙评测**上
比 DSH 现货更强或 DSH 根本没有。

结论：差距多数是**有意差距**（哲学锚与内核永不插件化），少数是**待补
差距**（compaction、子代理编排、沙箱、检索——均为 V1+ 能力提案候选）。
本文不主张 Vivy 追赶 DSH 的能力面；它主张把差距分成"应该学/应该拒绝/
可后补"三类分别对待。

---

## 1. 定位对比：两种产品、两种身份

| 维度 | DeepSeek Harness | agent-vivy（物种） |
|---|---|---|
| 产品身份 | 开源 agent harness，插件框架 + 参考实现（`README.md:5-7`） | 个人网关应用（`prd-agent-vivy-v0.md:76-86`） |
| 一句话 | "everything is a plugin"（`docs/architecture.md:9-13`） | "过日子的唯一身体；Journal 的唯一写入者"（`VIVY-STUDIO.md:352-356`） |
| 语言/运行时 | TypeScript，pnpm monorepo，Node ≥22（`package.json`） | Go 1.26 单模块，单二进制 + `go:embed` UI（`go.mod:1`，`ui/embed.go:17-18`） |
| 宿主框架 | 自研（vendor Cordis，论文《Spatiotemporal Composability》） | 第三方 Eino v0.9.13，隔离在 `internal/runtime`+`internal/provider`（D-007） |
| 内核地位 | 无特权内核；loop 也是可卸插件（`docs/architecture.md:9-13`） | 内核永不插件化（Journal/政策/密钥/身份，`SELF-EVOLVING-GATEWAY.md:159-169`） |
| 插件模型 | 运行时热挂/热卸，模型可自写插件（`packages/extensions/tool-cordis`） | 编译期：源码 → `vivy-sdk pack` → 新 EXE（NG-11，`SELF-EVOLVING-GATEWAY.md:226-249`） |
| 发展状态 | 开发者预览 `0.1.0-rc.5`，允许破坏性变更（`README.md:9-11`） | V0/V1 组装完成、Studio bootstrap 完成（`README.md:12-15`） |
| 与对方的关系 | 是 Vivy Studio 第一任开发发动机（`VIVY-STUDIO.md:183-191`） | 是 DSH 的住户产品；不嵌 Node、不依赖 DSH（NG-5） |

DSH 是"能搭出 agent 的组合框架"；agent-vivy 是"一个能过夜的 agent"。
两者不是同一竞品位，但 Vivy 的架构方向（seam、日志不变式、双事件面）
明确以 DSH 为标杆（`SELF-EVOLVING-GATEWAY.md:87-112`）。

---

## 2. 体量与形态总览

| 维度 | DSH | agent-vivy |
|---|---|---|
| 包/模块数 | ~50 个 npm 包 + 3 个 bundle + 6 个 examples + apps/cli + apps/web + python/sdk + native/landlock-run（`packages/README.md`） | 1 个 Go module；`internal/` 13 个包 + `cmd/vivy` + `cmd/vivy-studio` + `sdk/` + `ui/`（`README.md:39-58`） |
| 行数量级 | 数十万行 TS（参考：`.workspace` 全量 64k TS/TSX 跨项目） | 中等体量 Go 单体（约 60+ 源文件，含测试） |
| 配置模型 | cordis.yml + profile + bundle + overlay 四层叠加（`docs/architecture.md:15-37`） | 单一 `config.yaml`，严格解码（`internal/config/config.go:1-13`） |
| 事件词汇 | merge-extensible `SessionEventMap`，逐事件 JSON Schema（`docs/persistence-catalog.md`） | 34 个 `RunEvent` 类型 + 每类型 payload schema（`internal/domain/event.go:8-43`） |
| 持久化 | 事件日志 + JSONL(zstd)/SQLite 双后端（`docs/subsystems/persistence.md:231-237`） | Journal + SQLite（modernc 纯 Go）单后端（D-031） |
| 界面 | Web GUI(:3080) + headless CLI + ACP + JSON-RPC SDK(TS/Python)（`apps/cli/README.md`） | 内置 Web UI(:8787) + JSON-RPC 控制面（`internal/rpc`） |
| 测试文化 | 单文件 100% 覆盖率门、keyless 快照、浏览器快照 CI（`docs/testing.md:9-49`） | `just ci` + CN-01..16 一致性套件 + Playwright 真进程 smoke（`justfile:22-25`） |

---

## 3. 能力矩阵（逐维对比）

### 3.1 架构与扩展模型

| 能力 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| 组合单元 | Cordis 插件（Service + inject + 可逆 effect，`docs/cordis-primer.md:7-44`） | 内核（不可装配）+ 出厂单元（loop/world/provider/tool）+ 用户 plugin（`VIVY-ASSEMBLY.md:42-66`） | 不同范式：运行时树 vs 编译期代 |
| 能力 seam | Service Definition / Provider / Consumer 三件套，~55 个 `ctx` 键（`docs/capability-seams.md:412-469`） | `provider` / `tool-world` / `loop` 等命名 seam（`VIVY-ASSEMBLY.md:47-66`） | Vivy 已有 seam 概念，但粒度粗、不运行时换绑 |
| 装配 | bundle → profile → home → `--patch` 叠加，`--dump-config` 可查（`docs/architecture.md:15-37`） | `vivy.generation.yml` 配方 + `vivy-sdk pack` 编译期叠加（`VIVY-ASSEMBLY.md:74-113`） | 等效概念，执行时点不同 |
| 循环可换 | agent-loop 是可卸插件（`docs/architecture.md:48-52`） | loop 是配方可换的装配单元，物种内特权（`VIVY-ASSEMBLY.md:146`） | 有意差距（内核不插件化） |
| 自修改 | `tool-cordis`：模型定义/挂载/卸载自己的插件（vm 沙箱，opt-in，`packages/extensions/tool-cordis`） | **拒绝**：无运行时装载；`execute` 指向源码是"无门自改写"待封（S8，`SELF-EVOLVING-GATEWAY.md:307-309`） | 有意差距（NG-11；DSH 自己也声明不是安全边界） |

### 3.2 会话、事件与持久化

| 能力 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| 会话历史模型 | append-only `SessionEvent` 日志；模型历史**派生**（`docs/subsystems/session.md:5`） | Journal + Message 投影；历史重建进 Eino feed（ADR-010） | 同构 |
| 模型可见 ≡ 已记录 | 运行时不变式 + 请求头快照 `request/header`（`docs/architecture.md:92-96`） | `model.request` 摘要事件 + 消息投影（ADR-010） | 同构；DSH 更强（整请求可重放） |
| 终态唯一 | 无此概念（turn 可被中断/重试/取消，`docs/agent-lifecycle.md`） | **恰好一次终态**，Journal 层双保险（`internal/storage/sqlite/journal.go:38-46`） | **Vivy 更强**（DSH 无对应物） |
| 重启恢复 | 崩溃后合成 `turn/end {interrupted}`（`docs/subsystems/persistence.md:13-17`） | 启动前 `Service.Recover` 结算每个非终态 run；审批/问题可续（`internal/app/app.go:273-276`） | 同构；Vivy 把"恢复"做成启动门 |
| 上下文压缩 | compaction seam：summary + surface 替换 + tool-result pruner（`docs/subsystems/compaction.md`） | **无**（明确延后，ADR-009/010） | **DSH 有、Vivy 无**（V1+ 提案候选） |
| 超长输出 | spill-to-file + 不透明定位符（`docs/subsystems/spill.md`） | 有界 head/tail + `[UNTRUSTED TOOL OUTPUT]` 折叠（`internal/runtime/tooladapter.go:182-202`） | 同构，机制不同 |
| 会话检索 | SQLite FTS5 全文检索 + 追踪（`packages/session-query/session-query-sqlite`） | 无（FTS 仅提案，`docs/research/hermes-tool-porting-research`） | **DSH 有、Vivy 无** |
| 持久化后端 | JSONL + SQLite 可换（`docs/subsystems/persistence.md:231-237`） | SQLite 唯一后端（fsjournal V1+ 探针，D-031） | 差距小；Vivy 有 conformance 套件保障 |

### 3.3 模型 / Provider 层

| 能力 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| 适配器 seam | `ctx.llm`：任意 adapter 注册，一次请求一次解析（`docs/subsystems/llm-streaming.md:627-702`） | `ProviderRef` + 预烤 YAML bundle（`internal/provider/bundle.go`） | 同构；DSH 开放注册，Vivy 精选目录 |
| 可用 provider | DeepSeek 官方 + pi-ai 两个库适配器；可自加任意 provider（`packages/llm/llm-deepseek`） | OpenAI-compatible（wired）+ **Anthropic 未接** + mock（`internal/provider/catalog.go:37`） | **DSH 有、Vivy 无**（provider 广度是提案制） |
| 流协议 | 统一 `StreamChunk` 封闭联合 + BlockAssembler（`docs/subsystems/llm-streaming.md:154-182`） | domain 流接口 + Eino 映射（`internal/runtime/mapper.go`） | 同构 |
| 密钥 | credential ref（永不落值），per-op 解析（`docs/subsystems/credentials.md`） | env_key 只存名字，运行时取（D-010，`internal/config/config.go:29-31`） | 同构；DSH 支持文件/多源，Vivy 只 env |

### 3.4 工具目录

| 能力 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| 文件系统 | read/write/edit/read_image + str_replace_editor + glob/grep（`docs/tool-catalog.md:16-28`） | read_file/search_files/write_file/patch（`config.example.yaml:74-76`） | 相当；DSH 多 read_image/str_replace |
| Shell | bash / pwsh / 持久 PTY bash + job 后台（`docs/tool-catalog.md:21-28`） | execute/commandline（允许列表、工作区限定、无 shell 语法，`internal/runtime/command_backend.go`） | **DSH 强**：持久终端、后台任务、双方言 |
| Web | web_search/web_fetch（多 provider seam，`docs/subsystems/web.md`） | network_search（5 provider，只读）；http_request（GET/HEAD 白名单，`internal/tools/http_request.go:39-48`） | 相当；Vivy 更保守（只读） |
| MCP | MCP client 桥接，工具按 server 限定（`packages/mcp/mcp-client/README.md`） | mcp_list_tools/mcp_call（审批门，`internal/tools/mcp.go`） | 相当；Vivy 视为"配置依赖"非插件（NG-19） |
| 终端/PTY | terminal_open/list/read/send/signal/close（`docs/tool-catalog.md:28`） | **无** | **DSH 有、Vivy 无** |
| LSP | lsp（goToDefinition/references/impl/hover，`docs/subsystems/lsp.md`） | **物种无**（只在 Studio 工具链，`VIVY-STUDIO.md:187`） | **DSH 有、Vivy 无**（物种侧） |
| 任务/待办 | todo_write（`packages/todo/tool-todo`） | task_create/get/update/list（`internal/tools/todo.go`） | 相当 |
| 计划 | plan mode 软指导 + exit_plan_mode（`docs/subsystems/plan.md:5-35`） | Plan Mode **物理拒绝** effectful 工具（H3，`GOAL-AGENT-HARNESS-ROADMAP.md:71-74`） | 同构但强度不同：Vivy 更硬 |
| 问用户 | ask_user_question（pauses tool call，`docs/tool-catalog.md:18`） | ask_user（独立 QuestionStore，答案=数据，`internal/tools/askuser.go`） | 同构 |
| 代码执行 | run_code（模型写 TS 程序，Code Mode，`docs/tool-catalog.md:19`） | **无**（sequential_thinking 是推理辅助） | **DSH 有、Vivy 无** |
| 技能 | skill 工具 + 分层 provider + 目录发现（`docs/subsystems/skills.md`） | skills_list/view/manage（HITL 修订，`internal/tools/skills.go`） | 相当；Vivy 管理更严（HITL） |
| 自我检查 | cordis_inspect_*（运行时自省，`docs/tool-catalog.md:23`） | species/inspect（只读身份，`internal/studio/inspect.go`） | 同构；范围不同（DSH 内省自身插件树） |

### 3.5 安全与沙箱

| 能力 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| 进程沙箱 | fail-closed：Linux bwrap/Landlock（native C）、macOS Seatbelt、Windows ACL restricted-token（`docs/subsystems/sandbox.md`；`native/landlock-run`） | **无 OS 级沙箱**；仅 per-run 目录隔离 + 遍历/符号链接 fail-closed（`internal/runtime/isolation.go`） | **DSH 有、Vivy 无**（V0 明示非目标，`prd-agent-vivy-v0.md:66`） |
| 沙箱模式 | read-only / workspace-write / danger-full-access（`docs/subsystems/sandbox.md:11-21`） | policy profile：default/plan/read_only/full_auto（`internal/domain/policy.go:6-11`） | 同构概念 |
| 审批 | allowed-once/rejected/cancelled/unavailable，fail-closed（`docs/subsystems/approval.md:21-29`） | 六步写序 + first-writer-wins + 过期 + 可审 proposal（`internal/runtime/service.go:1035-1142`） | 同构；Vivy 更完整（proposal、过期、Review Center） |
| 权限预设 | permission preset 捆绑 sandbox+approval（`docs/subsystems/permission-presets.md:11-27`） | policy profile 语义等价（`internal/runtime/policy.go:47-53`） | 同构 |
| 参数安全 | ToolDefinition 输出 schema + 校验（`docs/subsystems/tools.md:9-23`） | ValidateArgs + security.go 危险参数门（`internal/tools/security.go:41-71`） | 同构 |
| 输出净化 | spill 定位符 + 凭证 scrub（`docs/defensive-patterns.md:27-33`） | 密钥/邮箱脱敏 + 未信任标记 + 有界折叠（`internal/runtime/tooladapter.go:182-202`） | 同构 |

### 3.6 审批、交互与人机协作

| 能力 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| 审批审计 | approval/asked + decided 成对事件（`docs/subsystems/approval.md:11-19`） | 6 种 approval 事件 + 可回放（`internal/domain/event.go:18-22`） | 同构 |
| 跨会话队列 | 无（审批在会话内） | **Review Center**：跨会话 review/list/get/respond（`docs/architecture/hitl-review-center.md`） | **Vivy 更强** |
| Ask User 与审批分离 | user-questions 与 approval 分开（`docs/subsystems/user-questions.md:5-27`） | question 独立于 approval（`internal/tools/askuser.go:12-13`） | 同构 |

### 3.7 上下文管理

| 能力 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| 上下文预算 | token-meter + pressure 事件（`docs/subsystems/token-meter.md`） | max_context_bytes / max_history_messages（`internal/config/config.go:109-111`） | Vivy 只有硬上限，无压力信号 |
| 压缩 | compaction-basic 自动 + `/compact`（`docs/subsystems/compaction.md`） | 无 | **DSH 有、Vivy 无** |
| 注入上下文 | agent.inject() 队列化注入（`docs/architecture.md:120`） | 前置 prompt 组装（MA-2，`internal/runtime/prompt.go`） | 同构 |
| 跨会话引用 | session-reference + session-query（`docs/subsystems/session-reference.md`） | 无 | **DSH 有、Vivy 无** |

### 3.8 编排（子代理 / workflow / goal / jobs / schedule）

| 能力 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| 子代理 | 6+ 种 transport（in-process spawn/fork、ACP、dsh-sdk、Codex、Claude Code）+ continuable 子会话 + 控制工具（`docs/subsystems/subagent.md`） | `vivy worker` 同二进制子 run：父代管模型/工具/审批/政策/预算（`internal/worker/supervisor.go`；GOAL-3..6） | 同构（父治理子），DSH 形态更多 |
| 工作流 | workflow 脚本 + worker-thread 引擎 + ralph（`docs/subsystems/workflow.md`） | **无 graph workflow**（child 树是唯一编排，`prd-agent-vivy-v0.md:65`） | **DSH 有、Vivy 无** |
| 目标 | 同会话 goal（revisioned phase + 轮次上限，`docs/subsystems/goal.md`） | 无 | **DSH 有、Vivy 无** |
| 后台任务 | 通用 jobs 注册表 + job_* 控制（`docs/subsystems/jobs.md`） | 后台 run：list/attach/logs/recover/cancel（H8） | 同构 |
| 定时 | schedule（session-local 提醒，`docs/subsystems/schedule.md`） | 无（scheduler 延后，`IMPLEMENTATION-PLAN.md:464`） | **DSH 有、Vivy 无** |
| 预算 | 无显式预算账本（有 timeout/guard） | **预算账本**：events/model_calls/tool_calls/retries，父子共享（`internal/runtime/budget.go`） | **Vivy 更强** |

### 3.9 接口

| 能力 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| CLI | `dsh --profile` / headless 一次性任务（`apps/cli/README.md:7-16`） | `vivy.exe` 无子命令面；`vivy-sdk` / `vivy-studio` 独立二进制（ADR-017） | 不同定位 |
| Web GUI | 完整产品 UI：设置/模型/工作区/插件目录（`docs/user/guide/index.md:5-23`） | 薄 UI shell：会话/流式/审批/Review Center（`ui/src/features/*`） | 差距大（功能面） |
| 自动化协议 | ACP 服务器（fresh sessions，`packages/acp/acp/README.md:5-81`） | ACP **提案 only**（`docs/architecture/ACP-REMOTE-CONTROL-PROPOSAL.md:3`） | **DSH 有、Vivy 无** |
| SDK | JSON-RPC SDK（TS + Python，`packages/sdk`；`python/sdk`） | JSON-RPC 控制面（`internal/rpc`），无对外 SDK | 差距中 |
| 外部 agent 桥 | Codex / Claude Code hooks 桥（`packages/hooks`） | 无（把 hook 语义内置为 policy/hook 链） | **DSH 有、Vivy 无**（Vivy 用不上） |
| RPC 类型 | Typert 类型图生成（`docs/api-gateway.md`） | 手写 JSON-RPC 方法（`internal/rpc/control.go`） | 差距中（工程性） |

### 3.10 可观测性与 DX

| 能力 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| 遥测 | session-telemetry + OTel，默认关（`docs/subsystems/session-telemetry.md`） | 事件全落 Journal，无 OTel 导出 | 差距中 |
| 反馈 | message-feedback + /feedback（`docs/subsystems/feedback.md`） | 无用户反馈面 | **DSH 有、Vivy 无** |
| 会话标题 | 自动标题（`docs/subsystems/session-title.md`） | 会话 title 字段（`internal/domain/session.go`） | Vivy 无自动生成 |
| 投影 | session-projection 折叠单元（`docs/subsystems/session-projection.md`） | ReviewItem 投影（`internal/domain/review.go`） | 同构，范围不同 |
| 运行时不变量 | ctx.invariants 每包注册（`docs/subsystems/invariants.md`） | ADR guard 测试 + conformance | 同构 |

### 3.11 成熟度与工程纪律

| 维度 | DSH | agent-vivy | 差距方向 |
|---|---|---|---|
| 发布阶段 | 开发者预览，允许破坏性变更（`README.md:9-11`） | V0/V1 组装完成，Studio 切换完成（ST-6） | 都早；Vivy 承诺更稳 |
| 覆盖率 | 单文件 100% 门（`docs/testing.md:10`） | go test -race + CN 套件 + Playwright | 都强，形态不同 |
| 快照测试 | keyless 会话快照 + 浏览器快照 CI（`docs/testing.md:12-13`） | Playwright mock 会话 smoke（`ui/e2e/smoke.spec.ts`） | DSH 更系统 |
| 生成文档 | tool/config/persistence 目录生成 + 保鲜门（`docs/graph-atlas.md`） | 手写 schema + README | DSH 更强 |
| 事故文化 | postmortem + agent notes（`docs/postmortem/`） | 接受日志 + ADR（`docs/logs/`） | 同构 |

---

## 4. 差距清单（按缺口类型）

### 4.1 DSH 有、agent-vivy 没有（且当前无实现）

| # | 能力 | Vivy 对应现状 | 证据 |
|---|---|---|---|
| G1 | 上下文压缩（compaction） | 无；原始但有界 feed | `AGENT-VIVY-ARCHITECTURE-V0.md:144-146` |
| G2 | 会话全文检索（FTS5） | 无 | `hermes-tool-porting-research`（仅提案） |
| G3 | OS 级进程沙箱 | 无（仅目录隔离+政策） | `prd-agent-vivy-v0.md:66` |
| G4 | 持久 PTY / 终端工具 | 无 | `config.example.yaml`（工具清单无 terminal） |
| G5 | LSP 语义导航（物种侧） | 无（仅 Studio 工具链） | `VIVY-STUDIO.md:187` |
| G6 | 代码执行（run_code / Code Mode） | 无 | `docs/tool-catalog.md:19` |
| G7 | 子代理多形态 + 可续子会话 | 仅同二进制 worker child | `GOAL-AGENT-HARNESS-ROADMAP.md:124-145` |
| G8 | 模型自写 workflow / ralph | 无 | `docs/subsystems/workflow.md` |
| G9 | 同会话 goal 目标管理 | 无 | `docs/subsystems/goal.md` |
| G10 | 定时提醒（schedule） | 无 | `IMPLEMENTATION-PLAN.md:464` |
| G11 | ACP 自动化协议 | 提案 only | `ACP-REMOTE-CONTROL-PROPOSAL.md:3` |
| G12 | 对外 SDK（TS/Python） | 无 | `packages/sdk`、`python/sdk` |
| G13 | Codex / Claude Code hooks 桥 | 无（不适用） | `packages/hooks` |
| G14 | OTel 遥测导出 | 无 | `docs/subsystems/session-telemetry.md` |
| G15 | 用户反馈面 | 无 | `docs/subsystems/feedback.md` |
| G16 | 自动会话标题 | 无 | `docs/subsystems/session-title.md` |
| G17 | 跨会话引用/检索注入 | 无 | `docs/subsystems/session-reference.md` |
| G18 | provider 广度（>2） | 仅 openai-compatible 可用 | `internal/provider/catalog.go:37` |
| G19 | 多持久化后端 | 仅 SQLite | `prd-agent-vivy-v0.md:450` |
| G20 | 全请求重放（request/header 快照） | 仅摘要事件 | `docs/subsystems/session.md:152-176` |

### 4.2 DSH 有、agent-vivy 有但形态不同

| 能力 | DSH 形态 | Vivy 形态 | 说明 |
|---|---|---|---|
| 事件溯源 | 会话日志 + 派生历史 | Journal + 消息投影 | 同构，Vivy 有终态唯一 |
| 审批 | 一次性 allowed-once | proposal + 过期 + first-writer-wins + Review Center | Vivy 更完整 |
| 沙箱模式 | 3 种文件效果模式 | 4 种 policy profile | 概念等价 |
| 插件 | 运行时热挂 | 编译期 pack | 有意差距 |
| 后台任务 | jobs 注册表 | 后台 run 管理 | 同构 |
| 预算 | timeout/guard | 显式账本（更强） | Vivy 更强 |
| 工具安全 | schema + scrub | schema + security 门 + 脱敏 | 同构 |

### 4.3 DSH 有、agent-vivy 有意拒绝（哲学，不是欠账）

| 能力 | DSH | Vivy 拒绝理由 | 决策号 |
|---|---|---|---|
| 一切皆插件（含 loop/log） | 核心卖点 | 环境不能是种群成员 | NG-7 |
| 运行时自我修改 | tool-cordis（opt-in） | 生产实例无门自改写；不是安全边界 | NG-11；`SELF-EVOLVING-GATEWAY.md:307` |
| 社区插件市场 | dsh-plugin 发现 | 精选目录锚 | PRD §5.0.3 |
| 多租户/托管 | 无（但可扩展） | 个人网关锚 | D-016 |
| 微服务/网格 | 无（但可扩展） | 本机不是集群 | NG-8 |
| Node 进热路径 | 本身就是 Node | 单 EXE Windows 一键 | NG-2 |

### 4.4 agent-vivy 有、DSH 没有或更弱

| 能力 | Vivy | DSH 现状 | 说明 |
|---|---|---|---|
| 恰好一次终态 | Journal 层双保险 | 无此概念 | DSH 允许 turn 被中断/重试 |
| 重启恢复门 | 启动前结算所有 run | 崩溃日志修复（不结算 run） | Vivy 产品化更强 |
| 审批完整性 | proposal + 过期 + 跨会话队列 | 一次性 + ask/never | Vivy 更强 |
| 预算账本 | 父子共享、不可放宽 | 只有 timeout/guard | Vivy 更强 |
| 代际账本 | Studio Generation/EvalRun/Release/Install | 无跨代适应度 | `VIVY-STUDIO.md:196-240` |
| 气隙评测 | Studio 拉起候选、独立数据目录 | 无 | NG-24 |
| 密钥纪律 | env-only、审计测试 | 多源但无 OS keychain（同样延迟） | 同强 |
| 编译期插件出处 | 哈希 + generation.json | 无 | `sdk/internal/pack.go:179-197` |

---

## 5. 结构性差距：插件 OS vs 个人网关

这不是清单差异，而是两条架构哲学的分岔：

1. **组合性的位置**。DSH 把"动态组合"放在运行时（Cordis 可逆 effect +
   响应式 coeffect），Vivy 把"组合"放在编译期（pack 出一代 EXE）。
   前者换来插件可热卸、模型可自演化；后者换来 Journal/政策/密钥永不
   被卸、失败突变体不能拆掉日常（NG-3）。
2. **谁的信任根**。DSH 的信任根是"组合正确性"（无特权内核）；Vivy 的
   信任根是"人闸 + 可回放"（内核永不插件化，`SELF-EVOLVING-GATEWAY.md:159-169`）。
3. **进化路径**。DSH 的进化是"活进程挂零件"（重启即散，官方声明非安全
   边界，`SELF-EVOLVING-GATEWAY.md:94`）；Vivy 的进化是"新 EXE + Studio
   评测 + 人发布"（NG-15、NG-25）。
4. **过夜承诺**。DSH 是开发者预览，允许破坏性变更；Vivy 的哲学锚要求
   住户产品今天能用、能审、能恢复（`prd-agent-vivy-v0.md:76-99`）。

所以差距分析的正确读法是：**G1..G20 中多数能力 DSH 以"插件/组合"形态
拥有，Vivy 若引入，应以"物种内一等公民"形态重做，而不是把 DSH 的能力
面搬进来**（NG-7：学纪律，拒身份）。`SELF-EVOLVING-GATEWAY.md:118-136`
给出过量化评估：按支柱计，物种只做 DSH 核心理念的 35–45%，加上 Studio
发动机后"对人的效果"约 70–80%，而"过夜可积累的进化"可以高于 DSH 现货。

---

## 6. 结论与建议

### 6.1 三类差距的处理

| 类别 | 条目 | 建议 |
|---|---|---|
| 应该学（纪律） | 模型可见 ≡ 已记录（已完成 ADR-010）；seam 命名；双事件面；guard/不变式 | 已采纳，继续按 DSH 纪律演进（NG-7） |
| 可后补（能力提案候选） | G1 压缩、G2 检索、G3 沙箱、G7 子代理形态、G12 SDK、G14 遥测 | 按能力提案流程（`AGENT-VIVY-DIRECTION.md`）逐项立项；V1+ 建议优先 G1/G3/G7 |
| 有意拒绝 | 热插件、自修改、插件市场、Node 热路径、多租户 | 维持 NG-2/NG-7/NG-11/NG-15，除非另行决策 |

### 6.2 给 Vivy 的定位结论

- DSH 是**组合框架**的参考上限，不是**个人网关**的竞品；Studio 用它当
  发动机即可，物种不需要 DSH 的能力面。
- Vivy 相对 DSH 的**真实优势**（终态唯一、人闸审批、重启恢复、预算账本、
  代际气隙评测）应作为物种身份继续强化，而不是去补齐 DSH 的清单。
- 差距最大的领域（上下文管理 G1/G2、编排 G7/G8、沙箱 G3）恰好是 V1
  Operate 阶段"让日子更好过"的候选提案；建议以"物种内一等公民"形态
  提出，逐个走证据 → 提案 → 合同 → 实现 → 真路径验证的流程
  （`GOAL-AGENT-HARNESS-ROADMAP.md:50-56`）。

---

## 附录 A：工具清单对照（模型可见面）

### DSH（30+，`docs/tool-catalog.md` 生成目录）

ask_user_question · run_code · exit_plan_mode · bash · pwsh · bash(持久) ·
cordis_define/run/stop/undefine/inspect_* · str_replace_editor · read/write/
edit/read_image · glob/grep · terminal_open/list/read/send/signal/close ·
create_goal/get_goal/update_goal · schedule_create/list/delete · lsp ·
workflow · ralph · skill · session_event_read/search/trace · session_search/
trace · subagent/subagent_fork · interrupt_agent/list_agents/send_message ·
report · job_kill/list/output · todo_write · web_search/web_fetch ·
mcp__<server>__<raw>

### agent-vivy（24，`config.example.yaml:66-90`）

echo_info · write_note · list_notes · read_note · ask_user · read_file ·
search_files · write_file · patch · http_request · mcp_list_tools · mcp_call ·
sequential_thinking · execute · commandline · skills_list · skill_view ·
skill_manage · task_create · task_get · task_update · task_list ·
network_search · tool_search

---

## 附录 B：证据索引

### DSH（相对 `.workspace/deepseek-harness/deepseek-harness`）

- 定位：`README.md:5-11`；预览声明 `README.md:9-11`
- 架构：`docs/architecture.md:9-37,39-52,63-96,98-104`
- Cordis：`docs/cordis-primer.md:7-44`；vendor `vendor/README.md`
- seam 表：`docs/capability-seams.md:412-469`
- 会话/持久化：`docs/subsystems/session.md`、`persistence.md`、
  `persistence-catalog.md`
- 压缩/溢出/检索：`docs/subsystems/compaction.md`、`spill.md`、`session-query.md`
- LLM：`docs/subsystems/llm-streaming.md`；`packages/llm/llm-deepseek/README.md`
- 工具：`docs/tool-catalog.md`（生成目录）
- 沙箱/审批：`docs/subsystems/sandbox.md`、`approval.md`、`permission-presets.md`；
  `native/landlock-run/README.md`
- 编排：`docs/subsystems/subagent.md`、`workflow.md`、`goal.md`、`jobs.md`、`schedule.md`
- 接口：`apps/cli/README.md`、`packages/acp/acp/README.md`、`packages/sdk/README.md`、
  `python/README.md`、`packages/hooks/README.md`、`docs/api-gateway.md`
- 工程：`docs/testing.md`、`.github/workflows/ci.yml`、`BENCHMARK.md`
- 明确 POC/部分：`packages/e2b/README.md`、`packages/code-runtime/code-runtime/README.md`

### agent-vivy（相对仓库根）

- 定位/哲学：`prd-agent-vivy-v0.md:76-141`；`AGENT-VIVY-DIRECTION.md`
- 架构/ADR：`docs/AGENT-VIVY-ARCHITECTURE-V0.md`（ADR-001..018）
- 世界观/Studio：`docs/architecture/VIVY-WORLDVIEW.md`、
  `VIVY-STUDIO.md`、`SELF-EVOLVING-GATEWAY.md`、`VIVY-GATEWAY-AND-STUDIO.md`
- 装配/插件：`VIVY-ASSEMBLY.md`、`VIVY-PLUGIN-SPEC.md`、`sdk/plugin/plugin.go`、
  `sdk/internal/pack.go`
- 运行时：`internal/runtime/service.go`、`engine.go`、`tooladapter.go`、
  `budget.go`、`policy.go`、`preflight.go`、`checkpoint.go`
- 领域/事件：`internal/domain/event.go`、`run.go`、`policy.go`；
  `schemas/events/run-event.schema.json`
- 存储：`internal/storage/contracts.go`、`internal/storage/sqlite/*`
- 工具：`internal/tools/*.go`、`config.example.yaml`
- 控制面：`internal/rpc/protocol.go`、`control.go`、`websocket.go`
- worker：`internal/worker/supervisor.go`、`server.go`；`internal/app/worker.go`
- Studio：`cmd/vivy-studio/main.go`、`internal/studiocore/service.go`、
  `internal/eval/*`
- 路线/清单：`docs/GOAL-AGENT-HARNESS-ROADMAP.md`、`docs/IMPLEMENTATION-PLAN.md`、
  `docs/TODO.md`
