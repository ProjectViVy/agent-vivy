# 2026-08-25 Settings-page layout and description consistency

## Changes

The Settings page (`ui/src/components/settings/`) was made consistent according
to oil-frontend conventions (visual engineering + information and actions):

1. **Kept “General & About”**: after that preview section was removed in the
   previous round, it was fully restored at the user’s request (`GeneralPreview`
   in `DivaSettingsPreview.tsx`, the mount point in `SettingsView.tsx`, and the
   `diva.general` copy block in the zh/en dictionaries were all restored).
2. **Unified card-header structure**: the Run Inspector card on the “Vivy
   Features” page in `SettingsView.tsx` used a one-off “icon beside title” layout;
   it now follows the standard “icon above + title + description” structure used
   by ThemePicker / Lifecycle cards.
3. **Removed the responsibility-free DemoNote**: the General page’s DemoNote
   claimed that “changes are saved only to vivy.demo.* localStorage,” but no
   content on that page writes to vivy.demo.* (ThemePicker uses real persistence,
   and the preview area has its own Agent-Diva declaration). It was copy without
   a clear information/action responsibility and was removed.
4. **Added a “Preview” marker to preview tabs**: the six Agent-Diva migration
   preview tabs (Channels / Network / Language / Compression / Self-Evolution /
   Sandbox) were indistinguishable from real tabs, so each now has a small
   “Preview” marker that makes the real/preview boundary clear in the tab bar.
5. **Filled in two missing preview-card descriptions**: the Network page’s
   “Current Preview Summary” and the Compression page’s “Compression
   Configuration” now have descriptions consistent with the other cards on each
   page (explaining the scope of preview data), removing the inconsistency where
   some cards had descriptions and others did not.

## Blocking fixes (left over from a parallel workflow)

The worktree contained uncommitted changes from another parallel i18n/masks
workflow, which prevented the app from mounting and blocked `just ci`:

- `mask-catalog` was changed to a functional export (`maskOptions()`), but
  `MaskAndModelSwitcher.tsx` and `MaskManagementView.tsx` still referenced
  `MASK_OPTIONS` (undefined at runtime → blank app).
- Module-level `GenerationSelect` in `LifecycleView.tsx` used `t` without it being
  defined in scope (type error + runtime ReferenceError).

These three locations were mechanical, unambiguous fixes required to unblock
verification; they were fixed and recorded together.

## Explicitly not done

- **i18n was not wired**: `DivaSettingsPreview` still hard-codes Chinese, no
  component consumes the `diva.*` dictionaries (zh/en), and `LanguagePicker.tsx`
  was created but not mounted. These are in-progress parts of the parallel i18n
  workflow and were not touched here; they are recorded in `docs/TODO.md` §0.1
  (UI-SET-I18N).
- ThemePicker / LanguagePicker (parallel-stream files) were not changed.
- No masks / lifecycle business logic was changed; only exports and scope were fixed.
