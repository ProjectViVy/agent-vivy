# Verification

## 门禁

- `just ci` — 通过（exit 0）。UI 侧 tsc / eslint / vitest / vite build
  全绿；删除 `useSkills.ts` 未产生任何类型或 lint 断裂，证明其零引用
  判断成立。

## Smoke 说明

无用户可见行为变化（删除零引用的死 hook，页面组件与路由未动），故未跑
`just ui-e2e` 浏览器 smoke；`just ci` 的完整 UI 构建 + 测试即本切片的
充分门禁。`/skills` 页本身的真实 RPC 接线此前已随 UI-E2E 既有 spec 与
2026-08-31 之前的历史切片落地。

## 复核证据（静态）

- `grep -rn "useSkills" ui/src` → 仅命中定义处，零导入。
- `ui/src/routes/_layout.skills.tsx` 直渲染 `SkillsView`，不引 hook。
- `SkillsView.tsx` 全量走 `@/lib/api`（listSkills/getSkill/
  setSkillEnabled/listSkillRevisions），无 demo-api 导入。
- demo-api 技能函数的消费方仅剩 `useEvolution.ts` /
  `EvolutionView.tsx` / `demo-api.evolution.test.ts`（UI-EVO 范围）。
