# 2026-08-25 VIVY UI i18n support

## Changes

Added complete i18n support to the Vivy species UI, with the language switcher
at “Settings → Language”. No dependencies were added; the implementation follows
the same pattern as `hooks/use-theme.ts` (module-level state +
`useSyncExternalStore` + localStorage persistence).

### i18n infrastructure (`ui/src/i18n/`)

- `index.ts`: locale registry (zh / en), dot-path lookup for `t(key, params)` +
  `{{param}}` interpolation, `useTranslation()` Hook (subscribed components
  rerender automatically when the locale changes), `setLocale()` /
  `getLocale()` / `dateTimeLocale()`, localStorage persistence (key
  `vivy.language`), browser-locale detection (after localStorage, zh→zh /
  en→en), and `document.documentElement.lang` synchronization.
- `zh.ts`: canonical dictionary (default locale zh, keeping tests with existing
  Chinese assertions unchanged). `en.ts`: strong type alignment via
  `Dictionary = typeof zh`.
- `index.test.ts`: zh/en leaf-key consistency + array-length alignment, missing-
  entry fallback, key fallback, interpolation, dot-path indexes for array leaves,
  setLocale persistence + DOM lang, and locale-following localeOptions.

### Copy migration scope

- **Shared/layout/navigation/chat components**: the sidebar, top bar, input area,
  sessions, and so on all render through `t()`.
- **Views**: approvals / audit / cron / lifecycle / masks / notebook / persona /
  planning / skills / all demo views (DemoBanner, Memory, Dashboard, Mcp,
  TokenStats), plus ThemePicker / LanguagePicker.
- **lib layer**: error messages and runtime copy in `rpc.ts`, `runtime-config.ts`,
  `store.ts`, `run-subscription.ts`, and `useSkills.ts` are all localized;
  `demo-api.ts` now generates demo-data seeds through `t()` on first write.
- **Demo-data strategy**: cached `vivy.demo.*` data retains the language from when
  it was written (like user data) and is not rewritten on switch; new strings
  generated at call time (new-session default names, errors, reports, replies)
  are all localized.
- **Key fixes**: added the `@` alias to `vitest.config.ts` (5 lib test files
  previously could not resolve `@/i18n`); changed `mask-catalog.ts` capabilities
  to use a `t()` key for fallback boundary detection (arrays are not returned directly).

### Explicitly not done (handed off / follow-up)

- **SettingsView wiring**: `LanguagePicker.tsx` is ready (clicking it calls
  `setLocale` → global switch + persistence) but is **not mounted** on the
  language page in `SettingsView.tsx`. That file is owned by another parallel
  workflow (`settingview is being edited by someone else; ignore it`), so it was
  not touched here. The current Settings-page
  “Language Preview” label is the other implementation’s preview; clicking it
  was confirmed not to change global copy. Wiring only requires mounting
  `<LanguagePicker />` on the language page (3 lines), recorded in
  `docs/TODO.md` §0.1 `UI-SET-I18N`.
- **Other party’s files**: `SettingsView.tsx`, `DivaSettingsPreview.tsx`,
  `diva-preview-data.ts`, and their tests were not changed.
- **`reveal-engine.ts`**: template file (marked “Do not modify business code”),
  containing only an internal `console.warn`; it was not touched.
- Remaining CJK exists only in code comments and demo keyword matching (Chinese
  keywords in `sendMessage`, used for data matching rather than copy).
