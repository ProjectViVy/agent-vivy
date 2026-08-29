# DSH 插件锚点：Vivy 特性缺口调研

> 状态：**研究报告**（分析文档，不是产品合同；不新增或改写 ADR/NG）。
> 日期：2026-08-29
> 目的：以 DeepSeek Harness（DSH）**用插件/包声明的特性**为坐标系，盘点
> Vivy 物种（`vivy.exe`）缺什么、以什么形态缺、以及是否该学。
> DSH 证据：`.workspace/deepseek-harness/upstream/`（HEAD `47f9438`，
> `0.1.0-rc.5`，与既有能力差距文同源）。
> 相关：
> [`DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md`](./DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md)
> （按能力维）、[`VIVY-ASSEMBLY.md`](../architecture/VIVY-ASSEMBLY.md)、
> [`SELF-EVOLVING-GATEWAY.md`](../architecture/SELF-EVOLVING-GATEWAY.md)、
> [`VIVY-GATEWAY-AND-STUDIO.md`](../architecture/VIVY-GATEWAY-AND-STUDIO.md)、
> [`docs/TODO.md`](../TODO.md) §0.1。

---

## 0. 摘要（TL;DR）

DSH 的产品特性几乎都以 **npm 包 = Cordis 插件** 的形式挂进运行时树
（`packages/<group>/<pkg>`，见 `packages/README.md`）。读 DSH 缺什么，
最稳的办法不是背能力口号，而是 **按插件清单逐行问 Vivy：有没有对等的
一等公民、出厂单元或用户 plugin seam**。

| 读法 | 既有 GAP 文（2026-08-16） | 本文 |
|---|---|---|
| 组织轴 | 能力维（会话/工具/沙箱/编排…） | **DSH 插件/包组**（plan、todo、goal、workflow…） |
| 问题 | Vivy 缺哪些能力 | Vivy 相对「DSH 用插件声明的特性」缺哪些**可命名单元** |
| 产出 | 身份对比 + G1–G20 | 插件地图 → 分类缺口 + 提案候选优先级 |
| 关系 | 保留 | **互补**；交叉引用，不取代 |

**粗统计（按 packages 组内正式产品包，POC/support 另计）：**

| 分类 | 含义 | 量级（约） |
|---|---|---|
| Present | Vivy 已有对等一等实现 | 少部分核心（session/journal、tools 管线、approval、ask_user、skills、mcp、fs 子集、todo 子集） |
| Analog | 有，但形态/强度不同 | 多（plan、sandbox 文件效果、worker vs subagent、budget vs guard、pack vs hot plugin） |
| Gap-Learn | 物种内尚无；可按 Vivy 身份重做的候选 | 本文重点（compaction、query、goal、terminal、workflow…） |
| Gap-Refuse | DSH 有；Vivy **有意不做** | 热插件、tool-cordis、市场、Node 热路径、loop/log 可卸… |
| Vivy-Stronger | Vivy 明显更完整或 DSH 无对应 | 终态唯一、Recover 门、Review Center、预算账本、代际/气隙、plan 物理拒绝 |

**前 10 个最值得关注的 Gap-Learn（调研建议，非实现承诺）：**

1. **compaction**（上下文压缩）— ADR 已 defer；长会话过夜刚需
2. **session-query / FTS** — 跨回合检索与轨迹面板的数据根
3. **token-meter 压力信号** — Vivy 仅有硬上限，无 pressure 事件
4. **goal**（同会话目标对象）— UI-GOAL 已挂板；进度条不是 goal
5. **terminal / 持久 PTY** 或等价长会话 shell — 仅有一次性 execute
6. **jobs 通用化** — 有后台 run，无模型面 `job_*` 注册表
7. **workflow / ralph 是否需要 Vivy-native 形态** — 现仅有 worker 子 run 树
8. **OS 级进程沙箱** — SBX-OS 已 DEFERRED；文件 sandbox ≠ OS sandbox
9. **ACP 实现** — 提案存在（ACP-1 DEFERRED）
10. **session-title 自动生成 / spill 定位符** — 体验与超长输出

**一句话结论：** 按插件锚点看，Vivy 缺的不是「再做一个 Cordis」，而是一批
**可命名的物种内能力单元**；DSH 用插件挂上的东西，Vivy 若引入，必须变成
内核一等公民、出厂 tool/world，或用户 `plugins/` + pack 代——**不能**热挂进
活进程（NG-7 / NG-11 / NG-15）。

---

## 1. 方法与读法

### 1.1 锚点定义

对 DSH 的每一行产品特性，优先取这三层之一作为锚：

1. **包组**（`packages/README.md` 表）— 能力家族边界
2. **具体包**（`packages/<group>/<pkg>`）— 可装卸的声明单元
3. **模型可见工具名**（`docs/tool-catalog.md`）— 人/模型实际碰到的面

