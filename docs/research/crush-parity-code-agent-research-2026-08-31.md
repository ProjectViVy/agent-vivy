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
| fantasy 多协议（anthropic 原生等） | openai 兼容 ✅ / anthropic native ❌（未接线） | ❌ | VC-2（改用 eino-ext/claude，见 §8.5 决策） |
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
6. Anthropic-native provider 接线：**改用 eino-ext `components/model/claude` 组件**，放弃自研 `vivy/anthropic` 适配器里程碑（决策与落地清单见 §8.5）。

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
4. **Anthropic 接线**：~~license 风险~~ 已核实干净（anthropic-sdk-go MIT、aws-sdk-go-v2 Apache-2.0；P3-1 针对的 claude-code upstream 与适配器无关）。路线已拍板：改用 eino-ext `components/model/claude` 组件，不再自研适配器（见 §8.5）。
5. **UI diff 工作量**：建议引入成熟 diff 视图组件而非手写；后端 diff 生成（unified diff + 增删统计）是小活。
6. **Crush 是 FSL-1.1-MIT**：源码可参考学习，但**代码不能直接拷入** Vivy（FSL 非 OSI 开源，2 年后才转 MIT）。路线中所有"移植"均指行为/协议对齐，实现自写。这一点必须写进每个 VC 任务的验收注释。

## 7. 可行性结论

- **功能对齐可行**：差距集中在工具面与呈现层，无架构性障碍；Vivy 的 ADK 回路、压缩、审批、存储都比 Crush 对应件更完备或等价。
- **顺序建议**：VC-1（工具面）→ VC-2（回路）是"能不能干编码活"的门槛；VC-3（LSP）是"干得好不好"的分水岭；VC-4 按需。
- **总工作量**：VC-0…VC-2 约 5–6 周可达"Crush 核心体验对齐"（bash/grep/glob/edit 族 + diff 呈现 + subagent + headless + hooks + 成本）；VC-3 再 3–4 周达"LSP 增强编码"对齐。
- **Vivy 的终局定位**不是"另一个 Crush"，而是**带强治理的编码 agent**：同样能干活，但每一次写文件、每一条 shell、每一分钱都有审批、审计与预算——这是 Crush 没有的东西，也是 Vivy 已有的东西。

---

## 8. 第二轮核对补遗（同日，三路复查源码后的修正与遗漏清单）

### 8.1 对前文结论的修正

| 前文表述 | 核对结果 |
|---|---|
| "server 模式（SSE 单向）" | 命令名为 `crush server`（无 `serve`）；client/server 架构由环境变量 `CRUSH_CLIENT_SERVER` 开启，TUI 只是客户端之一；server 带完整 OpenAPI 文档（`internal/swagger`）；**仅本地 unix socket / Windows 命名管道，无任何鉴权层**。含 stale-server 版本协商、spawn flock single-flight、`/v1/health` 就绪探测。 |
| "多 agent coordinator（coder/task）" | 仅**硬编码**两个 agent（`SetupAgents`）；`Config.Agents` 标记 `json:"-"`——**用户不能在配置里定义命名 agent**（区别于旧版 Crush 的 agents 配置）。 |
| （未展开）task 子代理工具面 | 只读子集：`glob/grep/ls/view/lsp_definition/lsp_symbols/lsp_call_hierarchy/sourcegraph`，`AllowedMCP` 为空 map = **无 MCP**；提示词仅 16 行，不感知 skills/上下文文件/git。 |
| （易混淆点）compact | Crush **没有 `/compact` 命令**；`compact_mode` 是 TUI 紧凑布局，与上下文压缩无关。摘要入口是命令面板 "Summarize Session" + token 水位自动触发（见 §3B）。 |
| （补充）主题系统 | 实际只有 2 套主题（Pantera 默认 / Hypercrush 品牌），按 provider 映射。比第一轮印象的少。 |

### 8.2 第一轮遗漏清单

**A. CLI 机器接口面**（`internal/cmd/`）

