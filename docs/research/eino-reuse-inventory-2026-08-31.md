# Eino/上游复用清单：VC track 哪些不用自研（2026-08-31）

- 日期：2026-08-31
- 范围口径（用户拍板）：**功能面对齐 Crush；Crush 没有的功能不擅自添加。** 本清单只覆盖 Crush 对标面（`docs/research/crush-parity-code-agent-research-2026-08-31.md` §3/§5 的 VC-0..VC-4 + 既有归并项），eino/上游超出该面的能力一律只做内部实现件或明确"不暴露"（见 §3 护栏）。
- 核查基线：eino v0.9.13（go.mod 锁定，module cache 实读）；eino-ext 组件经 `go list -m @latest` 验证存在；Crush 上游依赖读其 go.mod（`.workspace/crush/`，FSL-1.1-MIT 仅参照，代码不拷）。
- 结论一句话：**VC track 约六成的"新工具面/回路"工作可落在现成组件上——eino 原生中间件 + Crush 同款上游库（MIT/BSD，可直接依赖）；真正必须自研的是治理挂钩（存档/审批/审计）和 Vivy 语义件（面具/预算/Journal），而这些正是差异化所在。**

---

## 1. 三档复用判定总表

判定档位：**A 已在用**（零工作）/ **B 直接复用**（接上即可，评估点已列）/ **C 薄自研**（Crush 同款也是自写，无上游可拿，但多数是小件）/ **D 治理自研**（Vivy 语义，本就不该外包）。

### 1.1 eino 原生（v0.9.13，无需新增依赖）

| 能力（Crush 对位） | 档 | 说明与评估点 |
|---|---|---|
| checkpoint/中断恢复（Crush resume） | A | `adk.CheckPointStore`；Vivy VersionedCheckpointStore 已消费。纯运行态，与文件回退（RB-1 L1/L2）无关 |
| AGENTS.md 注入（D6 上下文文件） | B | `middlewares/agentsmd`：@import 递归 5 层、总量字节上限、model-call 瞬态注入（不进会话状态/不进摘要）。**D6 免费直通**；`vivy init` 生成 AGENTS.md 仍自建 |
| 文件/搜索工具注册面（ls/read_file/write_file/edit_file/glob/grep/execute 七工具+中英描述） | B | `middlewares/filesystem` 原生注册；Vivy 的 `EinoFilesystemBackend` 已实现 `einofs.Backend`，接注册层即得 grep/glob 工具。评估点：工具命名对表（eino `edit_file` vs Vivy `patch`——按 Crush 面 `edit`/`multiedit` 定名，eino 工具名可经 `selectToolName` 定制）；策略标注/审批走 Vivy 既有管线 |
| read 支持图片/PDF（VC-3 read_file 图片） | B | `filesystem.MultiModalReader` 协议位现成，backend 实现即可 |
| execute 后台标志位 | B(部分) | `Shell`/`StreamingShell` + `RunInBackendGround` 协议位原生；**job 取回/终止管理原生无**（见 C） |
| 悬空 tool calls 修补 | B(评估) | `middlewares/patchtoolcalls`——resume/压缩边界卫生；内部件，非产品面。评估后可替代自研修补 |
| 压缩/技能/工具检索 | A | `summarization`/`reduction`/`skill`/`dynamictool(toolsearch)` Vivy 已用或已有等价 |
| gitignore 感知 glob（`**`） | B | eino 自身用 `bmatcuk/doublestar/v4`（见 1.2）；Vivy `matchesGlob` 现为 filepath.Match 手写，无 `**` 递归——换 doublestar 即补齐 |

### 1.2 Crush 同款上游库（MIT/BSD，可直接依赖——"移植"即引库，符合 FSL 行为对齐要求）

| Crush 用途 | 上游库（license） | Vivy 落点 |
|---|---|---|
| 嵌入式 POSIX shell（bash 工具） | `mvdan.cc/sh/v3`（BSD-3）+ `mvdan.cc/sh/moreinterp`（扩展命令解释器，Crush 同引） | VC-1 bash 工具核心。Windows 无 WSL 可跑 POSIX 语法；与沙箱路径约束的兼容是 VC-1 验收点 |
| unified diff 生成 | `aymanbagabas/go-udiff`（MIT） | VC-1 后端 diff（write/patch/execute 提案与结果出 unified diff + 增删统计）；替换 Vivy 手写 `boundedDiff` 单 hunk 伪 diff |
| `**` 递归 glob | `bmatcuk/doublestar/v4`（MIT） | glob 工具 + search_files glob 过滤升级 |
| LSP 客户端 | `charmbracelet/x/powernap`（MIT） | VC-3 LSP manager（懒启动/自动发现/诊断）。fallback：自写最小 jsonrpc2 客户端（1-2k 行，既有拍板保留） |
| diff 小工具 | `pmezard/go-difflib`（BSD） | 增删行统计等杂项（go-udiff 不够处） |
| ripgrep 优先 | 外部 `rg` 二进制探测（Crush 同策略） | grep 工具 rg-first：有 rg 用 rg（原生 gitignore 感知），无则纯 Go 回退（backend `GrepRaw` 已有 regex 走查） |
| MCP 客户端 | `eino-ext/components/tool/mcp` v0.0.9（包裹 mark3labs/mcp-go，Apache-2.0；已验证存在） | VC-4 MCP stdio 传输：评估以组件替代 Vivy 手写 Streamable HTTP 客户端的增量（现有 HTTP 客户端已带会话/Bearer/热更，取舍看 stdio+OAuth 成本）；D-007 检疫不受影响（组件属 eino 家族，仍在 runtime 边界内引） |
| Anthropic 原生 | `eino-ext/components/model/claude` v0.1.25（已验证存在） | VC-2，§8.5 既定拍板，6 项落地清单 |

