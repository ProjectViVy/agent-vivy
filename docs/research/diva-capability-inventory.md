# Diva 能力清单（P2-1）— Keep / Adapt / Defer / Drop

> **Date:** 2026-09-02
> **Closes:** TODO §0.1 行 P2-1（`AGENT-VIVY-ASSEMBLY-OPTIONS.md` §5 的 V0 占位由本文档取代为全量清单）；PRD v0 行 23 的"deferred to a later artifact"即此文档。
> **Evidence base:** `morediva/agent-diva`（Rust 工作区，17 crates）2026-09-02 现场盘点——README/AGENTS-ARCH.md/LAPUTA.md、各 crate 目录、`agent-diva-providers/src/providers.yaml`（47 个 provider 预设，grep `- name:`）、`agent-diva-gui/src-tauri/src/commands.rs`（grep `#[tauri::command]` = 176+1）。逐条证据在行内标注文件路径；无法核实的条目显式标注"未核实"。
> **Status 语义：** 每个 tag 是**清单推荐**。凡"Keep 但未交付"的能力，仍必须走 ASSEMBLY-OPTIONS §6 的能力再入流程（提案 → 设计 → 架构决策 → 实现 → 验证）才允许进入 Vivy——tag 不是实施授权。

## 0. Tag 语义（承 ASSEMBLY-OPTIONS §5/§6 与 PRD 行 23）

| Tag | 含义 |
|---|---|
| Keep | 能力本身属于 Vivy 产品核心；Vivy 已交付或应通过提案再入。 |
| Adapt | 值得要，但必须换形态/换架构再入（不是照搬 Diva 的做法）。 |
| Defer | 现在不动；等真实使用场景或前置能力（多数等 MEM-1 / CH 轨道 / SystemV）。 |
| Drop | 明确不带走：license 风险、无消费者、或被 Vivy 架构取代。 |

**Vivy 现状参照（2026-09-02）**：内核 = session/run/event + eino 执行 + provider 目录 + 工具（fs/bash/grep/multiedit/skill）+ 审批/HITL + compaction + faces/masks + skills 市场 + cron + MCP + token 统计 + browser UI（3015）+ 插件通道（telegram/discord/feishu/qq/dingtalk/lsp）。逐条对照见表。

## 1. 会话 / 对话 / 运行

| # | Diva 能力 | Diva 证据 | Tag | Vivy 现状 / 再入去向 |
|---|---|---|---|---|
| 1.1 | Session CRUD + 自动标题 | `agent-diva-gui/src-tauri/src/lib.rs:369+`（get/update/generate/delete_session 等） | Keep | **已交付**（internal/session + session RPC + UI；标题=会话名） |
| 1.2 | Agent 循环（context assembly + skill/subagent 流） | `AGENTS-ARCH.MD` CODE MAP；`agent-diva-cli/src/main.rs` | Keep | **已交付**（internal/runtime + eino ADK） |
| 1.3 | 流式输出 + reasoning + 工具日志 | `agent-diva-cli/src/main.rs` | Keep | **已交付**（run_events 订阅；UI 流式面板） |
| 1.4 | 事件总线 | `agent-diva-core/src/`（event bus 目录） | Keep | **已交付**（run_events + RPC 订阅） |
| 1.5 | Token 台账 | `agent-diva-core/src/` token_ledger（未读内部） | Keep | **已交付**（stats/tokens + 轨迹投影 token 用量） |
| 1.6 | Heartbeat / rate limiter / presence | `agent-diva-core/src/`（目录名核实，行为未读） | Defer | 无当前消费者；多通道在线态等 CH 轨道驱动 |
| 1.7 | audit / audit_parse / audit_sink | `agent-diva-core/src/`（目录名核实） | Adapt | 需求已被 D-010 红action + 结构化日志覆盖大半；剩余（审计流导出）无消费者 |
| 1.8 | supervised / quality / experience 模块 | `agent-diva-core/src/`（仅目录名） | Defer | MEM-1 家族邻接；等能力提案 |
| 1.9 | 重启恢复 / 终态唯一 | `agent-diva-agent/` recovery 语义 | Keep | **已交付**（run 恢复 + TT-2 快照；ASSEMBLY-OPTIONS §8 门禁全过） |
| 1.10 | 消息编辑 / 回退 / 分叉（截断式重生成） | GUI 命令簇 | Keep（未交付） | **UI-CHAT-ACT 在办**——需内核 Journal 截断/分支 RPC 设计片先行 |