子系统参考页（`docs/subsystems/*.md`）提供类型与语义；架构总览见
`docs/architecture.md`（everything is a plugin；profile/bundle 叠加）。

### 1.2 五分类标签

| 标签 | 判定 |
|---|---|
| **Present** | Vivy 物种内有可指认的实现，语义大体同构 |
| **Analog** | 有对应物，但执行时点、强度、范围不同（必须写清差在哪） |
| **Gap-Learn** | 物种内无对等一等公民；**可以**按 Vivy 哲学重做（提案候选） |
| **Gap-Refuse** | DSH 有；Vivy 文档/NG 已拒绝或与身份冲突（不是欠账） |
| **Vivy-Stronger** | Vivy 更完整，或 DSH 无同级概念 |

附加修饰：

- **Analog-Studio** — 仅 Studio/开发发动机侧有，物种日常路径无
- **User-plugin-capable** — 理论上用户 `plugins/` + pack 可补，但出厂未提供
- **Proposal-only** — Vivy 已有书面提案，未采纳/未实现

### 1.3 哲学边界（写入分类的硬约束）

摘自 `SELF-EVOLVING-GATEWAY.md` / `VIVY-GATEWAY-AND-STUDIO.md` / `VIVY-ASSEMBLY.md`：

| 纪律 | 含义 | 对本文的影响 |
|---|---|---|
| NG-7 | 学 DSH 纪律，拒 DSH 身份 | 不建议「把某 dsh-* 包搬进 Vivy」 |
| NG-11 | 拒绝 WASM/dll/Go plugin/stdio 外置插件 exe；能力经 pack 进新 EXE | 热挂 = Gap-Refuse |
| NG-15 | 加减能力 = 新 Generation + Studio eval | 「缺插件」≠「缺运行时 add」 |
| NG-2 | 不把 Node 嵌进物种热路径 | client/host 全家不进物种内核缺口 |
| PRD §5.0.3 | 精选目录，非社区市场 | dsh-plugin 发现 = Gap-Refuse |
| 装配 | 按所是命名；只有用户层叫 plugin | 出厂缺口应叫 tool/world/loop，不叫「缺插件」 |

### 1.4 与 G1–G20 的交叉

既有 GAP 文 §4.1 的 G1–G20 是能力维清单。本文附录 C 给出 **插件锚 → G#**
对照。若本文与 2026-08-16 数字冲突（例如工具数量），以本文重新计数为准，
并注明增量。

### 1.5 范围

- **默认：Vivy 物种**（kernel + 出厂 tool/UI 控制面）。用户只说「Vivy」时不
  把 Studio 换皮当补齐路径。
- Studio 仅在「Vivy-Stronger / Analog-Studio」旁注出现。
- 不产生实现排期；优先级是调研建议。

---

## 2. DSH 插件地图（按 packages 组）

每组表列：代表包 | 职责/对外面 | Vivy 映射 | 分类 | 证据。

### 2.1 core — 产品 API 脊骨

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `core/session` | append-only SessionEvent 日志 | Journal + RunEvent（`internal/domain/event.go`，`internal/storage/sqlite`） | **Analog**（Vivy 另有 exactly-one-terminal） |
| `core/agent` | Agent 句柄、inbox、拦截 | `internal/runtime/service.go` Run/Cancel/Recover | **Analog** |
| `core/agent-loop` | 默认可卸 loop 驱动 | Eino 封装于 `internal/runtime`；配方可换 loop 概念见 ASSEMBLY | **Analog**（loop 可装配但非热卸插件） |
| `core/tools` | 工具注册 + 执行管线 | `internal/tools` + `tooladapter` + policy/hooks | **Present** |
| `core/system-prompt` | prompt section 装配 | `internal/runtime/prompt.go` | **Analog**（无独立 section 注册 seam） |
| `core/scope` | per-agent 作用域注册 | 无对等原语；工具在进程/配方级 | **Gap-Learn**（低优先；编译期装配弱化需求） |
| `core/agent-default-model` / `agent-tool-presentation` | 默认模型与工具 UI 意图 | UI 手写渲染；无 tool presentation 契约 | **Analog** / 部分 **Gap-Learn**（DX） |

**组摘要：** 脊骨同构度最高。Vivy 用 Journal/六态 run 换 DSH 的可卸 loop；
缺的是细粒度 scope 与 prompt-section 插件化，而非「没有 agent」。

