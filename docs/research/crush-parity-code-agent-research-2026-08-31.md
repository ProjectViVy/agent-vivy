# Crush 对标研究：Vivy 走 CODE AGENT 路线的差距与路线图

- 日期：2026-08-31
- 研究对象：`.workspace/crush/`（Charm Crush，Go 终端 AI 编程助手，FSL-1.1-MIT）
- 对照基线：本仓库 `internal/`（vivy.exe 运行时）+ `ui/`（Web UI），截至 2026-08-30 main
- 结论一句话：**可行。Vivy 的治理基座（HITL 审批 / checkpoint / Journal 审计 / 常驻 server）在多个维度领先 Crush，真正缺的是"编码工具面 + LSP + diff 呈现 + headless 面"四块；按 VC-0…VC-4 五个阶段补齐即可达到功能对齐，且不必放弃 Vivy 的差异化定位。**

---

## 1. Crush 是什么

Crush 是 Charm 出品的终端编码 agent（TUI-first，后加 server 模式），核心构成：

- **Agent 回路**：`charm.land/fantasy` 统一 LLM 抽象 + `fantasy.NewAgent` 循环；`StopWhen` 双条件（token 水位触发自动摘要；10 步窗口内重复"调用+结果"签名 >5 次判死循环）；每步流式写库；排队/取消竞态处理（AcceptedRun/RunID）；401 自动重认证重试。
- **27+ 内置工具**：bash / edit / multiedit / write / view / glob / grep / ls / fetch / download / web_fetch / web_search / agentic_fetch / sourcegraph / todos / question / job_output / job_kill / crush_info / crush_logs / 8 个 lsp_* 工具 / 2 个 MCP 资源工具 / agent(task 子代理) / MCP 包装工具。工具描述全部是可 `go:embed` 的 `.md` / `.md.tpl` 模板文件。
- **LSP 集成**：懒启动管理器 + 自动发现（powernap），编辑后把 LSP 诊断回填进工具结果（模型直接看到 lint 错误），8 个 LSP 工具。
- **MCP**：stdio / http / sse 三传输 + OAuth 2.1（含动态客户端注册），工具按 `mcp_{server}_{tool}` 合并进工具面。
- **权限**：allowlist（`tool` 或 `tool:action` 粒度）+ per-session 持久授权 + yolo；bash 有 60+ 只读命令白名单（免提示）与 ~70 条阻断命令（curl/sudo/…）。
- **Hooks**：目前仅 `PreToolUse` 一个事件；协议 = stdin JSON payload + env 注入 + exit code（2 阻断工具 / 49 halt 整轮）+ stdout JSON 信封（`decision: allow|deny`、`reason`、`context`、`updated_input` 浅合并）；并行执行、按配置顺序聚合、在权限检查**之前**运行、兼容 Claude Code 输出格式。
- **会话**：SQLite（modernc/ncruces 双驱动）+ goose 迁移 + sqlc；父子子会话（子代理成本汇总回父会话）；文件版本 history 表；filetracker 记录"每会话读过哪些文件及时间"用于 stale-read 防护。
- **运行形态**：TUI（Bubble Tea v2）/ headless `crush run` / 常驻 server（Unix socket / Windows 命名管道 + SSE 事件流），多客户端共享同一 workspace（按 `--cwd` 分组）。
- **Skills**：Agent Skills 开放标准（agentskills.io），多路径发现（`~/.claude/skills`、`.agents/skills`、`.cursor/skills`…），3 个 builtin skills（crush-config / crush-hooks / jq），frontmatter 支持 `user-invocable` / `disable-model-invocation`。
- **上下文文件**：工作区 AGENTS.md / CLAUDE.md / CRUSH.md / .cursorrules 等自动注入系统提示词；全局 `~/.config/crush/CRUSH.md`；`.crushignore` 叠加 `.gitignore`。
- **Provider**：fantasy 多协议（anthropic / openai / openai-compat / bedrock / vertex / ollama / …）+ Catwalk 远端模型目录自动更新（context window、价格、reasoning 元数据）。
- **配置**：`crushrc`（嵌入式 bash 解释器跑配置 DSL，跨平台一致）为主，`crush.json` 遗留兼容。

## 2. Vivy 现状基线（与编码 agent 相关）

**已具备、且多数强于 Crush 的**：

