# Settings → make Language functional (2026-08-27)

## Changes

- Upgraded the settings-page 「Language」 section from an Agent-Diva migration preview to a
  real setting: it now renders the existing but previously unwired `LanguagePicker`
  (`ui/src/components/settings/LanguagePicker.tsx`), immediately switches the global UI
  language on click, and persists it to `localStorage['vivy.language']` across refreshes.
- The Language section no longer shows a 「Preview」 badge and is a first-class section at the
  same level as 「General / Model / Tools / Vivy Features」; the `?tab=language` deep link now
  points to the real section (`SettingsTab` includes `'language'`).
- Removed the superseded fake implementation: the `LanguagePreview` fake-preview component,
  `Languages` icon import, and `language` branch from `DivaSettingsPreview.tsx`;
  `DIVA_PREVIEW_SECTIONS` / `DIVA_ADDITIONAL_SECTIONS` in `diva-preview-data.ts` no longer
  contain `'language'`, and `SettingsView`'s `DIVA_TAB_LABELS` removes the entry as well.
- Updated `diva-preview-data.test.ts`: assert that `DIVA_PREVIEW_SECTIONS` does not contain
  `'language'` and remove `'language'` from the new expected list.
- Added the e2e real-path test `ui/e2e/language-setting.spec.ts`: deep link to the Language
  section → click English → immediate switch to English → localStorage /
  `document.documentElement.lang` assertions → persistence after refresh → switch back to
  Simplified Chinese.

## Scope notes

- Only the routing was connected; nothing else was done: no i18n entries were added
  (`language.*` and `settings.themeSelected` dictionaries already existed with matching
  bilingual structure); the `src/i18n` implementation was not changed; other preview
  sections (Channels / Network / Compaction / Self-evolution / Sandbox) remain previews.
- Settings-page labels (General / Model / Tools / Vivy Features / Language) remain hard-coded
  Chinese, consistent with the existing pattern; copy in the Language card follows the
  current language live.
- Unrelated incidental changes were not restored; nothing was committed (commit authorization
  was not granted).