### 2.2 plan + todo — 用户点名的范例

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `plan/plan-mode` | **软指导**：`plan/mode` 日志状态 + `plan:policy` section + `exit_plan_mode` 经 user-questions 审出 + `/plan` | `domain.RunModePlan` + policy profile plan→**Deny** effectful（物理拒绝） | **Analog（强度相反方向）** |
| `todo/tool-todo` | `todo_write` 整表替换；`todo/write` 事件；turn 开始可清空 standing plan | `task_create/get/update/list`（`internal/tools/todo.go`）；session 投影 + UI 只读 | **Analog** |
| `client/ui-plan` / conversation todo 条 | Web 计划条 | 聊天区 plan/todo 展示（2026-08-29 log）；人不可改 todo（UI-TODO-MUTATE） | **Analog** |

**语义差（必须写清）：**

| 维度 | DSH plan | Vivy plan |
|---|---|---|
| 强制力 | 软指导；sandbox/approval **独立**配置，不读 plan 状态 | **物理安全边界**：plan 模式 effectful 直接 `ErrPlanModeToolDenied`，且不能与宽松 profile 组合（`runmode.go`） |
| 退出 | `exit_plan_mode` 交完整 markdown 计划 → 人审 → 静默 pending exit | 无独立 exit 工具；模式是 run 选项 / UI toggle，不是协作状态机 |
| 持久 | `plan/mode` 可 fold 恢复 | `run.started.mode` 标记；非独立 plan 对象 RPC（历史 UI 迁移文档曾列 plan/* 缺口） |

| 维度 | DSH todo | Vivy todo |
|---|---|---|
| API | 单工具整表 replace | 四工具 CRUD + 依赖字段 |
| 生命周期 | 常与 turn 绑定的 standing plan | 会话持久任务；人闸编辑未开放 |
| 并行 in_progress | 部署可选 `allowParallelInProgress` | 由模型/任务依赖约束，无同名开关 |

**组摘要：** plan/todo **不是 Gap**——是 **Analog**。若「学 DSH plan」，学的应是
*协作退出弧线 / 日志状态*，不是把 Vivy 的物理 Deny 改软。若「学 DSH todo」，
学的是投影生命周期与 UI 契约，不是再做一个 `todo_write` 别名。

### 2.3 goal + schedule + jobs

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `goal/goal` + `tool-goal` + `goal-round-driver` + `command-goal` | 同会话 goal：revisioned phase、active/paused/blocked/complete、轮次驱动 | 无 goal 对象；UI 用 in_progress task 冒充概览（**UI-GOAL**） | **Gap-Learn** |
| `schedule/schedule` | 会话内定时提醒 | 无（IMPLEMENTATION-PLAN 延后） | **Gap-Learn** |
| `jobs/jobs` + `jobs-local` + `tool-jobs` | 通用后台 job 注册表 + `job_list/output/kill` | 后台 run：list/attach/logs/recover/cancel（H8）；无模型面通用 jobs | **Analog** → 部分 **Gap-Learn**（模型可控 job 面） |

**组摘要：** goal 是清晰内核缺口；schedule 次之；jobs 已有 run 管理，缺的是
「任意生产者注册 + 模型 job_*」这一层抽象。

### 2.4 interaction — 人机协作

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `interaction/user-approval` | 一次性审批 seam | 六步写序 + first-writer-wins + 过期 + proposal（`service.go`） | **Vivy-Stronger** |
| `interaction/user-questions` + `tool-ask-user` | 问答与审批分离 | `ask_user` + QuestionStore | **Present** |
| `interaction/permission-presets` | sandbox+approval 捆绑预设 | sandbox mode + approval policy + 权限三段 UI（cautious/smart/trust） | **Analog** |
| `interaction/commands` | `/` 人命令注册表 | 无通用 command registry（slash 面弱） | **Gap-Learn**（低–中） |
| Review 跨会话 | （DSH 会话内） | **Review Center** 跨会话队列 | **Vivy-Stronger** |

**组摘要：** 人闸是 Vivy 身份优势区。可学的是 commands 注册与 preset 热切换
（SBX-LIVE 仍 DEFERRED：进行中 run 不热切）。

### 2.5 sandbox + fs + shell + subprocess + terminal

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `sandbox/sandbox*` + windows-acl + native landlock | **OS 级**进程监禁 fail-closed | 工作区路径隔离 + 命令白名单 + HTTP 策略（`isolation` / `SandboxManager`）；**SBX-OS** DEFERRED | **Gap-Learn**（已挂板） |
| `sandbox` 三模式 | read-only / workspace-write / danger-full-access | 同名文件效果模式（`config.example.yaml` sandbox）+ policy profile | **Analog**（文件层 Present，OS 层 Gap） |
| `fs/*` + `tool-fs` + search + str_replace_editor | read/write/edit/glob/grep/image | `list_dir/read_file/search_files/write_file/patch` | **Analog**（缺 read_image、独立 glob/grep 工具、str_replace_editor） |
| `shell/*` + bash/pwsh + persistent | 双方言 shell + 持久 bash | `execute`/`commandline`：无 shell 语法、白名单 argv | **Analog**（Vivy 更保守） |
| `subprocess/*` | 显式 spawn spec + DSH_* 环境 | 进程在 command backend 内；无独立 subprocess seam | **Analog** |
| `terminal/*` + `tool-terminal` | 持久 PTY：open/list/read/send/signal/close | **无** | **Gap-Learn** |

**组摘要：** 名称都叫 sandbox 时最易误判——Vivy 已对齐 **文件效果三档**，未对齐
**OS 监禁**与 **PTY**。

### 2.6 code-runtime + lsp + skill + mcp

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `code-runtime/*` + run_code | 模型写 TS，worker-thread 执行（Code Mode） | 无；`sequential_thinking` 仅推理辅助 | **Gap-Learn**（安全争议大，宜后） |
| `lsp/*` + tool-lsp | goToDef/refs/impl/hover | 物种无；Studio 工具链可有 | **Gap-Learn (species)** / **Analog-Studio** |
| `skill/*` + tool-skill | 发现、加载、目录 | `skills_list/view/manage` + HITL | **Present**（管理更严） |
| `mcp/mcp-client` | MCP 桥，按 server 限定工具 | `mcp_list_tools`/`mcp_call` + 审批；NG-19 配置依赖非插件 | **Present**；管理 UI 仍 demo（UI-MCP） |

### 2.7 compaction + spill + context + token-meter

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `compaction/*` + command-compact | summary + surface 替换 + tool-result pruner | **无**；ADR-009/010 明确 defer | **Gap-Learn**（P0 候选） |
| `spill/*` | 超长输出落盘 + 不透明 locator | 有界 head/tail + `[UNTRUSTED TOOL OUTPUT]` 折叠 | **Analog**（机制弱于 spill） |
| `context/agent-instructions` | 工作区指令注入 | AGENTS.md / prompt 组装 | **Analog** |
| `context/session-reference` | 跨会话结构化引用 | 无 | **Gap-Learn** |
| `context/time-context` / `tmux-context` | 时间/tmux 上下文 | 无专用包 | **Gap-Learn**（低） |
| `llm/token-meter` | 不可变计量 + pressure | max_context_bytes / max_history_messages 硬顶；dashboard token 统计已接 Journal | **Analog**（有用量账本，无 pressure/compaction 触发） |

**组摘要：** 这是「过夜长会话」最大的真实欠账带：压缩 + 检索 + 压力信号。

### 2.8 subagent + workflow + extensions

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `subagent/*`（in-process spawn/fork、ACP、dsh-sdk、Codex、Claude Code）+ control/report 工具 | 多传输子代理 + 可续子会话 | `internal/worker` 同二进制子 run；父代管模型/工具/审批/预算 | **Analog**（形态少；父治理同构） |
| `tool-subagent` / control / report | `subagent`/`subagent_fork`/`send_message`/… | 无对等模型工具名；worker API 偏控制面 | **Gap-Learn**（模型委托 UX） |
| `workflow/*` + `tool-workflow` + `tool-ralph` | 模型自写 workflow 脚本 + ralph 新代理轮 | **无** graph workflow（prd non-goal）；child 树唯一编排 | **Gap-Learn** 或长期 **Gap-Refuse-until-proposal** |
| `extensions/tool-cordis` 等 | 模型定义/挂载/卸载自己的插件 | **拒绝** | **Gap-Refuse**（NG-11） |
| `guard/repeat-tool-reminder` + `timeout-policy` | 循环卫生 | max_tool_turns + budget ledger + hooks | **Analog**；budget **Vivy-Stronger** |

### 2.9 session 数据面家族

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `session/session-persistence` + jsonl + sqlite | 双后端耐久 | SQLite Journal 唯一（fsjournal 探针延后） | **Analog** |
| `session/session-projection*` | 投影单元（todos 等） | ReviewItem 等；todos 有 RPC 投影 | **Analog** |
| `session/session-title*` | 自动标题 | title 字段；无自动 LLM 标题 | **Gap-Learn** |
| `session/session-telemetry` + otel | 出站遥测 | 事件落 Journal；无 OTel | **Gap-Learn**（中低） |
| `session/session-stats` / checkpoint-policy | 统计与检查点策略 | token stats RPC；checkpoint 恢复路径存在 | **Analog** |
| `session-query/*` + tool-session-query | FTS、trace、关系过滤 | 无；轨迹 UI 为 demo（UI-TRAJ） | **Gap-Learn** |
| `session-query/session-log-export` | 导出 | 无一等导出 | **Gap-Learn**（低） |

### 2.10 web + attachment + credentials + settings + storage + workspace

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `web/*` + tool-web | web_search/web_fetch 多 provider | `network_search`（多 provider）+ `http_request`（GET/HEAD 白名单） | **Analog**（Vivy 更保守） |
| `attachment/*` | 内容寻址附件 | 附件 UI stub（UI-CHAT-TOOLBAR） | **Gap-Learn** |
| `credentials/*` | CredentialRef 多源解析 | env_key 名 only（D-010） | **Analog**（更窄更硬） |
| `settings/*` | 用户设置 seam + 热提交 | settings RPC + yaml overlay 部分面 | **Analog** |
| `storage/*` | 非会话存储 hub | SQLite 域内聚；无通用 storage domain 插件 | **Analog** |
| `workspace/*` | 工作区实体注册 | `workspace_root` + per-run 沙箱目录 | **Analog** |

### 2.11 llm + preset + bundle + hooks + identity + feedback

| DSH 包 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `llm/llm` + deepseek + pi-ai + retry | 适配器 seam | openai-compatible + mock；Anthropic 等广度提案制 | **Analog** / provider 广度 **Gap-Learn** |
| `preset/agent-presets` + persona | 每会话 preset cordis.yml | 无 per-session agent 组合预设 | **Gap-Learn**（中；与 pack 代冲突需小心设计） |
| `bundle/base|web-app|headless` | 可安装 profile 层 | `vivy.generation.yml` + pack | **Analog**（编译期） |
| `hooks/hooks-claude-code` + codex | 外桥 | 无；策略/hook 内化 | **Gap-Refuse**（物种用不上）或极低优 |
| `identity/anonymous-user-id` | 匿名身份 | 单用户个人网关假设 | **Analog** / N/A |
| `feedback/*` | 消息反馈 | 无 | **Gap-Learn**（低） |

### 2.12 接口面：api / sdk / acp / host / client / boot

| DSH 包组 | 职责 | Vivy 映射 | 分类 |
|---|---|---|---|
| `api/*` + `typert/*` | BFF + 类型图 RPC | 手写 JSON-RPC（`internal/rpc`） | **Analog**（工程性差距） |
| `sdk/*` + `python/sdk` | 对外 JSON-RPC SDK | 控制面存在，无发布 SDK 包 | **Gap-Learn** |
| `acp/acp` | Agent Client Protocol 服务器 | **Proposal-only**（ACP-1） | **Gap-Learn** |
| `host/*` + `client/*`（大量 ui-*） | Web GUI 主机/浏览器半 | 薄 UI shell + 嵌入/split Vite | **Analog**（功能面差距大，但是壳，不是内核身份） |
| `boot/*` | 启动胶水 | `cmd/vivy` + app 组装 | **Present** |

**读法提醒：** 不要把 `client/ui-*` 三十多个包逐个算成「物种缺 30 个特性」。
它们是 **同一 Web 产品壳的插件化切分**。对 Vivy，只把「缺控制面能力」记入
Gap-Learn（例如 goal RPC、trajectory 事件查询、MCP 管理 RPC）。

### 2.13 POC / support（不进 P0）

| 组 | 处理 |
|---|---|
| `e2b/*` | POC — 记录存在，不进缺口优先级 |
| `examples/*` / `test-support/*` / `util/*` / `runtime-diagnostics/invariants` | 工程支撑；Vivy 用 `just ci` + CN 套件 + Playwright 对应 |
| `extensions/ui-cordis` 等 | 与 tool-cordis 同属自修改家族 → **Gap-Refuse** |

---

## 3. 模型可见工具面对照

### 3.1 DSH（生成目录 `docs/tool-catalog.md`，抽样全量名）

`ask_user_question` · `run_code` · `exit_plan_mode` · `bash` · `pwsh` ·
持久 `bash` · `cordis_*` · `str_replace_editor` · `read`/`write`/`edit`/
`read_image` · `glob`/`grep` · `terminal_*` · `create_goal`/`get_goal`/
`update_goal` · `schedule_*` · `lsp` · `ralph` · `workflow` · `skill` ·
`session_event_*` / `session_search` / `session_trace` · `subagent` ·
`subagent` 控制族 · `job_*` · `todo_write` · `web_search`/`web_fetch` ·
`mcp__<server>__<raw>`

### 3.2 Vivy（`config.example.yaml` `tools.enabled` + 注册表，2026-08-29）

默认启用（约 **23**，另 `echo_info` 注册但默认关闭）：

`write_note` · `list_notes` · `read_note` · `ask_user` · `list_dir` ·
`read_file` · `search_files` · `write_file` · `patch` · `http_request` ·
`mcp_list_tools` · `mcp_call` · `sequential_thinking` · `execute` ·
`commandline` · `skills_list` · `skill_view` · `skill_manage` ·
`task_create` · `task_get` · `task_update` · `task_list` ·
`network_search` · `tool_search`

相对 2026-08-16 GAP 文附录 A 的「24」：当时未强调 `list_dir` 与默认关闭的
`echo_info`；以本表为准。

### 3.3 对照判定（摘）

| 模型面 | 判定 |
|---|---|
| fs 读写搜补丁 | Analog（Present 子集） |
| shell | Analog（Vivy 无 shell 语法、无 PTY） |
| todo | Analog（task_* vs todo_write） |
| plan 退出工具 | **无** exit_plan_mode；模式是 RunMode |
| ask user | Present |
| skills | Present |
| mcp | Present |
| web | Analog |
| goal/schedule/terminal/lsp/run_code/workflow/ralph/job_*/session_query/cordis_* | **Gap**（Learn 或 Refuse） |
| subagent 工具族 | Gap-Learn（有 worker，无同名工具） |
| notes / tool_search / sequential_thinking | Vivy 自有；DSH 无同名 |