| 能力 | Vivy 实现 | Crush 对照 |
|---|---|---|
| 常驻 server + 事件流 | 天生就是常驻网关（HTTP :8787 + WebSocket JSON-RPC `run/subscribe`），Web/TUI/headless 构建共用 | Crush 后加的 serve 模式（SSE 单向），Vivy 架构更原生 |
| HITL 审批 | 三层（沙箱 `read_only/workspace_write/danger_full_access` × 审批策略 `ask/never/auto` × 会话开关）+ 审批持久化 + checkpoint 恢复 + 超时自动拒 | Crush 只有 allowlist + yolo，无审批持久化恢复 |
| 审计 | Journal 事件溯源（~40 种 RunEvent 先落库再推送） | 无 |
| 预算 | BudgetLedger 断路器，父子共享收紧不放宽 | 无 |
| 上下文压缩 | 运行内 reduction + summarization + 会话级 CompactSession + UI meter | token 水位触发摘要（较简单） |
| 会话存储 | SQLite（16 migration）/ Postgres 双后端 | SQLite 单后端 |
| Resume | checkpoint 桥 + 重启恢复（FR-8） | 会话消息重放 |
| 技能 | 运行时 skills_root + Eino skill middleware + 技能市场（skills.sh）+ prompt-injection 扫描 | 标准发现 + builtin 3 个，无市场 |
| 插件 | vivy-sdk pack 编译式插件 ABI（tool_world / channel seam） | 无 |
| 提问 | `ask_user` 独立 question 流 + UI 应答 | `question` 工具（等价） |
| Todo | `task_*` 五件套（持久 + 依赖） | `todos` 全量替换 |
| MCP 客户端 | Streamable HTTP（JSON-RPC + SSE 解码、会话、Bearer、大小上限、settings 热更） | 三传输 + OAuth，无热更 |
| 文件工具 | `read_file`（行号化）/ `write_file` / `patch`（精确替换）/ `search_files`（字面）/ `list_dir` | 更全（见 §3 矩阵） |
| 受控执行 | `execute`（basename 白名单、无 shell 复合、恒审批）+ 沙箱三档 | `bash`（嵌入式 POSIX shell + 后台 job） |
| 网络 | `http_request`（host 白名单 + 私网拒）+ `network_search`（5 引擎降级） | `fetch` / `web_search` |

**关键缺口**（Crush 有、Vivy 无）：

1. 无正则 grep、无独立 glob、无 multiedit；`patch` 无空白容错；无 stale-read 防护。
2. `execute` 不能跑复合命令（管道/`&&`/`git diff`），无后台 job —— 对编码 agent 是硬伤。
3. 无 LSP。
4. UI/TUI 均无 diff 渲染、无文件预览、审批无内联 diff 高亮。
5. 无命名 subagent / `agent` 工具（底层 child run 已有但未暴露为模型工具）。
6. 无工作区上下文文件注入（AGENTS.md/CLAUDE.md）。
7. hooks 有接口（`ToolHookChain`）无任何实现，不可配置。
8. 无 headless 一次性运行面（`vivy run "prompt"`）——FACE-0 在 TODO。
9. Anthropic-native provider 未接线（catalog 显式报 "not wired yet"）。
10. 无货币成本核算（有 token 统计）；模型元数据（context window）是硬编码小表。
11. MCP 仅 HTTP 传输；无 stdio、无 OAuth。

## 3. 全景对标矩阵

判定：✅ 已对齐 / 🟡 部分对齐 / ❌ 缺失 / ➖ 不建议照抄。

### A. 工具面

| Crush 工具 | Vivy 现状 | 判定 | 归属 |
|---|---|---|---|
| `bash`（复合命令+后台 job+只读白名单+超时转后台） | `execute`（basename 白名单、无 shell、恒审批） | ❌ 差距最大 | VC-1 |
| `edit`（old/new 精确替换 + 空白容错回退 + stale-read 防护 + diff 入 history） | `patch`（精确替换，无容错、无防护） | 🟡 | VC-1 |
| `multiedit` | 无 | ❌ | VC-1 |
| `write`（stale-read 防护） | `write_file` | 🟡 补防护 | VC-1 |
| `view`（行号/图片/skills 虚拟路径） | `read_file`（行号） | 🟡（图片读取低优） | VC-3 |
| `grep`（rg 优先纯 Go 回退，尊重 .gitignore/.crushignore） | `search_files`（仅字面） | ❌ | VC-1 |
| `glob` | 仅 `search_files` 的 glob 过滤参数 | ❌ | VC-1 |
| `ls` | `list_dir` | ✅ | — |
| `todos` | `task_*` 五件套（更强） | ✅ | — |
| `question` | `ask_user` | ✅ | — |
| `job_output` / `job_kill` | 无 | ❌ | VC-1 |
| `fetch` / `web_search` | `http_request` / `network_search` | ✅（更严） | — |
| `agentic_fetch` / `sourcegraph` | 无 | ➖ 可选 | VC-4 |
| `lsp_*` ×8 + 编辑后诊断回填 | 无 | ❌ 编码质量核心 | VC-3 |
| MCP 资源 ×2 / `mcp_{server}_{tool}` 直通工具 | `mcp_list_tools` / `mcp_call`（审批+提案，间接调用） | 🟡 | VC-4 |
| `agent`（task 子代理，child session + cost 汇总） | 底层 child run 有，无模型可见工具 | ❌ | VC-2 |
| `crush_info` / `crush_logs` | `tool_search` 部分；无自诊断工具 | 🟡 低优 | VC-4 |

