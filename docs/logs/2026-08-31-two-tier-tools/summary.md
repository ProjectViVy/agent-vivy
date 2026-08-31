# 两层工具体系：Active 全量绑定 + Hidden 设置装配 + SKILL 动态挂载

日期：2026-08-31。分支 `feat/two-tier-tools`（worktree `../agent-vivy-two-tier-tools`）。

## 问题

用户报告：聊天中问"你现在有什么工具"，模型回答没有任何工具、不能读工作区。
根因是 2026-08-10 `373927e` 引入的请求级关键词工具选择器
（`internal/tools` `Selector.Select`）：用户消息不含某工具的英文关键词时返回
**空 Selection**，经 `withSelectedTools` 写入 run 上下文，
`toolselection_middleware` 据此把 agent 工具清空，Eino 因工具数为 0 根本不调
`WithTools`，且 preamble 明写 "No tools are selected for this request."——模型
照实回答"没分配工具"。中文消息在 `tokenSet` 切不出英文关键词 token，因此中文
聊天几乎恒为 0 工具。这不是近期 mock provider 移除或后端切换的回归。

## 交付（三个聚焦 commit + 文档）

1. **`fix: bind the full enabled toolset on every request`**
   - 退役关键词选择器：`Engine.SelectTools()` 返回全部 config 已解析工具
     （注册序），每次请求绑定全量激活面；`tools.enabled` 是唯一准入。
   - 静态 Instruction 与 preamble 文案改为"当前 run 列出的工具即可用全集"；
     空集分支保留为合法"纯对话模式"（0 工具）。
   - 默认 `tools.enabled` 补 `list_dir`（与 config.example.yaml、沙箱
     auto-approve 列表对齐；echo_info 仍默认排除）。
2. **`feat: real tools config in settings (active/hidden assembly)`**
   - settings.yaml 新增 `tools_enabled` 覆盖层（nil=配置默认；非 nil 含空表=
     整表替换，空表即纯对话模式）；校验 trim/非空/去重。
   - app 启动折叠覆盖层；`resolveActiveTools` 供启动与每次引擎重建复用；
     保存设置时检测激活面变化并 `ScheduleEngineReload`，改动无需重启进程。
   - 新 RPC `tools/list`（内置注册表全量目录 + active 旗标 + 只读/审批提示）
     与 `tools/set-active`（整表替换；未知/重复名字拒绝）。
   - UI 设置→工具配置 从 demo 假数据（localStorage）换为真实目录卡片，
     逐工具开关、"恢复配置默认"、dirty 提示。
   - `tools.Registry` 新增 `Specs()`（注册序全量目录）与 `Except()`
     （Resolve 的隐藏补集）。
3. **`feat: skill-declared tools mount dynamically mid-run`**
   - SKILL.md frontmatter 新增 `tools:` 列表；`skillFrontMatter.Tools` 为
     canonical 字段，`skill_manage`/`SetSkillEnabled` 重渲染**保留**声明。
   - `SkillSummary.Tools` 下发到 skill_view 结果；查看 SKILL.md（非支撑文件）
     时把声明工具记入 run 级 `tools.MountedTools`（新 ctx 载体，service.drive
     每 run 初始化）。
   - 新 `toolSurfaceMiddleware`（`BeforeModelRewriteState`，Eino v0.9.13
     官方推荐的动态工具面钩子）：可执行宇宙 = active + hidden 全注册，
     模型视图 = active ∪ 已挂载 ∪ 外来工具（如 Eino skill 中间件注入的
     工具，永不被隐藏）。挂在最前，Eino skill / compaction 中间件随后。
   - `toolAdapter` 执行门放行"selected ∪ mounted"；hidden 工具在被挂载前
     不可见也不可调。治理不变：effectful 工具仍走提案/HITL 审批（D-012）。

## 挂载作用域（设计决策）

挂载存活到当前 run 结束：下一 run 模型需重新 `skill_view`（技能正文本就在
transcript 里，重新查看成本低）。会话级 pin、resume 恢复挂载集、journal
挂载事件化见 TODO §0.1（TT-1/2/3）。

## 明示未做

- 不做通用派发器（方案 B，用户已拍板选声明→挂载）。
- 不改 provider/后端；不动 `tool_search` 语义（目录仍限激活集）。
- 不做工具组模板；`model.request.selected_tools` 仍只记 run 前的基面
  （active），挂载增量不回写该事件（TT-3）。
- UI tools tab 之外的设置页未动。