---

## 4. 缺口合成

### 4.1 Gap-Learn 候选（物种内一等公民重做）

建议优先级只服务「先写哪份能力提案」，**不是** sprint 承诺。

#### P0 — 过夜与长会话体感

| ID | 锚（DSH） | Vivy 现状 | 备注 |
|---|---|---|---|
| L-COMPACT | `compaction/*` | ADR defer | 与 G1 对齐；需事件模型 + 不可破坏 Journal 不变量 |
| L-QUERY | `session-query/*` | 无；UI-TRAJ demo | 解锁轨迹/检索/HITL 历史；与 G2 对齐 |
| L-TOKEN-PRESSURE | `token-meter` | 硬上限 + 用量统计 | 有数无「压强→动作」闭环 |

#### P1 — 干活与编排

| ID | 锚（DSH） | Vivy 现状 | 备注 |
|---|---|---|---|
| L-GOAL | `goal/*` | UI-GOAL OPEN | 独立对象 + revision + phase；勿与 task_* 混名 |
| L-TERMINAL | `terminal/*` | 无 | 或先增强 execute 长会话/后台，再决定是否 PTY |
| L-JOBS-MODEL | `jobs/tool-jobs` | 后台 run API | 模型面 job_* 是否必要，先写提案边界 |
| L-SUBAGENT-UX | `tool-subagent*` | worker 树 | 委托工具与 UI-TREE |
| L-WORKFLOW | `workflow/*` `ralph` | prd 非目标 | **先决策要不要**；要则 Vivy-native，禁搬 worker-thread TS |