## 2. Provider / 模型

| # | Diva 能力 | Diva 证据 | Tag | Vivy 现状 / 再入去向 |
|---|---|---|---|---|
| 2.1 | 47 个 provider 预设 | `agent-diva-providers/src/providers.yaml`（grep `- name:`） | Adapt | Vivy 用**精选** YAML bundle（D-018）；广度只随真实消费者增长，不整体照搬 |
| 2.2 | 网关前缀 model 重写 | providers/（AGENTS.md 规则同源） | Keep | **已交付**（仅真聚合网关前缀，出站 model 断言测试在位） |
| 2.3 | 自定义 OpenAI 兼容端点 | providers/src/ | Keep | **已交付**（provider 目录 + bundle） |
| 2.4 | 模型目录 / 元数据（含 thinking 标注） | providers/ catalog | Keep | **已交付**（ModelInfo/SupportsImages/SupportsThinking，D9 单一事实源） |
| 2.5 | 转写（speech-to-text） | providers/src/ 文件列表 | Defer | 无场景；等真实需求 |
| 2.6 | 重试 / 请求观察者 | providers/src/ | Keep | **已交付**（resolving 模型每调用构造 + 重试面；轨迹可见重试行） |
| 2.7 | CLI provider 登录/切换 | `agent-diva-cli/src/main.rs` | Defer | Vivy 面 = 设置 UI + config；CLI 形态无行 |

## 3. 工具

| # | Diva 能力 | Diva 证据 | Tag | Vivy 现状 / 再入去向 |
|---|---|---|---|---|
| 3.1 | filesystem / shell / web(grep) 工具 | `agent-diva-tools/src/` | Keep | **已交付**（VC-1 工具族：read/write/edit/multiedit/glob/grep/bash） |
| 3.2 | attachment / message / read_tool_result | `agent-diva-tools/src/` | Keep | **已交付**（附件 VC-1g-2 链路；read_tool_result 同族） |
| 3.3 | cron 工具 | `agent-diva-tools/src/` | Keep | **已交付**（内核 cron + UI + at 调度 UI-CRON-P2） |
| 3.4 | spawn（子代理） | `agent-diva-tools/src/` | Adapt | 子 run（children RPC）已交付；"把子代理暴露为工具"再入需提案（审批语义复杂） |
| 3.5 | ask_user | `agent-diva-tools/src/` | Keep | **已交付**（HITL-P0 询问流；review/list 含 question） |
| 3.6 | planning / update_plan / execution_todo | `agent-diva-tools/src/` | Keep | **已交付**（RunMode plan + 会话待办面板 + TodoProgressStrip） |
| 3.7 | working checkpoint | `agent-diva-tools/src/` | Defer | eino CheckPointStore 桥已在内核（D-028），产品级暴露无消费者 |
| 3.8 | tool_discovery / skill_view 挂载 | `agent-diva-tools/src/` | Adapt | skill_view 已交付；TT-1 会话级 pin 在办；更广的 discovery 等真实场景 |
| 3.9 | mcp_sdk 工具 | `agent-diva-tools/src/` | Keep | **已交付**（MCP 管理页 + 工具接线） |
| 3.10 | memory_* 工具族 | `agent-diva-tools/src/` | Defer | MEM-1（DEFERRED） |
| 3.11 | actmem（active-memory 写入） | `agent-diva-tools/src/` | Defer | MEM-1 |
| 3.12 | wtf 工具 | `agent-diva-tools/src/`（用途未核实） | Drop | 用途不明 + 无消费者；若将来弄清再走提案 |

## 4. 沙箱 / 权限 / HITL

| # | Diva 能力 | Diva 证据 | Tag | Vivy 现状 / 再入去向 |
|---|---|---|---|---|
| 4.1 | ExecPolicy 规则式审批 + 审批缓存 | `agent-diva-sandbox/src/lib.rs:1-18`、`exec_policy.rs` | Keep | **已交付**（审批中心 HITL-P0 + 权限三段预设 UI-CHAT-TOOLBAR） |
| 4.2 | Guardian 自动放行 | `agent-diva-sandbox/` | Adapt | Vivy 以预设（谨慎/智能/信任）承担同职责；"自动学习放行"等 SBX 真实需求 |
| 4.3 | Windows Restricted Token 隔离 | `agent-diva-sandbox/src/lib.rs` | Defer | SBX-*（DEFERRED）； approval 闸门已覆盖当前风险面 |
| 4.4 | Linux Bubblewrap/Landlock/Seccomp | `agent-diva-sandbox/src/lib.rs` | Defer | 同上（Vivy 当前 Windows-first） |
| 4.5 | 持久审批中心（decide/cancel/review + 流） | CLI `approvals`；GUI 命令簇 | Keep | **已交付**（审批中心页 + run 级审批事件） |
| 4.6 | 命令规则 CRUD | GUI 命令簇 | Adapt | 预设已覆盖；逐规则 UI 等真实治理需求 |

