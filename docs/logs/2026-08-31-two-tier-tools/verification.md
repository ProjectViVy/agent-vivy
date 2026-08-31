# 验证记录 — 2026-08-31 两层工具体系

环境：worktree `../agent-vivy-two-tier-tools`（分支 `feat/two-tier-tools`，
基线 e983b49）。每个 commit 都要求 `just ci` 全绿；前两个 commit 已分别
验证后才落下一个。

## 命令与结果

| 步骤 | 命令 | 结果 |
|---|---|---|
| Commit 1 | `just ci`（worktree，先 `pnpm build` 供 `go:embed`） | 全绿（fmt-check / vet / go test ./... / headless-compile / ui typecheck+test+build） |
| Commit 2 | `just ci` | 全绿（CI_EXIT=0） |
| Commit 3 定向 | `go test ./internal/runtime/ -run 'TestEngineHiddenTools\|TestSelectToolsBinds\|TestToolSurface\|TestToolAdapterAllowsMounted\|TestEinoSkillBackendDeclaredTools\|TestMountedTools\|TestSkillView'` | ok |
| Commit 3 全量 | `just ci` | 首跑在 fmt-check 抓到 `skills_backend.go` 未格式化（gofmt -w 修复）；复跑通过（JUST_CI_EXIT=0，见下） |

> 备注：首轮 ci 曾因 `ui/dist` 缺失在 vet/`go:embed` 失败——worktree 冷启动
> 已知问题，先 `pnpm build` 后即全绿（与 TODO §0.1 UI-CI-BOOTSTRAP 一致）。

## 新增/改写的代表性断言

- `TestServiceRunBindsToolsOnKeywordlessRequest`：中文无关键词消息
  （"你现在有什么工具？"）的 `model.request.selected_tools` 非空——旧选择器
  下该断言必然失败，是本次修复的回归门。
- `TestSelectToolsBindsFullActiveSurface` / `TestEngineHiddenToolsStayOutOfActiveSurface`：
  全量绑定注册序；hidden 进可执行宇宙但不进激活面。
- `TestToolSurfaceMiddlewareFiltersViewModel` / `...ToleratesMissingMountsAndNilInfos`：
  视图过滤（active ∪ mounted ∪ foreign）与空态。
- `TestToolAdapterAllowsMountedTool`：挂载后执行门放行。
- `TestEinoSkillBackendDeclaredTools`：frontmatter `tools:` 解析进
  `SkillSummary.Tools`，且 `SetSkillEnabled` 重渲染不丢声明。
- `TestSaveAndLoadToolsEnabledOverlay` / `TestValidateToolsEnabledOverlay`：
  overlay 三态（缺省/列表/空表）与结构校验。
- `TestToolsCatalogListAndSetActive`：tools/list + set-active 全链路
  （覆盖层写入、OnSettingsChanged 触发、未知/重复名字拒绝、空表=纯对话）。

## 真路径 smoke（已完成，worktree 8790 嵌入式新 UI）

环境约束与变通：3015 被根树另一 lane 的 Vite（`--strictPort`）占用、8787 被
其 backend 占用且 organism lease 单活——smoke 后端改在 `127.0.0.1:8790`
（worktree 自己的 scratch data/，不触碰生产 journal），用 worktree `ui/dist`
的**嵌入式新 UI**（构建于 just ci，含本迭代全部改动）。

| # | 步骤 | 结果 |
|---|---|---|
| 1 | 启动 smoke 后端（新代码） | `healthz ok`，`vivy starting addr=127.0.0.1:8790` |
| 2 | 浏览器（IAB）打开设置→工具 | 真实目录卡片渲染：全量内置工具、只读/需审批徽标、开关状态与配置默认一致（2 个），提示"当前未写覆盖层"——`tools/list` 经真实 UI+RPC 走通 |
| 3 | 关闭 `write_note` 开关 → 保存工具配置 | `tools/set-active` 落盘 worktree `data/settings.yaml`：`tools_enabled: [echo_info]` |
| 4 | 刷新页面重进工具页 | `echo_info=checked / write_note=unchecked`，提示"当前使用自定义覆盖层"——持久化与回显正确 |
| 5 | 新会话发送"你现在有什么工具？你能看到工作区里有什么吗？" | run 走到模型请求后按预期失败（本环境无供应商密钥，"无法连接！请检查供应商配置！"为 mock 移除后的既定错误文案）；**journal `model.request` 记录 `"selected_tools":["echo_info"]`**——中文无关键词消息绑定了非空激活面且遵循覆盖层；旧关键词选择器下该字段必为 `[]` |

未能在浏览器覆盖的项：skill_view 同 run 挂载需要真实模型驱动 ReAct 循环
（本环境无密钥），由单测覆盖（中间件视图过滤、适配器放行、挂载记录、
frontmatter 解析与重渲染保留）。模型真实回复与 SKILL 挂载的用户可见验收，
需在配有密钥的日常实例按 acceptance.md 操作。

清理：smoke 后端已停止，临时 config 端口恢复 8787，浏览器标签关闭；
worktree `data/` 为 per-checkout scratch，保持原样。