#### P2 — 安全、语言、接口

| ID | 锚（DSH） | Vivy 现状 | 备注 |
|---|---|---|---|
| L-SBX-OS | `sandbox/*` native | SBX-OS DEFERRED | 与 G3 对齐 |
| L-LSP | `lsp/*` | 物种无 | 或保持 Analog-Studio |
| L-RUN-CODE | `code-runtime` | 无 | 安全面接近 tool-cordis，需强沙箱前置 |
| L-ACP | `acp` | ACP-1 提案 | 已有书面；实现另批 |
| L-SDK | `sdk` python/ts | 无对外包 | 控制面稳定后再冻 SDK |
| L-ATTACH | `attachment` | UI stub | 与多模态输入一起 |

#### P3 — 体验抛光

| ID | 锚 | 备注 |
|---|---|---|
| L-TITLE | session-title | 自动标题 |
| L-FEEDBACK | feedback | 消息反馈 |
| L-TELEMETRY | session-telemetry-otel | 默认关的导出 |
| L-SCHEDULE | schedule | 会话提醒 |
| L-SPILL | spill | locator 取代纯截断 |
| L-COMMANDS | interaction/commands | `/` 命令注册表 |
| L-PRESET | agent-presets | 须不破坏 pack 代语义 |

### 4.2 Gap-Refuse（不是欠账）

