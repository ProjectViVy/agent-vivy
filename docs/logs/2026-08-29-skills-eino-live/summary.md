# 2026-08-29 · Skill 真正可用（Eino middleware + 真实目录 UI）

Date: 2026-08-29
Status: complete for this concern
Lane: root tree (shared working copy was already dirty with unrelated sandbox/settings work)

## Outcome

Skills 不再只是 demo localStorage。模型通过 Eino `adk/middlewares/skill` 发现并按需加载 `skills_root` 里的 `SKILL.md`；人在 `/skills` 看到同一份真实目录。Skills 页上的「变更请求」Tab 已删除——它来自 Evolution 演示数据，与 `skill_manage` / `skill_revisions` HITL 无关。

## Delivered

### Runtime

- `EngineConfig.SkillBackend`：非 nil 时在 `toolSelectionMiddleware` **之后**挂 `einoskill.NewMiddleware`。
- 这样 Eino 的只读 `skill` 工具不会被关键词选择丢掉；用户没说 “skill” 也能看到目录。
- `app.New` 把已构造的 `*EinoSkillBackend` 同时交给 Engine 与 RPC。nil 指针不赋给 interface，避免 typed-nil。
- 本迭代只做 inline 加载。`context: fork` 仍按 Eino 原生产错。

### RPC

- `skills/list`、`skills/get`（`name` 必填，`path` 可选）。
- Wire 对齐 backend：`name` / `description` / `hash` / `warnings`；get 另含 `content` / `relative_path` / `supporting_files`。
- 未接线 → method not found。越界 path / 缺失 skill 失败。
- capabilities 增加 `skills.list` / `skills.get`。

### UI

- `useSkills` 只走 `@/lib/api`，不再 import `demo-api`。
- `/skills` 去掉 DemoBanner、变更请求 Tab、假的启用/内置/始终加载徽章。
- 列表身份 = 名称 + 描述；警告非空时显示；点选加载正文；附属文件走 `skills/get` 的 `path`。
- 空状态说明放到 `skills_root` 后刷新。不提供 UI 编辑/安装（变更仍走 `skill_manage` + Review Center）。
- Evolution 页继续用 `vivy.demo.*`。

## Explicitly not done

- fork / AgentHub / ModelHub。
- Skills 页上的 `skill_manage` 表单。
- MCP / Evolution / Memory / Cron / 通道 / 聊天占位按钮接真实后端。
- 不往 `data/`、`data/dev-home/`、`~/.vivy` 写种子技能。
- 不删 `docs/TODO.md` §0.1 的 `UI-*` 行。
- 整树 `just ci`：根树被并行 lane 弄脏（sandbox / settings / filesystem），全量门禁红。本交付用隔离测试 + app RPC 冒烟验收。