### B. Agent 架构与回路

| Crush | Vivy 现状 | 判定 | 归属 |
|---|---|---|---|
| 多 agent coordinator（coder/task） | 单 agent（ADK ChatModelAgent，Name 恒 "vivy"） | ❌ | VC-2 |
| 系统 prompt 模板 + GitStatus + ContextFiles 注入 | 静态 Instruction + 每 run preamble | 🟡 | VC-1 |
| 上下文文件（AGENTS.md/CLAUDE.md/全局） | 无 | ❌ | VC-1 |
| 死循环检测（签名去重） | 仅 `MaxToolTurns` | ❌ | VC-2 |
| token 水位自动摘要 | compaction（reduction+summarization，更强） | ✅ | — |
| 401 重认证重试 | 无 | 🟡 低优 | VC-4 |
| 成本核算（价格元数据） | token 统计无货币化 | ❌ | VC-2 |
| filetracker + stale-read 防护 | 无 | ❌ | VC-1 |

### C. 治理（Vivy 领先区，不照抄）

| Crush | Vivy | 判定 |
|---|---|---|
| allowlist + yolo + per-session 授权 | 三层沙箱/策略/会话开关 + 审批持久化 + checkpoint 恢复 + 超时调度 | ✅ 领先 |
| Hooks：仅 PreToolUse，shell 脚本可配置 | `ToolHook` 接口 + 空链，不可配置 | ❌ 补可配置脚本 hook（VC-2），协议对齐 Claude Code |
| bash 只读命令白名单（safe.go 60+ 条）/ 阻断命令表 | 无 | ❌ 随 VC-1 bash 化引入（正好接进 Vivy 审批策略：白名单=auto、其余=ask） |

### D. 会话/持久化

| Crush | Vivy | 判定 |
|---|---|---|
| SQLite + sqlc + goose | SQLite/Postgres + Journal 事件溯源 | ✅ 领先 |
| 子会话 + cost 汇总回父 | child run 有父子预算，无 cost 汇总 | 🟡（VC-2） |
| 文件版本 history（可查看/恢复旧版本） | 无 | 🟡 低优 | VC-3 |
| `--continue` / `--session` resume | checkpoint + `session/*` RPC | ✅ |

### E. 运行形态

| Crush | Vivy | 判定 | 归属 |
|---|---|---|---|
| TUI（Bubble Tea v2 + glamour 渲染 + 主题） | `vivy tui`（--live 全屏 / --plain REPL，骨架级） | 🟡 | FACE-TUI-1/2（已有 TODO） |
| headless `crush run`（stdin 管道、--continue、退出码语义） | 无 | ❌ | VC-1（FACE-0 已有 TODO） |
| server 模式 + 多客户端 workspace | 天生常驻网关 + WS 双向 | ✅ 领先 | — |
| diff 渲染（工具 metadata + TUI） | UI/TUI 均无 diff 组件 | ❌ | VC-1（UI）/ VC-3（TUI 审批高亮=FACE-TUI-2） |
| 桌面通知 / clipboard / 自更新 / 遥测 | 无 | ➖ 可选 | VC-4（遥测不照抄：Vivy 本地 Journal 即审计） |

### F. Provider

| Crush | Vivy | 判定 | 归属 |
|---|---|---|---|
| fantasy 多协议（anthropic 原生等） | openai 兼容 ✅ / anthropic native ❌（未接线） | ❌ | VC-2（注意 P3-1 license 前置） |
| Catwalk 模型目录（价格/窗口/reasoning 元数据） | 硬编码 context window 小表 | 🟡 取其元数据结构，不抄远端目录 | VC-2 |
| 本地模型发现（ollama/lmstudio/…） | 无 | ➖ 可选 | VC-4 |

