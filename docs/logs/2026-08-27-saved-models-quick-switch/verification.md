# Verification record — 2026-08-27 「Selected models」 quick switch

## Automated gates

| Command | Result |
|---|---|
| `cd ui; pnpm exec vitest run src/components/settings/saved-models.test.ts` (new cases first) | ✅ 16 / 16 passed (including custom-event and storage-broadcast paths) |
| `cd ui; pnpm typecheck` | ✅ no errors |
| `cd ui; pnpm test` (full vitest suite) | ✅ 14 files / 84 tests all passed (including 16 new `saved-models.test.ts` cases; i18n zh/en structure-sync tests passed, proving the new zh/en entries correspond one-to-one) |
| `just ci` (repository root, = fmt-check + vet + go test + headless-compile + ui-ci[install/typecheck/test/build]) | ✅ all green, ui build `✓ built in 3.55s` (only the existing chunk>500kB warning, unrelated to this change) |

## Browser smoke test (split pair: `just run` :8787 + `cd ui; pnpm dev` :3015)

**Self-testing skipped; delegated to the user's Studio debug session.** When this delivery
was executed on 2026-08-27, `127.0.0.1:8787` (`vivy-backend`) and `127.0.0.1:3015`
(this repository's `pnpm dev` Vite) were both occupied by the user's Vivy Studio debug
session (pid 22900 / 21516). Per plan, the user session was left untouched and dev was not
started separately; the user's Studio session performed the individual browser-path checks.

The user was advised to quickly walk through the core flow in Studio using `acceptance.md`:
add (click a model in the settings page / bookmark from the top bar) → appears in the top
bar → switch → remove → persists after refresh.

## Conclusion

`just ci` is all green (including i18n structure-sync tests), satisfying
`just-ci-is-the-gate`; user-visible behavior was delegated because the ports were occupied
by the user's Studio session, and the `smoke-for-user-visible-change` verification record
is this section of the file plus `acceptance.md`.
