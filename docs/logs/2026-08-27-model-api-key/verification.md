# Verification record — 2026-08-27 end-to-end model-key support

## Automated gates

| Command | Result |
|---|---|
| `go test -count=1 ./internal/app/... ./internal/rpc/...` | ✅ all passed (settings round-trip + validation, RPC api_key write/clear/no-leak, three new app-overlay cases for injection/empty value/missing file) |
| `cd ui; pnpm typecheck` | ✅ no errors |
| `cd ui; pnpm test` (full vitest suite) | ✅ 15 files / 105 tests all passed (including 3 new `apiKey`/`customApiKeyFor` cases, the old-entry missing-value-to-empty-string case; i18n zh/en structure-sync tests passed) |
| `just ci` (repository root, = fmt-check + vet + go test + headless-compile + ui-ci[install/typecheck/test/build]) | First blocked by `settings_overlay_test.go` not being gofmt-formatted; after `gofmt -w` ✅ all green—ui build `✓ built in 3.53s`, all 105 tests passed (only the existing chunk>500kB warning) |

## Browser smoke test (split pair: `just run` :8787 + `cd ui; pnpm dev` :3015)

**Self-testing skipped; delegated to the user's Studio debug session.** When this delivery
was executed on 2026-08-27, `127.0.0.1:8787` (`vivy-backend`) and `127.0.0.1:3015`
(this repository's `pnpm dev` Vite) were still occupied by the user's Vivy Studio debug
session (pid 22900 / 21516). Per plan, the user session was left untouched and dev was not
started separately; the user's Studio session performed the individual browser-path checks.

The user was advised to quickly walk through the core flow in Studio using `acceptance.md`:

1. Enter an API Key when adding a custom provider;
2. Click its model → `data/agent-home/settings.yaml` is stored with `api_key`;
3. Switching from the top bar/chip carries the key; switching back to a catalog model
   clears the key (falls back to env);
4. After refreshing the page, the 「API Key configured」 notice remains; `settings/get`
   returns only `api_key_set`.

## Conclusion

`just ci` is all green (fmt-check blocked first → passed after gofmt), satisfying
`just-ci-is-the-gate`; user-visible behavior was delegated because the ports were occupied
by the user's Studio session, and the `smoke-for-user-visible-change` verification record
is this section of the file plus `acceptance.md`.
