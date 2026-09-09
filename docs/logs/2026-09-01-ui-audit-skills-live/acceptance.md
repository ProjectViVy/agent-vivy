# Acceptance

## Manual acceptance

1. Open `http://127.0.0.1:3015` and go to the Skills page: the skill catalog
   comes from the real backend `skills/list` (installed SKILL.md directories),
   and toggling a skill writes real frontmatter (hash CAS). Behavior is exactly
   as before the deletion, with no user-visible change in this slice.
2. At the code level, `ui/src/hooks/` no longer contains `useSkills.ts`;
   `grep -rn "useSkills" ui/src` returns no matches.
3. The Evolution page (`/evolution`) is unaffected—its demo-data paths (such as
   `vivy.demo.skills`) remain unchanged and are handled by the later UI-EVO item.

## Acceptance criteria

- `just ci` is green (no UI tsc/eslint/vitest/build breakage).
- The `/skills` page lists, displays, and toggles skills normally in the browser,
  as it did before the deletion.