- `crush session list/show/last/delete/rename`：全套 `--json` 机器接口，输出含 cost / tokens / **skills 已加载元数据** / 完整消息 parts；hash 前缀解析 + 歧义候选；`show` 用与 TUI 相同的渲染器 + pager。
- `crush stats`：**自包含 HTML 仪表盘**（非终端表格）——按日/模型/小时/星期聚合、平均响应时长、**工具调用计数**（从 messages.parts 用 `json_each` 挖出）、热力图；`--crawl-dir`/`--all` 跨项目聚合。
- `crush projects`：`projects.json` 项目注册表（path / data_dir / last_accessed，支撑 stats --all）。
- `crush models`：非 TTY 时每行 `provider/model`（机器可读，可接 `crush run --model`）。
- `crush login/logout`：Hyper 与 GitHub Copilot **设备码 OAuth 流**；Copilot 支持从 VS Code `apps.json` 导入既有 token。
- `crush logs --follow/--tail`、`crush dirs`（配置目录可视化）、`crush update-providers --source=catwalk|hyper`、`crush schema`（hidden，provider type 枚举动态注入含本地注册 provider）。

**B. TUI 交互面**（编码 agent 的"手感和安全网"，`internal/ui/`）

- **bang 模式 `!`**：shell 直执行——服务端执行、流式回显为聊天项、esc 可中断、进 prompt 历史、**持久化到会话记录**。
- **消息排队**：agent 忙时 prompt 自动入队（队列 pill 显示），esc 两段式（清队 → 再按取消）。
- **附件链路**：剪贴板贴图 / `ctrl+f` 文件选择器 / 粘贴图片路径 / `@` 文件补全（含 **MCP resource 补全**）；5MB 上限、图片类型白名单；**图片能力由模型元数据 `SupportsImages` 门控**，含 tool result 携图的 provider 兼容 workaround（仅 anthropic/bedrock 允许 tool result 带图，其余降级为占位 + user 消息补 FilePart）。
- 思考块三态视图（collapsed → 尾部 200 行 → 全展开）；REFUSED 拒答横幅。
- **diff 查看器 unified/split 双模式**（权限对话框按 `options.tui.diff_mode` 切换）。
- 通知 4 后端（native / OSC99 / OSC777 / bell）+ 失焦才通知；Kitty 图形协议渲染图片；终端不确定进度条；ANSI16 输出重映射；`ctrl+y` 运行中热切 yolo；`ctrl+o` 外部 `$EDITOR`。
- 命令面板含 Docker MCP Catalog 一键启停（自动探测）。

**C. Prompt 模板与行为规则**（`internal/agent/templates/`，8 个模板——这部分是"agent 行为特性"）

- `coder.md.tpl` 关键规则：**未经用户明说绝不 commit**（commit 遵循含 attribution 的格式）；默认输出 <4 行、禁 emoji；引用用 `file:line` 格式；**LSP 优先编辑策略**（replace_symbol/rename 优先于文本 edit）；错误处理至少 2-3 种补救策略；bash 的 description 参数必填；禁用 bash 跑 curl（用 fetch）；并行工具调用。
- 运行时注入：git branch / status(--short head 20) / log(-3) 快照、平台、日期、上下文文件渲染为 `<project_context>`/`<user_preferences>`、skills XML 目录（`crush://skills/...` 虚拟路径由 view 工具原生读取）。
- `initialize.md.tpl`：生成 `options.initialize_as`（默认 AGENTS.md）；空目录拒绝；探测 `.cursor/rules`、`.cursorrules`、`.github/copilot-instructions.md` 等既有规则文件；原则是"只记录非显而易见的知识"。
- `title` 生成：small→large 回退链、`/no_think` + 空 `<think></think>` 反思考泄漏技巧、40 token 上限、shell 会话以 `"$ cmd"` 命名、标题用量也计费。
- `summary.md`：固定 5 段（Current State / Files & Changes / Technical Context / Strategy & Approach / Exact Next Steps）；**resume 时摘要消息角色改为 User、截断其之前的全部历史、PromptTokens 清零**；自动摘要若打断的是含 tool call 的回合，用改写过的 prompt 重新入队继续。
- 首条 user 消息注入 `<system_reminder>` 空 todo 提醒；非法 JSON 工具参数消毒为 `{}` 并回错误文案。

**D. Provider / 成本 / 缓存细节**（parity 的隐形大头）

