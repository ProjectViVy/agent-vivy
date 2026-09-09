# UI title naming: VIVY

## What changed

- `ui/index.html`: static `<title>Agent Diva Frontend Demo</title>` → `<title>VIVY</title>`
  (naming approved on 2026-09-02).
- `ui/src/i18n/en.ts` + `ui/src/i18n/zh.ts`: `app.documentTitle` `Vivy` → `VIVY` —
  `__root.tsx:20` dynamically overrides `document.title` after mounting, unifying the
  static and runtime values in both languages.
- Added `ui/e2e/app-title.spec.ts`: the real 3015 path asserts
  `page.toHaveTitle('VIVY')` (the same welcome-skip setup as existing specs), making the
  title a permanent regression check.

## Explicitly not done

- The "Vivy" naming in i18n copy such as `app.loading`/`app.cannotConnect` is outside this
  line (that is product nomenclature, not the title; the decision scope was "UI title").
- The top-bar wordmark and Studio skin (submodule) were untouched.

## Notes

- The loading-splash wordmark already rendered `VIVY` (`__root.tsx`); this only aligned the
  two browser-tab sources.