## 5. 通道 / 网关

| # | Diva 能力 | Diva 证据 | Tag | Vivy 现状 / 再入去向 |
|---|---|---|---|---|
| 5.1 | 六通道（Telegram/Discord/QQ/DingTalk/Feishu/Email） | `agent-diva-channels/src/*.rs` | Adapt | Vivy 以**插件**形态交付 5 通道（dingtalk/discord/feishu/qq/telegram）+ CH-0 契约 docs；email 与其余归 CH 轨道（拍板：留待办） |
| 5.2 | 已退役通道（Slack/WhatsApp/Matrix/IRC/Mattermost/Nextcloud） | README cargo features | Drop | 无消费者 |
| 5.3 | 网关=会话/路由单一事实源 + HTTP 控制面 | README "How it works" | Keep | **已交付**（vivy 控制面 8787 + 消息注入面） |
| 5.4 | `POST /api/hook/message` 外部注入 | README | Adapt | CH 轨道内（外发/补跑通道等 CH-0 家族） |
| 5.5 | neuro_link 通道 | `agent-diva-channels/src/neuro_link.rs`（用途未核实） | Drop | 同 wtf：用途不明 |
| 5.6 | Windows service 包装 | `agent-diva-service/` | Defer | Vivy 以前台进程 + Studio 生命周期覆盖；服务化无行 |

## 6. 记忆 / 人格 / 自演化

| # | Diva 能力 | Diva 证据 | Tag | Vivy 现状 / 再入去向 |
|---|---|---|---|---|
| 6.1 | BML 记忆权威（SQLite+FTS5 `.laputa/`） | `agent-diva-laputa/src/lib.rs:1-8` | Defer | MEM-1（DEFERRED） |
| 6.2 | ACTMEM 工作记忆（ring/capsule） | laputa | Defer | MEM-1 |
| 6.3 | MEMRULES / 记忆蒸馏 | laputa | Defer | MEM-1 |
| 6.4 | Laputa 治理层（提案制写权威 + 回滚 + 审计） | `AGENTS-ARCH.MD`（仅 `apply_proposal()` 可写权威） | Adapt | 治理**模式**已被 Studio 生命周期（pack→eval→release→rollback）吸收；记忆域本身等 MEM-1 |
| 6.5 | 人格 Markdown 工作区 + Frozen Core | laputa | Adapt | Vivy 有 persona 页 + faces 轨道（FACE-TUI-1 F0 paperwork 已落）；冻结快照语义随 FACE 提案 |
| 6.6 | AutoDream 提案生命周期 | `agent-diva-autodream/src/lib.rs` | Defer | MEM-1；UI AutoDream 维持 stub（拍板） |
| 6.7 | 进化控技能请求（create/accept/reject） | autodream | Defer | 同上 |

## 7. 技能 / 文件 / 工作区

| # | Diva 能力 | Diva 证据 | Tag | Vivy 现状 / 再入去向 |
|---|---|---|---|---|
| 7.1 | SKILL.md 加载（用户+repo 回退） | skills 加载面 | Keep | **已交付**（skills 目录 + enable/disable） |
| 7.2 | 技能市场（搜索/安装/上传） | skills.sh 面板 | Keep | **已交付**（SKILL-MKT-1 版本比对+原地升级、SKILL-MKT-2 always 注入） |
| 7.3 | 技能历史/修订 | 技能修订 RPC | Keep | **已交付**（skills/revisions/list） |
| 7.4 | 文件索引 / FileManager / 上传 | `agent-diva-files/` | Keep | **已交付**（工作区 + 文件面板 + file_versions + 附件） |
| 7.5 | 工作区检查/切换 | GUI workspace 命令簇 | Keep | **已交付**（WorkspaceManager 多工作区） |

## 8. 面具 / 计划 / 定时