### 1.3 UI 侧（浏览器组件，不自研渲染）

| Crush 用途 | 上游 | Vivy 落点 |
|---|---|---|
| diff 视图 unified/split | react-diff-view / diff2html / primevue diff 等（实现时选型，MIT 系） | VC-1 UI diff 渲染（D10 呈现行为对齐 Crush，组件用现成） |
| 文件预览语法高亮 | shiki / prism（实现时选型） | VC-3 UI 文件预览 |

## 2. C/D 档：必须自研（Crush 亦自写，或 Vivy 语义专有）

| 件 | 档 | 说明 |
|---|---|---|
| multiedit | C | eino 只有单 edit；Crush 自写。小件 |
| patch 空白容错回退 | C | Crush `normalizedReplace` 同款逻辑自写（行为对齐） |
| stale-read filetracker + file_versions 存储 + 恢复 RPC | D | RB-1 L1/L2：治理核心（存储迁移+审批+Journal），Crush 版本链亦无恢复消费方。2026-09-01 拍板：记录侧（file_versions + filetracker）随 VC-3 尾款落地；恢复 RPC 暂缓（RB-L2-DEFER，MVP 后再议） |
| 后台 job 注册表（job_output/job_kill、超时转后台） | C | eino 只有协议位；Crush 自写。挂在 bash 工具实现内 |
| 死循环检测（签名去重） | C | Crush 自写；Vivy 挂 MaxToolTurns 旁 |
| 成本核算元数据表（context window/价格） | C | D9 拍板与 web provider/model 管理同步；Crush 用远端 Catwalk 我们明确不引 |
| headless `vivy run`（FACE-0） | C | Crush 自写 CLI 面；Vivy 落 face 装配 |
| hooks 引擎（PreToolUse 协议） | C | Crush 自写；Vivy 挂既有 `ToolHookChain` |
| 401 重认证重试 | C | Crush 自写三分支；Vivy 对齐 chatmodel retry（eino 有 retry_chatmodel/failover 原生——评估点：`adk` retry/failover ChatModel 可能覆盖大半） |
| 会话自动标题 | C | 小件 |
| 消息排队 + 两段式取消 | C | UI 交互件 |
| 图片附件链路 | C | UI+provider workaround |
| LSP 诊断回填 | C | powernap 拿到诊断后，回填 write/patch 结果的粘合层是 Vivy 治理面 |
| gitignore 纯 Go 回退走查 | C | rg 缺席时的 fallback；可评估 `sabhiram/go-gitignore`（MIT）减量 |
| 沙箱/审批/预算/Journal/面具 | D | Vivy 专有差异化，本就不外包 |

## 3. 护栏：eino/上游超出 Crush 面的能力——处置表（不擅自添加）

| eino/上游能力 | Crush 有无 | 处置 |
|---|---|---|
| `adk/prebuilt/deep`（DeepAgents）、`planexecute` | 无 | **不采用不暴露**。理由（2026-08-31 用户质询后记录）：① `planexecute` = plan–execute–replan 回路，Crush 面没有，按"不擅自添加"出局；② `deep` 是自带 task_tool/计划文件的完整 DeepAgents 栈，超出 Crush 的 task 语义且与 Vivy 治理（审批/预算/Journal/面具）平行，接入即双轨；③ 三者的"协调"都发生在 eino 图内，而 Vivy 子代理治理（WorkerChildAuthority/预算/审批并入父会话 D5/PolicySnapshot 面具）全在 runtime 服务层——用 prebuilt 得先拆它再接回管线，比薄封装既有 child run RPC 更费工。Crush 面真正要的只是 `agent` 工具=薄包装。**重启条件**：将来立项"规划-执行"类能力提案时，prebuilt 可作为内核候选再评估（走提案流程） |
| `adk/prebuilt/supervisor` | 无（Crush 硬编码 coder/task 亦被我们否决） | 不采用；该 prebuilt 本身就是"中央 agent 协调一群子 agent"（supervisor.go 包注释原文）——恰是 2026-08-31 拍板否决的 Crush coordinator 模式；vivy = 单主人格戴面具，supervisor 语义由产品人格承担而非编排组件。子代理 = Vivy child run 自有语义的薄包装 |
| `middlewares/plantask`（task_*） | 有等价（todos） | 不切换不双暴露；Vivy task_* 维持 |
| `middlewares/dynamictool/toolsearch` | 无 | Vivy 已有 tool_search（既有能力维持现状，不借 eino 扩面） |
| `middlewares/filesystem` large_tool_result | 内部件 | 内部卫生，评估采用，不构成产品面 |
| 未来 eino 新中间件/组件 | — | 一律先过"Crush 面"过滤：产品可见即违护栏，内部实现件可评估 |

## 4. 对 VC 工作量的影响（对研究 §5 的修正）

- **VC-1 减量最显著**：grep/glob/execute 工具定义层、unified diff 后端、`**` glob、POSIX shell 全部改"引库/接 middleware"；自研集中到 multiedit、空白容错、filetracker/版本链、job 注册表、审批标注粘合。粗估 2-3 周 → **1.5-2 周**。
- **VC-2**：Anthropic 引 claude 组件（既定）；hooks/headless/成本仍自研；retry/failover 评估 eino 原生减量。粗估 2 周 → **1.5 周**。
- **VC-3**：LSP 库不自研（powernap），自研集中诊断回填粘合与版本链读侧。粗估 3-4 周 → **2.5-3 周**。
- RB-1（回退）结论不受影响：eino 无文件版本原语，L1/L2 照 §5 落 Vivy 侧。
