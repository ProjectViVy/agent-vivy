# Remove the 「Persona」 panel from Settings

Date: 2026-08-27
Scope: `ui/` (the settings-page persona tab and its demo API / i18n / e2e)
Ownership: Vivy UI delivery

## Conclusion

The settings-page 「Persona」 panel has been removed: the tab, panel content, demo API,
types, i18n, and e2e steps were all removed; `just ci` is all green and the `:3015` smoke
test passed.

## Commit split explanation (parallel lanes converged in the same root tree)

This delivery was performed in the shared root worktree at the same time as another parallel
lane (model-settings visual cleanup). That lane's commit
`30c78b8 ui(settings): visual cleanup of the model settings page + theming of the generation-parameter card` merged with my changes in the
same files (`ui/src/components/settings/SettingsView.tsx`, `ui/src/i18n/zh.ts`,
`ui/src/i18n/en.ts`); its commit description noted that "another parallel lane on the shared root tree was deleting the persona-demo tab; its SettingsView section changes were merged into the same files for this change, while that lane's types.ts/demo-api.ts/e2e changes were not included in this commit".

**Therefore, the commit corresponding to this log contains only my remaining slices**:
- `ui/src/lib/types.ts`: removed the `PersonaProfile` interface.
- `ui/src/lib/demo-api.ts`: removed `getPersonaProfile` / `updatePersonaProfile` /
  `MOCK_PERSONA` / `STORAGE_KEYS.PERSONA`.
- `ui/e2e/runtime.spec.ts`: removed the two assertions for 「Settings → Persona tab → persona
  configuration visible」.

The UI section (the tab/panel in `SettingsView.tsx` and the `i18n` entries) landed in
`30c78b8` and is not repeated here.

## Changes (complete delivery checklist)

- `ui/src/components/settings/SettingsView.tsx` (landed in `30c78b8`):
  - removed the settings-page 「Persona」 tab: the full `TabsTrigger value="persona"` /
    `TabsContent value="persona"` content (name / system prompt / save persona demo);
  - removed `'persona'` from `SETTINGS_TAB_VALUES` (the `?tab=persona` deep-link allowlist no
    longer accepts it);
  - removed `persona` state, `getPersonaProfile()` loading,
    `persistDemo('persona')` branch, and the corresponding demo API import; narrowed the
    `demoBusy` / `persistDemo` types to `'model' | 'tools'`.
- `ui/src/lib/demo-api.ts` (this commit): removed `getPersonaProfile` /
  `updatePersonaProfile` / `MOCK_PERSONA` / `STORAGE_KEYS.PERSONA` (referenced only by the
  settings page and now dead code). `MOCK_PERSONA_DOCS`, `getPersonaDocument`, and related
  items used by the sidebar 「Persona」 page are unaffected.
- `ui/src/lib/types.ts` (this commit): removed the `PersonaProfile` interface used only by
  the settings page.
- `ui/src/i18n/zh.ts` / `en.ts` (landed in `30c78b8`): removed `settings.tabs.persona`,
  `settings.personaTitle` / `personaDescription` / `savePersonaDemo` / `name` /
  `systemPrompt`, and `demo.personaProfile` (referenced only by the removed `MOCK_PERSONA`).
- `ui/e2e/runtime.spec.ts` (this commit): removed the two 「Settings → Persona tab → persona
  configuration visible」 steps; retained the assertion that the sidebar 「Persona」 link
  reaches the persona page (which still exists).

## Not done

- The sidebar 「Persona」 page was not removed (`/persona`, `PersonaMemoryView`, `persona.*`
  i18n, `getPersonaDocument`, etc.)—the user asked only to remove the panel from Settings.
- `DIVA_PREVIEW_SECTIONS` (Channels/Network/Language/Compaction/Self-evolution/Sandbox
  preview sections) and the Model/Tools/Vivy Features tabs were not touched.
- Other unused `settings.tabs.*` keys (general/model/tools/vivy/preview labels) were not
  removed; this round cleaned only keys directly related to the Persona panel.