| # | Diva 能力 | Diva 证据 | Tag | Vivy 现状 / 再入去向 |
|---|---|---|---|---|
| 8.1 | 面具（人格预设）CRUD/切换 | CLI `mask`；GUI 命令簇 | Adapt | Vivy 把 masks 升级为真实 run 模式 + Face 契约（VC-0c；FACE 轨道）；Diva 式纯文案预设不照搬 |
| 8.2 | 计划审批流（get/approve/report） | GUI 命令簇 | Keep | **已交付**（plan 模式 + 审批链） |
| 8.3 | Cron CRUD + 时区 + 通道投递 | CLI/GUI；runs inside gateway | Keep | **已交付**（cron 页 + at 调度）；"经通道投递"归 CH 轨道 |

## 9. GUI / 客户端形态

| # | Diva 能力 | Diva 证据 | Tag | Vivy 现状 / 再入去向 |
|---|---|---|---|---|
| 9.1 | Tauri GUI（177 命令） | `#[tauri::command]` grep 176+1 | Drop（形态） | Vivy 以 browser UI（3015 split Vite / 8787 embedded）覆盖同需求；逐能力已散入上表对应行 |
| 9.2 | 桌面宠物 Mate（VRM/TTS/置顶窗） | lib.rs 命令簇（VRM import、MiniMax/SiliconFlow TTS） | Defer | 无内核能力；等提案（UI-CHAT-TOOLBAR 行已注明） |
| 9.3 | 语音合成输入 | 同上 | Defer | 同上 |
| 9.4 | 多语言切换 / splash / GUI 偏好 | 命令簇 | Adapt | Vivy i18n en/zh 已交付；偏好持久化按 UI 需求增长 |
| 9.5 | 日志尾随 / token 面板（GUI 内） | 命令簇 | Keep | **已交付**（token 统计页 + 轨迹面板 UI-TRAJ） |
| 9.6 | wipe_local_data | 命令簇 | Adapt | 数据归属是 Vivy 哲学锚点（local-first）；"一键重置"随设置页提案 |

## 10. CLI / 内核支撑面

| # | Diva 能力 | Diva 证据 | Tag | Vivy 现状 / 再入去向 |
|---|---|---|---|---|
| 10.1 | CLI 全家（onboard/chat/tui/status/doctor/…） | `agent-diva-cli/src/main.rs` | Defer | Vivy 当前控制面+UI 双面；CLI 无行。FACE-TUI-1 的 faces/tui 是 TUI 形态的**脸**，不是 Diva 式全功能 CLI |
| 10.2 | neuron（单轮 LLM 节点基元） | `agent-diva-neuron/src/lib.rs` | Drop | eino compose/graph 已覆盖组合需求 |
| 10.3 | manager（网关运行时+HTTP 控制面） | `agent-diva-manager/` | Keep（形态异） | Vivy 的 cmd/vivy + internal/rpc 即同职责 |
| 10.4 | migration 工具 | `agent-diva-migration/` | Drop | 无 Diva 用户数据可迁 |
| 10.5 | e2e 真 LLM 测试 harness | `agent-diva-e2e/src/lib.rs` | Adapt | Vivy 有 Playwright + Go 全测；"真 provider e2e 门"已用（TEST-1 决策）；更重 harness 无消费者 |
| 10.6 | 打包（NSIS/MSI/deb） | `scripts/package-*.ps1` | Adapt | Vivy 分发 = embedded UI exe + split build + Docker；安装器随发布提案 |

## 11. 统计

68 行：**Keep 29**（已交付 28 + 待提案 1：1.10 消息编辑/回退/分叉）· **Adapt 15** · **Defer 18** · **Drop 6**。

> 注：计数按行（部分行合并多个子工具）。"已交付"判定以 2026-09-02 TODO §10 与 `docs/logs/` 为准。**Defer 重仓区与拍板一致**：MEM-1 家族（6.1-6.3、6.6-6.7、3.10-3.11、1.8）、SBX（4.3-4.4）、CH 轨道余量（5.1 部分、5.4）。Drop 集中在"用途不明"（3.12、5.5）、"被架构取代"（9.1、10.2）与"无迁移对象"（10.4）。

## 12. 再入规则重申（ASSEMBLY-OPTIONS §6）

本清单不改变任何当前 TODO 优先级。任何 Defer→实施、Adapt→实施的转换都必须先产出能力提案（问题陈述 / 工作流 / 验收 / 安全审批模型 / 状态与事件语义 / 持久化恢复 / UI 后果 / 显式 tag 决议），**不得**从 Diva 的 crate 边界、Tauri 命令名、旧 schema 或后端实现出发。
