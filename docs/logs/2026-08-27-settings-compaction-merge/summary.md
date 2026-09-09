# Settings → merge Compaction into General (2026-08-27)

## Changes

- Removed the standalone 「Compaction」 section from the settings page:
  `DIVA_PREVIEW_SECTIONS` / `DIVA_ADDITIONAL_SECTIONS` in `diva-preview-data.ts` no
  longer contain `'compaction'`; `SettingsView.tsx` likewise removes the `compaction`
  entry from `DIVA_TAB_LABELS`, so the top tab list no longer shows a 「Compaction」 tab
  with a 「Preview」 badge.
- Moved compaction configuration into the 「General」 section: `GeneralPreview` in
  `DivaSettingsPreview.tsx` adds a 「Context compaction」 card (budget progress bar + three
  inputs for max tokens / compaction threshold / retain recent messages + Run compaction
  preview / Restore preview defaults), placed between 「Chat display」 and 「Cache & runtime
  status」; the General-section description also mentions 「Context compaction」.
- Removed the superseded fake implementation: the standalone `CompactionPreview` preview
  component, the `Minimize2` icon import, and the `case 'compaction'` branch in
  `DivaSettingsPreview`.
- Updated `diva-preview-data.test.ts`: assert that `DIVA_PREVIEW_SECTIONS` does not contain
  `'compaction'`, and remove `'compaction'` from the expected list.
- The `?tab=compaction` deep link now silently falls back to the default 「General」 section
  because of the `isSettingsTab` allowlist change (the router already silently discards
  invalid values, so no router change was needed).
- Also fixed an existing gap for invalid tab deep links: `SettingsView` initialization and
  the initialTab-change effect now clamp through `isSettingsTab(initialTab)`, sending all
  invalid values to the 「General」 section. Previously `?tab=bogus` and similar invalid
  values entered `activeTab` unchanged, leaving Radix Tabs without a matching value and the
  settings page blank (`validateSearch` did not actually filter in this version; debugging
  confirmed `Route.useSearch()` returned the invalid value unchanged). Because
  `?tab=compaction` now follows the invalid-value path, it was fixed in the same delivery,
  so old deep links land on General rather than a blank page.

## Scope notes

- Only the section reorganization was made; other content was untouched: compaction-preview
  copy, defaults, and interaction behavior remain exactly as-is, with only the location and
  card-combination method changing (the former two cards, 「Budget status / Compaction
  configuration」, became one 「Context compaction」 card).
- `src/i18n` was not changed: `settings.tabs.compaction` and `diva.compaction` dictionary
  entries were never referenced (the preview components used hard-coded Chinese), consistent
  with the existing 「Language-section upgrade」 iteration, so they remain untouched.
- Other preview sections (Channels / Network / Self-evolution / Sandbox) remain previews.
- Nothing was committed (commit authorization was not granted).