| DSH 锚 | 拒绝理由 | 决策 |
|---|---|---|
| everything is a plugin（含 loop/log） | 环境不能是种群成员 | NG-7 |
| `extensions/tool-cordis` 自挂载 | 生产无门自改写；非安全边界 | NG-11 |
| 社区插件发现 / marketplace | 精选目录 | PRD §5.0.3 |
| 运行时 `dsh plugin add` 式热装 | 加减能力 = 新 Generation | NG-15 |
| Node 进物种热路径 | 单 EXE / Windows | NG-2 |
| hooks-codex / claude-code 桥 | 外 agent 身份；Vivy 内化 policy | 产品用不上 |
| 多租户托管控制面 | 个人网关 | D-016 等 |

### 4.3 Analog / Vivy-Stronger（避免假缺口）

| 主题 | 说明 |
|---|---|
| Plan Mode | Vivy **更硬**（物理 Deny）；DSH 更软（指导 + 独立 sandbox） |
| Todo | 双方都有；API 形状不同；Vivy 人闸编辑未开放 ≠ 无 todo |
| Approval | Vivy proposal/过期/Review Center 更完整 |
| 终态 / Recover | Vivy Journal 双保险 + 启动结算门 |
| Budget | Vivy 显式父子账本 |
| 装配 | pack 代 vs Cordis 树——等效「组合」，异时点 |
| 文件 sandbox 三档 | 已对齐命名；勿与 OS sandbox 混谈 |
| Skills / MCP / AskUser / FS 子集 | 已 Present 或强 Analog |