- **Anthropic prompt caching**：最后一条 system 消息 + 最后 2 条消息打 ephemeral `cache_control`；`CRUSH_DISABLE_ANTHROPIC_CACHE` 开关；`x-session-id` / `x-session-affinity` 会话亲和头。
- **reasoning/thinking 参数映射大 switch**（`coordinator.go` ~200 行）：openai `reasoning_effort`、anthropic `thinking{budget_tokens}`、google `thinking_config`、openrouter `reasoning{enabled,effort}`、以及 ZAI/DeepSeek/Fireworks/MiniMax/Alibaba/Baseten 等各家 extra_body 特判。
- 计价公式四项：cache_creation / cache_read / input / output；**估算 usage 强制 0 费用**；OpenRouter 用响应内 `usage.cost` 覆盖本地计价 + `:exacto` 模型后缀；Hyper 余额从响应 metadata 侧信道读取。
- 15 种 provider type；`aws_auth_refresh`（Bedrock 凭证过期自动执行命令后原地重试）；`flat_rate`；`system_prompt_prefix`。
- `discover_models`：5 个本地 enricher（ollama / omlx / lmstudio / llamacpp / litellm）探测各自端点回填 context window 等，**只填零值字段**（用户显式配置优先）；litellm 是唯一回填价格的。
- **OAuth token 刷新为跨进程 flock 单飞**，含 refresh-token 轮换防吊销（采纳 peer 新 token / 用 peer 新 refresh_token 重试）；401 → OnAuthRefresh 三分支（OAuth 刷新 / AWS SSO / API key 模板重解析）；refresh token 被吊销 → 阻塞等待交互式重登。

**E. MCP 超集**（不止 tools）

- **prompts 能力** → 自动包装成命令面板条目（取回文本直接作为用户消息发送）。
- **resources 能力** → `list_mcp_resources` / `read_mcp_resource` 内置工具 + 变更通知监听。
- **实验 channels**：隐藏 flag `--channels server:webhook`，MCP server 可主动推 `claude/channel` 事件注入会话。
- Docker MCP Catalog 自动探测注册；无 sampling 支持。

**F. 会话/数据/工程治理**

- **每 session 文件版本历史**（`internal/history`，SQLite 版本链）——轻量 undo 基建，配合 filetracker。
- **配置热重载**（crushrc / hook / model 均有 reload，copy-on-write + pubsub）。
- **内嵌 gojq**：bash 环境免外部 jq 二进制（Windows 受益）。
- **VCR 测试基建**（`charm.land/x/vcr` 录制 LLM 交互回放）——测试策略直接可借鉴。
- herdr 终端复用器状态上报（unix socket JSON-RPC：idle / working / blocked，最佳努力不阻塞）。
- Android/Termux 兼容（dns resolver 替换）；`CRUSH_SERVER_READY_TIMEOUT`、`--channels` 等 env/flag 面。

### 8.3 Crush 明确没有的能力（= Vivy 的差异化/机会清单）

- 无 session 分享/导出；无 ACP（仅注释提及规划）；无 IDE 扩展（仅 Copilot token 导入）；无手动压缩命令（Vivy 的 `CompactSession` RPC 在此点**更强**）；无 watch mode；无 cron/定时任务（Vivy 已有 cron_scheduler）；无多根工作区；无 MCP sampling；server 无远程鉴权层（仅本地 socket）；权限无 hard-deny 层（"可见但必拒"，FUTURE.md 规划中——Vivy 的 policy deny 已实现）。
- 官方 `docs/*/FUTURE.md` 暴露的路线图：agent 实时修改运行时配置（带权限门控）、`state.json` 状态/配置分离、hook `UserPromptSubmit` 事件与 `context_files` 返回、`include_sub_agents` 子代理 hook opt-in。

### 8.4 对 VC 路线图的增补

**VC-0/VC-1 增补**：

- code face 的系统提示词直接以 coder.md.tpl 的规则集为蓝本（绝不擅自 commit、<4 行默认输出、file:line 引用、LSP 优先编辑、错误多策略补救）——这是零代码量的"行为对齐"。
- filetracker 与文件版本 history **合并为一次存储设计**（同为 read_files/versions 族表）。
- UI 附件基线：图片粘贴/上传进入消息链路（composer 附件 stub 已有），按模型元数据门控图片能力 + tool result 携图的 provider workaround。
- 消息排队 + 两段式取消（UI 交互安全网，对应 Crush 队列 pill / esc 语义）。

**VC-2 增补**：

- Anthropic 接线按 §8.5 决策执行（eino-ext/claude 组件）；验收项：prompt caching 生效（注明组件断点策略 = system + tools + 最后一条消息，与 Crush 的 system + 最后 2 条存在已知差异）、thinking/reasoning 参数、四项计价数据源（`CachedTokens` / `GetCacheCreationInputTokens`）。
- 会话自动标题（small→large 回退链）。
- subagent（agent 工具）的工具面照 Crush 语义收窄为只读子集 + 无 MCP——与 Vivy worker 的 PolicySnapshot 精神一致，直接映射实现。
- session 机器接口：Vivy RPC 已覆盖大部分，补齐 cost / skills / 消息 parts 的 `--json` 等价字段即可；token 统计面板加 cache 命中与成本维度（对齐 crush stats 字段集）。

