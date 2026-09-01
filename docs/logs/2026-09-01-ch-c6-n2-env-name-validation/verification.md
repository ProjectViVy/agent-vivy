# Verification — CH-C6-N2

Environment: worktree `agent-vivy-vc0`, branch `feat/vc1a-bash-tool`.

| Command | Result |
| --- | --- |
| `go vet ./internal/channelhost/ ./internal/config/` | ok (after fixing missing test imports `bytes`, `log/slog`) |
| `go test ./internal/channelhost/ ./internal/config/ -count=1 -race` | `ok agent-vivy/internal/channelhost 12.497s` / `ok agent-vivy/internal/config 1.983s` |
| `gofmt -l ./internal/channelhost ./internal/config` | empty (no formatting diffs) |
| `just ci` | exit 0 — fmt-check, vet, full go test, headless compile, plugin-ci (6 modules), UI tsc/eslint/vitest/vite build all green |

## New tests

- `TestSecretRefusesMalformedSettingsEnvName` — envelope settings declare
  `client_id_env: bad-name` (malformed) and
  `client_secret_env: VIVY_TEST_GOOD_NAME` (valid). The malformed variable
  is actually set (`bad-name=leak-attempt`) so the refusal is observable
  rather than vacuous: `Secret("bad-name")` errors and the error does not
  carry the value; the valid sibling still resolves to its value.
- `TestStartAllWarnsMalformedSettingsEnvName` — StartAll over a buffer
  logger emits exactly one warning containing
  `malformed environment variable name`, the settings key `client_id_env`,
  and the declared name `bad-name`; the valid sibling emits nothing.
- Pre-existing `envKeyPattern` config tests still pass unchanged
  (`agent-vivy/internal/config` ok), confirming the export did not alter
  parse-time behavior.

## Gate

`just ci` exit 0 (fmt-check, vet, full go test, headless compile,
plugin-ci ×6, UI tsc/eslint/vitest/vite build).

## Notes

- First `-race` attempt failed on `TestStartAllWarnsMalformedSettingsEnvName`
  with `journal, messages, and sessions stores are required` — the test
  host was built without the stores StartAll requires. Fixed by building
  the host with `openBackend(t)` + `runRecorder` like the other StartAll
  tests; not a product-code failure.
