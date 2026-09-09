# Verification

Commands for this delivery:

- `go test ./internal/config ./internal/app ./internal/provider ./internal/runtime ./internal/rpc ./internal/testsupport -count=1` — passed.
- `pnpm test` (from `ui/`) — passed (21 files, 175 tests).
- `pnpm typecheck` (from `ui/`) — passed.
- `pnpm build` (from `ui/`) — passed.
- `just ci` — passed after the final TUI demo-label and saved-model cleanup (format check, vet, Go tests, headless compile, UI install/typecheck/tests/build).
- `pnpm e2e -- e2e/runtime.spec.ts` (from `ui/`, no provider credential) — passed; the browser run showed `Unable to connect! Check the provider configuration!` and asserted no `mock reply`.
- A full `pnpm e2e` run also passed the runtime, MCP, network, sandbox, and generation-parameter paths, while the pre-existing `language-setting`, `model-refresh`, and `welcome-wizard` locator/spec paths remain stale and failed independently of this change (tracked as `E2E-STALE` in `docs/TODO.md`).

No provider secret is stored in the repository, fixtures, journal, logs, or
iteration record.
