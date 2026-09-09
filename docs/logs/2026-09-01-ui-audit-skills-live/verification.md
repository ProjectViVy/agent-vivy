# Verification

## Gates

- `just ci` — passed (exit 0). UI tsc / eslint / vitest / vite build are all
  green; deleting `useSkills.ts` caused no type or lint breakage, confirming the
  zero-reference finding.

## Smoke notes

There is no user-visible behavior change (the zero-reference dead hook was
deleted, while page components and routes were untouched), so browser smoke via
`just ui-e2e` was not run; the complete UI build + tests in `just ci` are the
sufficient gate for this slice. The `/skills` page's real RPC wiring was already
covered by existing UI-E2E specs and earlier slices before 2026-08-31.

## Static review evidence

- `grep -rn "useSkills" ui/src` → only the definition location, with no imports.
- `ui/src/routes/_layout.skills.tsx` renders `SkillsView` directly and does not
  use the hook.
- `SkillsView.tsx` uses `@/lib/api` throughout (`listSkills`/`getSkill`/
  `setSkillEnabled`/`listSkillRevisions`) and has no demo-api import.
- The only remaining consumers of the demo-api skill functions are
  `useEvolution.ts` / `EvolutionView.tsx` / `demo-api.evolution.test.ts` (UI-EVO
  scope).
