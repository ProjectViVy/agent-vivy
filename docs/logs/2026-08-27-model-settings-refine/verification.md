# Verification record — 2026-08-27 model-settings interaction refactor

## Automated gates

| Command | Result |
|---|---|
| `cd ui; pnpm typecheck` | ✅ no errors |
| `cd ui; pnpm test` (full vitest suite) | ✅ 15 files / 105 tests all passed (i18n zh/en structure-sync tests passed; this iteration was a UI-only refactor with no new pure-logic tests) |
| `just ci` (repository root, = fmt-check + vet + go test + headless-compile + ui-ci[install/typecheck/test/build]) | ✅ all green, ui build `✓ built in 4.80s` (only the existing chunk>500kB warning) |

## Browser smoke test (split pair: `just run` :8787 + `cd ui; pnpm dev` :3015)

**Self-testing skipped; delegated to the user's Studio debug session.** `127.0.0.1:8787`
(`vivy-backend`) and `127.0.0.1:3015` (this repository's `pnpm dev` Vite) were still
occupied by the user's Vivy Studio debug session (pid 22900 / 21516); the user conducted
this review in Studio and verified the browser paths.

The user was advised to quickly walk through `acceptance.md` in Studio:

1. The persistent pencil button on a custom-provider row → the edit dialog appears (change
   address/alias);
2. 「Sync from official」 and 「Add」 buttons appear in the model-list header, and 「Add」
   can manually add and immediately apply a model;
3. The bottom form is gone, and API Key appears above the model list (editable for custom
   providers, disabled for catalog providers).

## Conclusion

`just ci` is all green (see the commit message/conclusion line), satisfying
`just-ci-is-the-gate`; user-visible behavior was reviewed and verified directly by the
user in the Studio session, and the `smoke-for-user-visible-change` record is this section
plus `acceptance.md`.
