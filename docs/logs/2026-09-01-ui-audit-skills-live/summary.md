# UI-AUDIT-SKILLS-LIVE — /skills 页接线复核 + 死 hook 清除

## 结论

复核推翻了 2026-08-31 审查行的前提。`/skills` 页（路由 `/_layout/skills` →
`ui/src/components/skills/SkillsView.tsx`）**早已全量接真实 RPC**，不存在
"未接 `skills/list` / `skills/get`" 的问题：

- `api.listSkills()` → `skills/list`（`{skills: SkillSummary[]}`）
- `api.getSkill(name, path?)` → `skills/get`（详情 + 支撑文件）
- `api.setSkillEnabled(name, enabled, hash)` → `skills/set-enabled`
  （hash CAS，409 时重读目录——组件内有注释明确该契约）
- `api.listSkillRevisions()` → `skills/revisions/list`（HITL 暂存修订 Tab）
- Marketplace Tab 由 capabilities `skills.marketplace` 门控

错误/空/警告态齐全：加载失败渲染错误卡 + 重试；目录空渲染空态卡；
warnings 逐条展示；master/detail 走 `MasterDetail`。

## 真实残留与处置

真正的 demo 残留是 `ui/src/hooks/useSkills.ts`：一个**零引用的死 hook**，
仍从 `@/lib/demo-api` 导入 `listSkills` / `getSkillDocument` /
`createSkillRequest` / `getSkillRequests`（指向 `vivy.demo.skills`
localStorage）。页面不经过任何 hook，直接调 `api.*`。按"确定未用即彻底
删除"的仓库规则删除该文件。

## 范围外（明确不做）

- `demo-api.ts` 的技能函数（`listSkills` / `getSkillDocument` /
  `updateSkillDocument` / `createSkillRequest` / `getSkillRequests` /
  `acceptSkillRequest` / `rejectSkillRequest` 等）**保留**：仍被
  Evolution 页（`useEvolution.ts` + `EvolutionView.tsx` +
  `demo-api.evolution.test.ts`）消费，归 UI-EVO 行管辖（该行前提是整页
  demo 数据，等 MEM-1 能力提案后才换真实 RPC），与本行无关。
- `lib/types.ts` 的 `SkillDto` / `SkillDocument` / `SkillRequest` 等类型
  同理保留（Evolution 页在用）。

## 变更清单

- 删除 `ui/src/hooks/useSkills.ts`（唯一变更）。
- `docs/TODO.md`：UI-AUDIT-SKILLS-LIVE 行翻转 DONE（复核结论），§10 登记。