### G. Skills / 配置

| Crush | Vivy | 判定 |
|---|---|---|
| Agent Skills 标准 + 多路径发现 + builtin skills | skills_root + Eino middleware + 市场 + 注入扫描 | ✅（可补 builtin 编码 skills：git/vivy-code 用法） |
| crushrc bash 配置 DSL | settings RPC + UI 热更 | ➖ 不照抄（Web 产品形态下 settings 更合适） |
| `.gitignore` + `.crushignore` 层级 ignore | `list_dir`/`search_files` 自带排除表（.git/node_modules） | 🟡 VC-1 统一为 gitignore 感知 + `.vivyignore` |

## 4. Vivy 不该照抄的部分（差异化定位）

1. **治理三件套是护城河不是包袱**：审批持久化 + checkpoint 恢复 + Journal 审计 + 预算断路器，Crush 全都没有。编码 agent 化不弱化它们，反而让 bash 工具的"只读白名单=auto、复合命令=ask"分级策略成为比 Crush 更细的权限模型。
2. **常驻 server 架构**：Crush 的 serve 模式是后加的；Vivy 天生 Web-first，编码场景直接复用 RunInspector/Review Center，不必做 TUI-first。
3. **配置形态**：crushrc（bash DSL）解决的是终端用户的跨平台配置问题；Vivy 有 settings RPC + UI 热更，照抄反而倒退。
4. **遥测**：PostHog 不引入；Journal 就是本地审计。

## 5. 路线图（VIVY-CODE track，建议开 VC-* 编号）

> 现成 TODO 归并：FACE-0（headless 面）、FACE-TUI-1/2（TUI/审批 diff 高亮）、SBX-OS/SBX-GLOB（沙箱升级）、HITL-P1-*（提案编辑/scoped allow）、CMP-1/2/3（压缩深化）、TEST-1（execute mock 场景）——这些不是新工作，是本路线的既有组成部分。

### VC-0 决策与骨架（约 0.5 周）

- 拍板产品形态：**vivy.exe 内的 "code" face**（复用同一 runtime / Journal / 审批 / 技能），而非独立二进制。FACE-0 的 `face` 装配是入口；UI 端 Masks 从纯徽章升级为真实 run 模式（工具面 + prompt 注入随 face 切换）。
- 开 `docs/TODO.md` §0.1 VIVY-CODE track，挂 VC-1…VC-4。
- 决策点：bash 工具与治理的边界（见 §6 风险 1）。

### VC-1 工具面对齐（核心，约 2–3 周）——最小可用编码 agent

1. `bash` 工具：嵌入式 POSIX shell（评估 `mvdan.cc/sh/v3`，Windows 无 WSL 可用）+ 只读命令白名单（移植 Crush `safe.go` 思路，白名单→auto_approve，其余→ask）+ 阻断命令表 + 输出头尾截断。
2. 后台 job：`job_output` / `job_kill`，超时自动转后台。
3. `grep`（rg 优先 / 纯 Go 回退）+ `glob` 工具，gitignore 感知 + `.vivyignore`。
4. `multiedit`；`patch` 空白容错回退；`write_file`/`patch` 接 stale-read 防护（新增 filetracker 存储，read 时记录、写前校验）。
5. 上下文文件注入：工作区 AGENTS.md / CLAUDE.md / VIVY.md → run preamble；`vivy init` 生成 AGENTS.md。
6. UI diff 渲染：write/patch/execute 提案与结果生成 unified diff（Crush 用 go-udiff + additions/removals 统计），MessageBubble 与 Review Center 渲染 diff 视图；顺带关闭 FACE-TUI-2 的审批 diff 高亮。
7. 验收：离线 mock provider e2e 走通 "读码→grep→multiedit→bash 跑测试→看 diff"（补 TEST-1 的 execute 场景）。

### VC-2 回路与 agent 化（约 2 周）

