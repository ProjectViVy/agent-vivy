# Verification

## Backend and authority

- Red: the new default-authority test failed because the generated assembly had
  no cognitive factory; the governance test rejected all three owner writes.
- Red: the coding-face regression test failed when the newly selected companion
  authority required onboarding for the independent coding product.
- Green: `go vet ./...` and `go test -timeout 35m ./...` (including SDK generation,
  failure matrices, rollback, provider conformance, and middleware gates).
- Green: `go test -tags vivy_headless ./internal/app -count=1` and
  `go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui`.
- Green: every independent Module under `plugins/*` and `faces/*` passed its
  own `go vet ./...` and `go test ./...`.
- Green: default authority persists across restart; existing sessions retain
  the same FrozenCore digest; a new session sees the saved identity; WORLD is
  excluded from the primary model instruction.
- Green: the real headless HTTP/RPC app test rejects uninitialized admission
  with an actionable conflict, initializes via ActionHost under the default
  policy, and observes the identity in the actual model request.

## Development transport smoke

Started a freshly built `vivy_headless` backend on 8787 and Vite on 3015 in the
same execution environment, with a fresh temporary data directory. A WebSocket
client connected through Vite's `/rpc` proxy. It initialized the real authority,
started a turn against a deterministic local OpenAI-compatible model endpoint,
verified the identity in the actual request, saved a new revision, verified a
stale CAS save was rejected, restarted the backend, and verified both frozen
session stability and the new-session identity. This passed. The model stub
made prompt inspection deterministic; no external model-provider result is
claimed.

## Environment limitations

`just ci` could not start because `just` and PowerShell are absent. Its Go,
Module, UI and formatting steps were run directly. The six bootstrap script
tests could not spawn `pwsh` (null process exit status); this is not recorded as
a passing bootstrap gate. The sibling Laputa checkout was manually verified at
the repository's locked commit and used by all Go builds.

A downloaded official Chromium binary could report its version, but browser
launch was blocked by the execution environment's local socket restriction
(`ProcessSingleton: socket() failed: Operation not permitted`). No rendered
browser screenshot or Playwright visual pass is claimed. The developer can
perform the remaining visual acceptance steps in `acceptance.md`.

Automatic approval review rejected the attempted dry-run push because the
public GitHub remote was not explicitly authorized and the operation could
send repository ref/commit metadata. No remote push or PR creation is claimed.

## Final integration checks

- UI: 74 test files / 579 tests passed, including live client dispatch,
  initialization, CAS revision saves, review decisions, stale response handling,
  and pending-operation editor protection. TypeScript typecheck and production
  build passed. Vite reports the existing bundle-size warning (>500 kB).
- I18N: completeness, eight cross-face checker tests, and all 13 shared semantic
  units passed.
- The relative-path app startup regression was observed failing specifically
  with `host data directory must be absolute` before the app-boundary fix.
- No unrelated persona demo persistence functions remain in `demo-api.ts`;
  existing browser data is not deleted or silently imported.
- After the relative-path review fix, the complete headless app package passed
  again (6.341s), `go vet ./...` passed, and headless command/UI compilation passed.
- Final source-digest conformance suite passed (71.711s), binding evidence to
  `f294dabb8a244bf24f5ed5ff014b79a8aa4f16c8348e9ef8bf2c9dc45e16169c`.
- `go run ./sdk verify plugins/vivy-persona` passed.
- Packing `recipes/default.vivy.yml` and inspecting the final artifact passed.
  Generation `613b0e958c0775b1716a0c741cc644c87f52dfa40a6f01a5b58e191bf69bad88`
  contains `vivy/diva-cognitive` with its cognitive factory and control actions,
  plus `vivy/persona` at source digest
  `a939fc68de4fbd0412f95542d00291d6af756873cf40aa0c6f1d63b8c4dc24e2`.
- Tracked Go formatting and `git diff --check` passed.
