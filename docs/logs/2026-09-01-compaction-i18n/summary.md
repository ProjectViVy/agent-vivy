# Fix raw i18n keys in the Settings → General compaction card (UI-I18N-COMPACTION)

## Symptom

The "Context compaction" card under Settings → General rendered
`settings.compaction.*` as raw keys (the title, description, switch label, all
three form labels, and the save/compaction buttons displayed the literal key
names). Several pieces of copy in the card were also hardcoded in Chinese
(`Max tokens`, `Compaction threshold (%)`, `Keep recent messages`,
`0 = model context window…`, `Session feed usage`, `{{percent}}% used`,
`Last compaction: …`, `Saving…`, `Compacting…`, the `Auto` placeholder, and the
empty-state hint), so they appeared in Chinese
even in the English interface.

## Root cause

The keys were in the wrong section, not missing: `zh.ts`/`en.ts` did contain a
`compaction: { … }` block, but it was attached under the `diva` section
(`diva.compaction`), while the component reads `settings.compaction.*`. The key
mismatch made every lookup miss, so `t()` fell back to rendering the raw key.
There were no `diva.compaction` references anywhere in the repository; the
block was a dead key block placed in the wrong location when the compaction
configuration graduated into real settings.

## Fix

- `ui/src/i18n/zh.ts` / `en.ts`: moved the `compaction` block into the
  `settings` section (an equivalent zh/en key migration) and added 11 keys:
  `saving`/`compacting`/`maxTokensLabel`/`maxTokensHint`/`autoPlaceholder`/
  `triggerLabel`/`keepRecentLabel`/`feedUsage`/`pressureBadge`/`lastCompaction`/
  `openSessionHint`. These absorb all hardcoded copy in the card; the zh copy
  is character-for-character identical to the original hardcoded text (no
  visible change for Chinese users), while en contains the new translations.
- `ui/src/components/settings/CompactionSettingsCard.tsx`: changed 10 hardcoded
  Chinese strings to `t()` calls, including interpolation parameters for
  `pressureBadge`/`lastCompaction`.
- Added the `ui/e2e/compaction-setting.spec.ts` regression spec: with zh as the
  default language, it asserts the card title and three form labels; it asserts
  that the full page has no `settings.compaction.` raw key; after switching to
  English through localStorage and reloading, it asserts the English labels and
  that the card contains neither Chinese leftovers nor raw keys.

## Explicitly not done (separate TODO)

- The General section of `DivaSettingsPreview` (chat display, cache and runtime
  status, About Vivy, and the "Compaction configuration has graduated" migration
  note) is hardcoded in
  Chinese throughout. It is a pre-existing preview-area debt unrelated to this
  card and is recorded in TODO row `UI-DIVA-PREVIEW-I18N`.
- The `General` tab trigger in the settings page's `SettingsView.tsx` is also
  hardcoded in Chinese (a neighboring problem on the same page, recorded in the
  same debt row).