1. `agent`（task 子代理）工具：模型可见，映射到既有 child run RPC；child session + 成本汇总回父 run；子代理默认 NonInteractive + 不触发 hooks（对齐 Crush 语义）。
2. 死循环检测：最近 N 步"调用+结果"签名去重（Crush：10 步窗口 >5 次）。
3. 成本核算：模型元数据表（context window / 输入输出价格 / reasoning 支持）→ token 统计货币化；UI token 面板加成本列。
4. headless 面：`vivy run "prompt"`（stdin 管道、`--continue`、退出码语义）——落 FACE-0。
5. Hooks 用户可配置：`runtime.hooks.pre_tool_use[]`（matcher/command/timeout），协议对齐 Crush/Claude Code（stdin JSON + exit 2 阻断 + stdout 信封 + `updated_input` 浅合并），跑在既有 `ToolHookChain` 上，决策仍入 Journal。
6. Anthropic-native provider 接线（前置：P3-1 license 检查）。

### VC-3 LSP 编码智能（最大单项，约 3–4 周）

1. LSP manager：懒启动 + 按文件类型/root marker 匹配 + 自动发现（gopls、typescript-language-server、pyright 等）+ 不可用缓存。
2. 编辑后诊断回填：write/patch/multiedit 结果附加 LSP 诊断文本（模型直接看到 lint/type 错误——Crush 编码质量的关键机制）。
3. `lsp_*` 工具族分批落地：diagnostics → definition/references/symbols → rename/replace_symbol（后者走 write 审批）。
4. 文件版本 history（编辑前存档，可查看/恢复）。
5. UI 文件预览 + 语法高亮；`read_file` 支持图片。
6. 注意：LSP 代码放 `internal/lsp` 或 `internal/tools`，不 import Eino 即不触检疫；实现可借鉴 Crush 对 powernap 的封装方式，license 合规前提下评估直接用 `charmbracelet/x/powernap`。

### VC-4 生态与产品化（持续）

- MCP：stdio 传输 + OAuth 2.1 + 资源（list/read_mcp_resource）+ `mcp_{server}_{tool}` 直通工具（沿用 Vivy 审批标注）。
- 沙箱升级（SBX-OS/SBX-GLOB 既有项）+ bash deny glob 可编辑 auto_approve。
- builtin 编码 skills（git 工作流 / vivy-code 用法 / just-ci）。
- 本地模型发现（ollama 等）；401 重认证重试；桌面通知；`crush_info` 式自诊断工具。
- 明确不做：crushrc DSL、Catwalk 远端目录、PostHog、TUI 主题系统照搬。

## 6. 风险与决策点

1. **bash 工具 vs 治理哲学（最大张力）**：Vivy 现状 execute 是 basename 白名单 + 恒审批；引入真 bash 等于放开 shell 复合命令。建议分级：只读白名单 auto、工作区写类 ask、阻断表（sudo/curl|sh 等）直接 deny 并入策略引擎规则（policy.go 已支持按工具/字段规则）。沙箱目录约束继续兜底。
2. **嵌入式 shell 选型**：`mvdan.cc/sh/v3`（BSD-3，Crush 同款）是 Windows 无 WSL 跑 POSIX 语法的成熟解；需验证与 Vivy 沙箱路径约束的兼容。
3. **LSP 库 license**：powernap 为 MIT（Charm），可用；若自写最小客户端（jsonrpc2 + 少量 method）约 1–2k 行，作为 fallback。
4. **Anthropic SDK**：官方 SDK license 与 P3-1 前置；也可先用 openai-compat 网关顶住。
5. **UI diff 工作量**：建议引入成熟 diff 视图组件而非手写；后端 diff 生成（unified diff + 增删统计）是小活。
6. **Crush 是 FSL-1.1-MIT**：源码可参考学习，但**代码不能直接拷入** Vivy（FSL 非 OSI 开源，2 年后才转 MIT）。路线中所有"移植"均指行为/协议对齐，实现自写。这一点必须写进每个 VC 任务的验收注释。

## 7. 可行性结论

- **功能对齐可行**：差距集中在工具面与呈现层，无架构性障碍；Vivy 的 ADK 回路、压缩、审批、存储都比 Crush 对应件更完备或等价。
- **顺序建议**：VC-1（工具面）→ VC-2（回路）是"能不能干编码活"的门槛；VC-3（LSP）是"干得好不好"的分水岭；VC-4 按需。
- **总工作量**：VC-0…VC-2 约 5–6 周可达"Crush 核心体验对齐"（bash/grep/glob/edit 族 + diff 呈现 + subagent + headless + hooks + 成本）；VC-3 再 3–4 周达"LSP 增强编码"对齐。
- **Vivy 的终局定位**不是"另一个 Crush"，而是**带强治理的编码 agent**：同样能干活，但每一次写文件、每一条 shell、每一分钱都有审批、审计与预算——这是 Crush 没有的东西，也是 Vivy 已有的东西。