**VC-3 增补**：

- diff 组件选型要求 unified/split 双模式；思考块折叠三态。

**VC-4 增补**：

- MCP resources（list/read 工具）+ prompts（映射进 Vivy 命令/技能体系）；channels 列为实验观察项，不急。
- 自定义命令（markdown + `$NAMED` 参数）与 user-invocable skill 在 Vivy 合并为同一机制（技能市场已是优势面）。
- `vivy init` 生成 AGENTS.md 时采用 initialize 模板要点（只记非显而易见知识、探测既有 .cursor/copilot 规则文件）。
- 测试基建：评估 VCR 式 LLM 录制回放，与现有 scriptedmodel mock 对齐。
- 主动差异化项（Crush 没有的）：cron、policy hard-deny、手动 CompactSession、（潜在）server 鉴权与远程多端——编码场景下这些是 Vivy 的卖点而非负担。

### 8.5 决策记录：Anthropic 后端改用 eino-ext/claude（2026-08-31 拍板）

**决策**：放弃自研 `vivy/anthropic` 适配器里程碑，Anthropic 接线改用 `github.com/cloudwego/eino-ext/components/model/claude`；OpenAI 兼容面（网关/DeepSeek/ZAI/Kimi/自定义 provider）维持 eino-ext openai 组件不变；不引入 deepseek/qwen/gemini 专用组件。

**决策依据**（当日源码核实）：

1. **原决策前提已过时**。`internal/provider/bundle.go` 注释与 `fixtures/provider/anthropic.yaml` provenance 写明自研理由是 "no official Eino Anthropic component exists"（2026-08-07 记录）；eino-ext 现已有 `components/model/claude`，其 go.mod 依赖 eino v0.9.1，与 vivy 锁定的 v0.9.13 同 minor 兼容。
2. **组件能力直接命中 VC-2 验收项**：`AutoCacheControl`（system/tools/尾部消息自动 cache_control 断点，TTL 5m/1h，另有 `SetMessageBreakpoint`/`SetToolInfoBreakpoint` 手动断点）；`CachedTokens` + `GetCacheCreationInputTokens` 用量回报（四项计价的数据源）；thinking 块签名往返（`WithThinking`/`GetThinking`+signature）；Bedrock（AWS credential chain/profile）与 Vertex（service account JSON）原生支持；`mergeAdjacentToolResults` 等 Anthropic 协议严格性处理；anthropic-sdk-go 自带 429/5xx 指数退避。
3. **License 干净**：anthropic-sdk-go MIT、aws-sdk-go-v2 Apache-2.0。P3-1（claude-code upstream LICENSE）与本适配器无关，不再是前置。

**架构影响**：爆炸半径限于 internal/provider——`Ref` 缝隙（`ref.go`，返回 `model.ToolCallingChatModel`）不变，runtime/modeladapter/ADK/mock 无感知；Eino 检疫（仅 internal/runtime、internal/provider 可 import eino）不受影响。

**落地清单**（进 VC-2）：

1. 新增 `claudeRef`（照 `openai.go` 模板，映射 `ModelSpec{ID, APIKey, BaseURL}` → 组件 `Config`）；`catalog.go` backend switch 加 `BackendEinoClaude`；`schemas/providers.bundle.schema.json` backend 枚举加值——bundle schema 是产品契约（D-022..D-025、严格解析），按规则走 `just ci` + outbound `model` 字段断言测试。
2. 修正过时记录：`bundle.go` 注释、`anthropic.yaml` provenance note、`catalog.go` 注释中的 "no official Eino Anthropic component exists"。
3. D-010 防护：`claudeRef` 在 key 为空时先抛 `KeyMissingError`（对齐 openaiRef 模式），Model 恒显式传入；补测试断言不触发组件的 `ANTHROPIC_API_KEY` / `ANTHROPIC_MODEL` 环境变量回退路径。
4. MaxTokens：Anthropic 协议必填，vivy openai 路径现状是 "0 = API 决定"；需 `ModelSpec` 增加 MaxTokens 或 claudeRef 给模型级默认值，避免 0 直发报错。
5. 依赖面：组件无条件 import bedrock/vertex 分支，aws-sdk-go-v2、google auth 等将进入 go.sum 与二进制（即使不用）；license 均干净，供应链审计记入该迭代 verification.md。
6. 缓存策略差异记录：组件自动断点 = system + tools + 最后一条消息；Crush = system + 最后 2 条消息。均在 Anthropic 4 断点最佳实践内，验收时注明即可。