---

## 5. 对 Vivy 装配模型的含义

DSH：**一个特性 ≈ 一个可挂插件**（Service Definition / Provider / Consumer）。

Vivy（`VIVY-ASSEMBLY.md`）：

```text
内核（不可卸）     journal / policy / rpc / inspect
出厂一等单元       loop / world / provider / tool / skill
用户层             plugins/<name> + 配方 plugins:
组合时点           vivy-sdk pack → 新 EXE（不是活树）
```

因此 Gap-Learn 落地时的**默认形态建议**：

| 缺口类型 | 建议归属 |
|---|---|
| 不变量、审批、压缩策略、goal 状态机 | **内核一等公民** |
| 终端、lsp、web、fs 增强 | **出厂 tool** 或 world 能力 |
| 可选集成（特定 SaaS、私有协议） | **用户 plugin** + pack |
| 仅开发期需要 | **Studio / 授权工具**，不进物种日常 |
| channel / face | 走已有提案 `VIVY-CHANNEL-PACK` / `VIVY-FACE-PACK`，不在本文扩 scope |

**反模式：** 为对齐 DSH 包名而在 `internal/` 造「伪插件目录」却仍热加载；或把
内核能力塞进 `plugins/` 冒充用户层（ASSEMBLY 已禁）。

---

## 6. 结论

1. **用插件锚点读 DSH，比用口号读更可执行。** plan/todo 证明：同名特性可以
   是 Analog 而非 Gap；真正的空位在 goal、compaction、query、terminal、
   OS sandbox、workflow 决策等。
2. **身份差距仍优先于清单差距。** 热插件与 tool-cordis 再强，也不是 Vivy 的
   作业。Vivy 的作业是把「该有的能力」做成 **可回放、可审、可恢复** 的物种内
   单元。
3. **与既有 GAP 文一致的大图：** 可后补集中在上下文、编排、沙箱；优势集中在
   终态、人闸、恢复、预算、代际。本文把该大图 **展开到包名级**，便于写能力
   提案时直接引用 `packages/...` 与 Vivy 路径。
4. **建议的下一步（仍是调研后动作，不是本文交付）：**
   - **已起草（待批准）：**
     [`docs/capability-proposal-eino-context-goal-query.md`](../capability-proposal-eino-context-goal-query.md)
     —— Track A 优先接线 Eino `reduction`/`summarization`；Track B session goal；
     Track C FTS query；明确不采用 prebuilt/deep 与第二循环
   - L-WORKFLOW / L-RUN-CODE 先做 go/no-go 备忘，默认不进内核
   - L-SBX-OS / L-ACP 已有板项，保持 DEFERRED 直到明确批准
   - 不要为对齐 `client/ui-*` 而扩物种内核

本文 **不** 主张 Vivy 追平 DSH 插件数量；主张 **按插件地图点名** 后，把差距
分成学 / 拒 / 后补，并在后补时保持编译期装配与 Journal 信任根。

---

## 附录 A：包组 → 分类速查（扁平）

| 组 | 主分类（多数包） | 物种行动 implicit |
|---|---|---|
| core | Analog / Present | 维持；prompt-section seam 可选 |
| plan | Analog | 保持物理 Deny；可选增强退出协作弧 |
| todo | Analog | 产品决策 UI-TODO-MUTATE；非从零开发 |
| goal | Gap-Learn | 提案 |
| schedule | Gap-Learn | 后 |
| jobs | Analog→Learn | 厘清与后台 run 边界 |
| interaction | Vivy-Stronger + 少许 Learn | commands / live preset |
| sandbox | 文件 Analog；OS Gap-Learn | SBX-OS |
| fs/shell/subprocess | Analog | 按需增强 |
| terminal | Gap-Learn | 提案 |
| code-runtime | Gap-Learn（慎） | 依赖沙箱 |
| lsp | Gap-Learn / Studio | 明确场景 |
| skill/mcp | Present | UI 管理补齐 |
| compaction/spill/context/token | Learn / Analog | P0 带 |
| subagent | Analog | UX 工具化 |
| workflow/ralph | Learn 或拒 | 先决策 |
| extensions/tool-cordis | Gap-Refuse | 不做 |
| session* / session-query | Analog + Learn | query P0 |
| web/attachment/credentials/settings | Analog / Learn | attachment |
| llm/preset/bundle/hooks | Analog / Refuse | preset 慎 |
| api/sdk/acp/host/client | Analog / Learn | ACP/SDK；UI 不单列内核债 |
| e2b/examples/test-support/util | N/A | 忽略 |

