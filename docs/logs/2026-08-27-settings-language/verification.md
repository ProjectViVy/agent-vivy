# Verification record (2026-08-27, Settings → Language)

## Commands run and results

- The repository-root `just ci` (fmt-check → vet → go test ./... → headless-compile →
  ui-ci[pnpm install --frozen-lockfile → typecheck → vitest → vite build]) **passed, exit code
  0**. UI unit tests: 105 passed (15 files), including
  `diva-preview-data.test.ts` (2 updated language-exclusion tests) and
  `i18n/index.test.ts` (9 tests); the Vite production build succeeded (2201 modules).
- Browser real path `just ui-e2e` (pnpm build → Playwright, webServer starts
  `go run ./cmd/vivy`, E2E_ADDR 127.0.0.1:8799):
  - **New `ui/e2e/language-setting.spec.ts` passed (530ms)**: deep link
    `/settings?tab=language` → Language section selected and Chinese copy visible → click
    English → immediate switch to English (`Pick the interface language…`, `Current
    language`) → `localStorage['vivy.language'] === 'en'`,
    `document.documentElement.lang === 'en'` → persists after refresh → switching back to
    Chinese restores it.
  - Existing `runtime.spec.ts` and `welcome-wizard.spec.ts` **failed and were classified as
    stale specs, unrelated to this change**: their two copy assertions ("Secrets are managed only by the runtime",
    "The API key is injected through a runtime environment variable and is not entered or saved in the UI") no longer exist anywhere in
    `ui/src` (the key notices were rewritten by i18n; current copy is in keys such as
    `catalogKeyHint` / `secretNote` in `ui/src/i18n/zh.ts`), and this change did not touch
    the related components (`ModelSettingsCard`, `WelcomeWizard`). They were recorded as
    `UI-E2E-STALE` under `docs/TODO.md` §0.1 per the rulebook's
    「todolist-capture-required」 and are not fixed in this iteration.

## Verification conclusion

- `just ci` is all green; the Language-section real path (switching + persistence + deep link)
  passed completely through Playwright.
- Not verified: none. No interaction regression was run outside the Language section; the two
  stale e2e specs are existing issues, see §0.1.