**明确不做**：为 OpenAI 兼容厂商引入 eino-ext deepseek/qwen 等专用组件（openai 组件 + BaseURL 已覆盖且更短）；gemini 原生组件待有真实需求再评估（依赖 google genai SDK，较重）。

### 8.6 决策记录：能力分层与人格模型（2026-08-31 拍板，用户）

对本文件 §3/§5 的两处修正性拍板，随 VIVY-CODE track（`docs/TODO.md` §0.1 VC-0..VC-4）生效：

1. **能力分层（主线内核 / code face / 插件）**：基础工具面（bash、后台 job、grep/glob、multiedit、patch 容错、stale-read 防护、上下文文件注入等）属**主线内核能力**，同步主线，不是 code face 专属——§5 把 VC-1 整体放在 code face 路线下的表述按此修正。code 特有能力（LSP 等）主线**可选**，故 **LSP 插件化**，不进默认 EXE 硬面。每个 VC 任务领取时先归三层归属。
2. **人格模型（不做 Crush coordinator）**：Crush 的命名 agent / coordinator（coder/task 硬编码双 agent）Vivy **明确不做**。Vivy 人格定义同 diva：**一个主人格 supervisor**，可以戴面具（mask/face），面具不影响 vivy 内核（治理/审批/审计仍在内核）。子代理（`agent` 工具触发的 child run）= **可戴面具、无内核、上下文干净**：任务作用域 worker，不携带主人格内核态与父会话历史。§3B "多 agent coordinator ❌→VC-2" 一行按此改写：要补的不是 coordinator，是面具化子代理。

### 8.7 决策记录：VC 决策清单 D1..D11 拍板（2026-08-31，用户）

| # | 决策点 | 拍板 |
|---|---|---|
| D1 | code face 产品形态 | 按研究 §5：vivy.exe 内 "code" face，复用 runtime/Journal/审批/技能，非独立二进制 |
| D2 | FACE-0 | 采纳 `VIVY-FACE-PACK.md`（`face: web\|tui\|headless` 一等装配 + 用户 `seam: face`）；TODO §0.1 FACE-0 随之关闭 |
| D3 | bash 治理边界 | 按研究 §6.1 分级：只读白名单→auto_approve、其余→ask、阻断表（sudo/curl 接 shell 等）→deny 入策略引擎 |
| D4 | LSP 装配形态 | vivy-sdk 独立 module 插件（tool_world seam），默认 EXE 无 LSP，对齐 CH-C4..C7c"默认 EXE 不链协议 SDK"先例 |
| D5 | 子代理边界 | 审批并入父会话（HITL 不豁免）；面具权限参考 diva、可编辑 |
| D6 | 上下文文件注入 | 只读 AGENTS.md（不引 CLAUDE.md/VIVY.md 多文件优先级） |
| D7 | ignore 契约 | 用 .gitignore，不引入 .vivyignore |
| D8 | hooks 治理 | hook 配置变更入 Journal；hook 脚本首次登记需 ask |
| D9 | 成本/模型元数据 | 逻辑与 web 端（provider/model 管理）同步，不另起数据源 |
| D10 | UI diff 呈现 | 对齐 Crush 的 diff 呈现行为（unified/split 双模式 + 增删统计）；FSL 约束下实现自写 |
| D11 | headless 面 | 跟随 D2：`vivy run` 落 face 装配（FACE-0 已采纳） |

**追加拍板（同日）**：

1. **vivy code 不带 channel**：code face 装配排除 channel 耳朵；channel 仍是主线（web）能力。
2. **ACP 不做**：TODO §0.1 ACP-1 由 DEFERRED 改判 WONT-DO。
3. **回退调研立项（RB-1）**：研究"回退怎么做、到底支不支持代码回退"——checkpoint 桥（FR-8 会话恢复）vs 文件版本 history（VC-3 编辑前存档）vs git 语义回退；产出 = Vivy 是否承诺代码回退能力及落点。

注：D4 的插件化把"编辑后诊断回填"的衔接（插件诊断如何并入内核 write/patch 工具结果）变为实现设计点，见 TODO §0.1 VC-3。
