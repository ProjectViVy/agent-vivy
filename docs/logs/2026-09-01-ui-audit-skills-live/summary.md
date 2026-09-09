# UI-AUDIT-SKILLS-LIVE — Verify `/skills` wiring and remove the dead hook

## Conclusion

The review disproved the premise of the 2026-08-31 audit item. The `/skills`
page (route `/_layout/skills` → `ui/src/components/skills/SkillsView.tsx`) was
**already fully connected to real RPCs**; there was no issue of missing
`skills/list` / `skills/get` wiring:

- `api.listSkills()` → `skills/list` (`{skills: SkillSummary[]}`)
- `api.getSkill(name, path?)` → `skills/get` (details + supporting files)
- `api.setSkillEnabled(name, enabled, hash)` → `skills/set-enabled`
  (hash CAS; on 409, rereads the catalog—the component comment explicitly
  documents this contract)
- `api.listSkillRevisions()` → `skills/revisions/list` (HITL staged-revision tab)
- The Marketplace tab is gated by the `skills.marketplace` capability.

Error/empty/warning states are all covered: load failures render an error card +
retry; an empty catalog renders an empty-state card; warnings are shown one by
one; master/detail uses `MasterDetail`.

## Actual residue and disposition

The actual demo residue was `ui/src/hooks/useSkills.ts`: a **zero-reference dead
hook** that still imported `listSkills` / `getSkillDocument` /
`createSkillRequest` / `getSkillRequests` from `@/lib/demo-api` (pointing to
`vivy.demo.skills` localStorage). The page uses no hook and calls `api.*`
directly. Per the repository rule "delete confirmed-unused code completely",
the file was deleted.

## Out of scope (explicitly not done)

- The skill functions in `demo-api.ts` (`listSkills` / `getSkillDocument` /
  `updateSkillDocument` / `createSkillRequest` / `getSkillRequests` /
  `acceptSkillRequest` / `rejectSkillRequest`, etc.) **are retained** and are
  still consumed by the Evolution page (`useEvolution.ts` +
  `EvolutionView.tsx` + `demo-api.evolution.test.ts`), which is governed by the
  UI-EVO item (that item treats the whole page as demo data until the MEM-1
  capability proposal enables real RPC); they are unrelated here.
- The `SkillDto` / `SkillDocument` / `SkillRequest` types in `lib/types.ts` are
  likewise retained because the Evolution page uses them.

## Change list

- Delete `ui/src/hooks/useSkills.ts` (the only change).
- `docs/TODO.md`: mark UI-AUDIT-SKILLS-LIVE DONE (review conclusion) and record
  it in §10.
