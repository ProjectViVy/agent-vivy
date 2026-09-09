# Acceptance — 2026-08-25 VIVY UI i18n support

## How a user can confirm it

**Current state (before wiring):**

1. Open `http://127.0.0.1:3015`; the entire UI renders in Chinese (the default
   language), with no bare i18n keys such as `common.retry` exposed. Copy is
   complete on every page (Chat / Dashboard / Cron Tasks / Persona / Masks /
   Evolution / Memory / Notebook / MCP / Settings).

**After wiring (`LanguagePicker` mounted in SettingsView):**

2. Go to Sidebar → “Settings” → the “Language” page; two language cards appear:
   “Simplified Chinese / English”.
3. Click “English”: the entire UI switches to English immediately (navigation,
   settings, and every view update together without a refresh).
4. Refresh the page: it remains in English (`vivy.language=en` is persisted in
   the current browser).
5. Switch back to “Simplified Chinese”: the UI returns to Chinese and is likewise persisted.
6. Observe in the browser address bar that `document.documentElement.lang` switches
   between `zh-CN` and `en` with the language.
7. Cached `vivy.demo.*` demo data retains the language from when it was written
   and is not rewritten when the locale changes.

## Acceptance criteria

- The default language is Chinese, with no regressions in existing UI copy.
- The zh / en dictionaries have the same structure (tests assert aligned leaf
  keys and array lengths).
- The global UI—not just one page—rerenders immediately after switching and
  stays switched after a refresh.
- No new dependencies; no tokens/secrets enter logs or dictionaries.
- `just ci` passes.

## Current follow-up (handed to the other party)

- `docs/TODO.md` §0.1 `UI-SET-I18N`: mount `<LanguagePicker />` on the language
  page in `SettingsView.tsx` (3 lines), then perform acceptance steps 2–7 above.