---

## 附录 B：证据索引

### DSH（相对 `.workspace/deepseek-harness/upstream`）

- HEAD `47f9438` · version `0.1.0-rc.5`
- 包地图：`packages/README.md`
- 架构：`docs/architecture.md`
- 子系统索引：`docs/subsystems/README.md`
- 范例：`docs/subsystems/plan.md`、`packages/todo/tool-todo/README.md`、
  `docs/subsystems/goal.md`、`docs/subsystems/sandbox.md`、
  `docs/subsystems/compaction.md`、`docs/subsystems/subagent.md`、
  `docs/subsystems/workflow.md`
- 工具目录：`docs/tool-catalog.md`

### Vivy（相对仓库根）

- 装配/哲学：`docs/architecture/VIVY-ASSEMBLY.md`、
  `SELF-EVOLVING-GATEWAY.md`、`VIVY-GATEWAY-AND-STUDIO.md`
- 能力维对照：`docs/research/DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md`
- Plan：`internal/domain/runmode.go`、`internal/runtime/runmode.go`、
  `internal/runtime/tooladapter.go`（`ErrPlanModeToolDenied`）、
  `internal/runtime/policy.go`
- Todo：`internal/tools/todo.go`；UI-TODO-MUTATE / UI-GOAL：`docs/TODO.md` §0.1
- 工具启用表：`config.example.yaml` `tools.enabled` / `runtime.sandbox`
- Worker：`internal/worker/*`
- 压缩 defer：`docs/AGENT-VIVY-ARCHITECTURE-V0.md` ADR-009/010 一带
- 板项：`docs/TODO.md`（SBX-OS、ACP-1、UI-GOAL、UI-TRAJ、UI-MCP…）

---

## 附录 C：与 G1–G20 对照

| G#（GAP 文） | 本文锚 | 分类落点 |
|---|---|---|
| G1 compaction | L-COMPACT · `compaction/*` | Gap-Learn P0 |
| G2 FTS | L-QUERY · `session-query/*` | Gap-Learn P0 |
| G3 OS sandbox | L-SBX-OS · `sandbox/*` | Gap-Learn P2（已挂板） |
| G4 PTY | L-TERMINAL · `terminal/*` | Gap-Learn P1 |
| G5 LSP | L-LSP | Gap-Learn / Studio |
| G6 run_code | L-RUN-CODE · `code-runtime` | Gap-Learn 慎 |
| G7 子代理多形态 | subagent 组 Analog + L-SUBAGENT-UX | Analog/Learn |
| G8 workflow/ralph | L-WORKFLOW | Learn 或拒 |
| G9 goal | L-GOAL · `goal/*` | Gap-Learn P1 |
| G10 schedule | L-SCHEDULE | Gap-Learn P3 |
| G11 ACP | L-ACP | Gap-Learn（提案） |
| G12 SDK | L-SDK | Gap-Learn |
| G13 hooks 桥 | hooks 组 | Gap-Refuse |
| G14 OTel | L-TELEMETRY | Gap-Learn P3 |
| G15 feedback | L-FEEDBACK | Gap-Learn P3 |
| G16 自动标题 | L-TITLE | Gap-Learn P3 |
| G17 跨会话引用 | `context/session-reference` | Gap-Learn |
| G18 provider 广度 | llm 组 | Gap-Learn（提案制） |
| G19 多持久化后端 | session-persistence | 有意保持 SQLite 为主 |
| G20 全请求重放 | session 日志深度 | Analog/Learn |

**G 未覆盖而本文强调的 Analog：** plan 软 vs 硬、todo 形状、文件 sandbox 已对齐、
budget/Review Center/终态等 Vivy-Stronger 项——避免把优势误写成缺口。

---

## 附录 D：修订

| 版本 | 日期 | 说明 |
|---|---|---|
| v1 | 2026-08-29 | 首版：以 DSH packages 组为锚的 Vivy 缺口调研；互补于能力维 GAP 文 |
| v1.1 | 2026-08-29 | §6 链到 Eino-first 能力提案（compaction/goal/query） |
