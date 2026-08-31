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

## 真路径 smoke

merge 回主线后在根树执行（记录于本目录 verification.md 追加节）：
`just dev` → http://127.0.0.1:3015 → 中文消息验证模型实际调用工具、
设置页切换激活态、声明 `tools:` 的技能同 run 调用隐藏工具。
