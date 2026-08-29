# Skill 真实化 + 技能市场（skills.sh 适配）

- 日期：2026-08-30
- 分支：`feat/skill-marketplace`（worktree `../agent-vivy-skill-market`，符合
  parallel-worktree-isolation：根树当时有未提交的 compaction 改动，且与本迭代文件重叠）
- 参考：agent-diva 的 skills.sh 市场适配
  （`agent-diva-manager/src/marketplace.rs`、`data/marketplace_featured.yaml`、
  `scripts/fetch_marketplace_featured.py`、`MarketplaceTab.vue`）

## 背景

Vivy 内核侧 Skill 早已是真实现（`EinoSkillBackend` + Eino skill 中间件 +
`skills_list/skill_view/skill_manage` 工具 + RPC `skills/list`、`skills/get`），
假的只有 UI：`/skills` 页走 `demo-api.ts` 的 localStorage 演示数据，`api.ts` 里的
真实客户端无人调用，且 UI 期望的 `SkillDto` 形状与真实载荷不匹配。DIVA 的
"插件市场"实际是技能市场（对接 skills.sh），本次把该逻辑移植到 Vivy。

## 变更

### 内核（Go）

- `internal/tools/skills.go`：`SkillSummary` 增加 `enabled`；`SkillOperations`
  增加 `SetSkillEnabled`（控制面 CAS 启停，不走 HITL 暂存）；新增
  `SkillsMarketplace` 接口与 `MarketplaceSkill/MarketplaceFeatured/
  MarketplaceInstallResult` DTO。
- `internal/runtime/skills_backend.go`：frontmatter 解析增加 `enabled`（缺省
  true）；Eino `List/Get` 只暴露 enabled 技能（disabled 的 `Get` 报错，控制面
  `ListSkills/ViewSkill` 仍可见全部）；`SetSkillEnabled` 以内容哈希做 CAS、
  原子改写规范化的 frontmatter（内容保留）。
- `internal/runtime/marketplace.go`（新）：skills.sh 适配器 —— `Search`（q≥2
  字符、limit 钳制 1..50 默认 20）、`Featured`（go:embed 离线快照）、
  `Install`（`owner/repo/slug` 三段 id、段白名单、create-only 冲突报错、
  SKILL.md frontmatter name 必须与 slug 一致、附属文件仅
  references/templates/scripts/assets、512KiB/文件、128 文件、5MiB 总量、
  拒二进制、路径规范化、失败整体回滚、装完用真 loader 复检并返回 SkillView、
  越界路径记入 skipped_files）。上游失败返回 `MarketplaceUpstreamError`；
  `VIVY_SKILLS_MARKETPLACE_URL` 环境变量可覆盖 base URL。
- `internal/runtime/marketplace_featured.yaml`（新）：内置 featured 排行快照
  （自 DIVA 快照移植，generated_at 2026-08-20）。
- `scripts/fetch_marketplace_featured.py`（新）：快照刷新脚本（有 token 走
  leaderboard API，无 token 抓首页排行；`VIVY_SKILLS_MARKETPLACE_TOKEN`）。
- `internal/config`：新增 `runtime.skills_marketplace_url`（默认
  `https://skills.sh`，校验必须是绝对 http(s) URL）。
- `internal/rpc/control.go`：新增方法 `skills/set-enabled`（409=哈希过期）、
  `skills/revisions/list`（skill_manage 暂存修订只读列表）、
  `skills/marketplace/search|featured|install`（上游失败 → 新错误码
  `-32010`，UI 映射 502）；`initialize` capabilities 增加
  `skills.marketplace` / `skills.revisions`（按依赖注入情况广播）。
- `internal/app/app.go`：装配 `MarketplaceService`（skills_root 配置时）并注入
  ControlDeps；`SkillRevisions` 指向 Journal。

### UI（React）

- `ui/src/lib/api.ts`：新增 5 个 RPC 方法与类型（`MarketplaceSkill`、
  `MarketplaceFeatured`、`MarketplaceInstallResult`、`SkillRevision`、
  `SkillSummary.enabled`）；`mapCode` 增加 `-32010 → 502 bad_gateway`。
- `ui/src/components/skills/SkillsView.tsx`：整页改走真实 RPC（对齐 MCP 页的
  `import * as api` 模式），三页签：已安装（列表 + 详情 + 启停开关 + 警告 +
  附属文件点击查看）、市场（按 capability 显示）、变更请求（真实待审修订只读
  卡片，含预览/警告/运行绑定）。启停 409 时自动重拉目录。
- `ui/src/components/skills/MarketplaceTab.tsx`（新）：300ms 防抖搜索（≥2
  字符），无搜索时展示 featured 排行（快照日期），安装数 k/m 格式化，已安装
  按 slug 禁用按钮，busy 态，失败可重试，安装成功刷新已安装列表。
- `ui/src/routes/_layout.skills.tsx`：移除 `DemoBanner`。
- `ui/src/i18n/{zh,en}.ts`：skills 域新增 marketplace/启停/修订键（zh 为基准，
  en 镜像）；保留 Evolution 页仍在用的 `builtin/user/alwaysLoaded/...` 键。

### 明确不做

- Evolution 页仍为 demo（`UI-EVO`，等内核 Evolution/AutoDream 能力提案）。
- 技能内容变更（create/edit/patch/delete）仍只走 `skill_manage` + HITL 评审，
  UI 市场页不提供内容编辑。
- 市场技能的更新/升级路径未做（DIVA 同为删除重装）；`always` 常驻注入未移植。
  已登记 TODO `SKILL-MKT-1`、`SKILL-MKT-2`。
- 启停开关不产生 SkillRevision 审计行（控制面元数据翻转，非内容变更）。
