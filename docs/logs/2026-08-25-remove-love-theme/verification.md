# Verification record — Remove the Love theme

## just ci (repository root)

Command: `just ci`. See the addendum below for the result.

## Targeted checks (ui/)

- `pnpm typecheck`: tsc reported no errors.
- `pnpm test`: all 30 cases in 8 test files passed (2026-08-25 06:20).
  The storage fallback case in `use-theme.test.ts` now supplies `'love'` as an
  invalid value and asserts fallback to `DEFAULT_THEME_ID`.

## Browser run (smoke-for-user-visible-change)

Environment: existing split pair (Vite `http://127.0.0.1:3015`, HMR).

1. Refresh `http://127.0.0.1:3015/settings`:
   - The “Theme” card has only 4 buttons: Vivy Blue (default), Minimal Pink &
     White, Deep Blue Night, and Miku Teal; “Love” no longer appears.
   - `<html data-theme="default">`; the theme card renders normally.
2. Leftover-storage fallback is covered by a unit test
   (`readStoredTheme('love') → default`); the index.html bootstrap script uses
   the same ID-allowlist logic.

## just ci addendum

`just ci` (fmt-check + vet + go test + headless-compile + ui-ci) passed after
these changes (exit 0).
