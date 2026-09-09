# Verification

Commands run from worktree `agent-vivy-vc0` (branch `feat/vc1a-bash-tool`), 2026-09-01.

| Command | Result |
| --- | --- |
| `go test ./internal/app/settings/ -count=1` | ok (including new `TestSaveConcurrentWritersKeepDocumentValid`) |
| `go test ./internal/app/settings/ -run TestSaveConcurrent -count=3 -race` | ok |
| `just ci` (golangci-lint + gofmt + go test ./... + UI tsc/eslint/vitest/build) | exit 0, twice (once before and once after the UI change) |
| `just ui-e2e` (full suite, 10 specs) | **9 passed, 1 skipped (runtime.spec provider gate with no provider key), exit 0** |
| `npx playwright test e2e/model-refresh.spec.ts` (isolated single-spec rerun) | passed (used to distinguish a race from an ordering dependency) |

## Process evidence (diagnostic chain)

- The first full run failed in three places: model-refresh (`providersError
  "internal error"`), runtime (the no-provider branch text did not appear), and
  welcome-wizard (raw i18n key + invalid step assertion).
- The Playwright error-context snapshot showed a frozen model-refresh page with an
  `internal error` paragraph and the registry missing my-local-model; runtime's
  send hit the dead port left by model-refresh at `http://127.0.0.1:30123`
  (`dial tcp ... refused` in vivy.log), producing the generic "This conversation
  did not complete" warning card. The welcome-wizard model step rendered the raw
  `welcome.provider` key and its prefill was polluted by model-refresh.
- `internalError()` (`internal/rpc/control.go:3429`) returns the fixed
  "internal error" for every settings.Load error (without leaking details);
  `settings.Save`'s old fixed `path+".tmp"` and unlocked rename caused the
  concurrency failure—the regression test reproduced
  `settings: commit: rename ... Access is denied` reliably before the fix.
- After the fix, the full e2e suite ran green repeatedly; model-refresh restored
  the global runtime configuration to openai / gpt-4o-mini at the end (the
  aria-pressed flip confirmed the save RPC completed), so later specs were no
  longer polluted by the temporary upstream.
