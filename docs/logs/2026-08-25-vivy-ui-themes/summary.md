# Vivy UI theming

> **2026-08-25 follow-up**: the `love` theme was removed following user feedback;
> 4 themes are now active. See `docs/logs/2026-08-25-remove-love-theme/`. The section below is the original delivery-time record.

## Changes

Vivy UI grew from “only a default light theme” to 5 switchable themes. All theme
values use shadcn semantic Tokens (the new convention), and selection takes
effect immediately and persists in the current browser.

- `ui/src/hooks/use-theme.ts` (new): the single theme entry point—a registry
  (id / label / description / appearance / preview-palette literals),
  `applyThemeToDOM` (sets `data-theme`; dark appearances also add the `.dark`
  class to drive Tailwind `dark:` variants), `vivy.theme` localStorage
  persistence, and `useTheme()` backed by a module-level
  `useSyncExternalStore` store.
- `ui/src/styles.css`:
  - Migrated the `.dark` token block to `[data-theme="dark"]` (values unchanged,
    Deep Blue Night theme); the `.dark` class now only toggles `dark:` variants
    and no longer carries token values.
  - Added three complete token blocks: `[data-theme="love"]` (Love, light),
    `[data-theme="pink"]` (Minimal Pink & White, light), and
    `[data-theme="miku"]` (Miku Teal, GitHub Dark base + support teal, dark).
    Values are converted from the Agent-Diva `love / default / miku` theme
    variables into semantic Tokens (`--background/--primary/--sidebar-*`, and so on).
- `ui/index.html`: the anti-flash inline script now reads `vivy.theme` and
  applies `data-theme` + `.dark`; removed the old iframe parent-window
  light/dark push bridge (a demo leftover with no consumers). Inline background
  colors are approximated per theme.
- `ui/src/components/settings/ThemePicker.tsx` (new): the real theme-selection
  cards on Settings → General (preview gradient + name + description + selected
  state), placed between “Application Info” and the migration preview.
- `SettingsView.tsx`: wired `ThemePicker` into the General tab; moved `DemoNote`
  so it covers only the migration preview, avoiding a false demo label on the
  real theme cards.
- Removed fake operations: the fake theme card in the `DivaSettingsPreview`
  General preview that said “selection does not change Vivy’s global theme,”
  `DIVA_THEME_PREVIEWS` in `diva-preview-data.ts`, and the `'theme'` section ID
  (the real feature replaced them).
- Tests: added `ui/src/hooks/use-theme.test.ts` (8 cases: registry integrity,
  ID validation, storage fallback, DOM application and `.dark` switching, and
  invalid-value ignoring); removed the theme-preview assertions from
  `diva-preview-data.test.ts` as well.

## Theme list

| id | Name | Appearance | Source |
|---|---|---|---|
| `default` | Vivy Blue | Light | Original `:root` default theme (unchanged) |
| `love` | Love | Light | Ported from Agent-Diva `love` |
| `pink` | Minimal Pink & White | Light | Ported from Agent-Diva `default` (Vivy’s default was already occupied, so it was renamed according to its “Minimal Pink & White” meaning) |
| `dark` | Deep Blue Night | Dark | Original `.dark` token block (like Agent-Diva `dark`, with a dark base and blue accent; existing values retained with zero regression) |
| `miku` | Miku Teal | Dark | Ported from Agent-Diva `miku` |

## Explicitly not done

- Diva’s gradient App background, glassmorphism, and floating cherry-blossom/
  heart decorations were not ported: under the new convention, themes express
  only semantic Token value differences, and large gradient backgrounds are
  outside the token system.
- `--radius` is not changed per theme (fixed at 12px) to avoid regressions in nested corners.
- Themes are not sent through the backend Settings RPC: theming is a browser-local
  preference (at the same level as the active-session key) and is not written to
  runtime configuration.
- The `<title>` in `index.html` was not changed (it remains the old demo title,
  recorded in §0.1 UI-TITLE).
- No sidebar quick-switch entry was added; this round only implements the Settings-page cards.

## Verification

See `verification.md` for verification; the manual acceptance path is in `acceptance.md`.
