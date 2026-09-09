# Verification — VC-1f

Worktree `agent-vivy-vc0` (branch `feat/vc1a-bash-tool`), 2026-08-31.

| # | Command | Result |
|---|------|------|
| 1 | `go build ./...` | exit 0 (after go-udiff integration on the server) |
| 2 | `go test ./internal/runtime -run 'TestBackendPatch\|TestBackendMultiPatch\|TestFilesystemBackend\|TestPatch'` | exit 0 (after updating 2 diff assertions) |
| 3 | `cd ui; pnpm typecheck` | exit 0 |
| 4 | `cd ui; pnpm test` | 24 files / 190 tests all green (including 12 new diff.test.ts cases + 2 DiffView.test.tsx SSR cases; one test expectation bug was corrected during the run: 1 deletion + 2 additions pair by position as 2 lines rather than 3) |
| 5 | `just ci` | The first run failed during `go test ./...` (internal/runtime, 90s, with no specific test name captured); separate reruns of `go test ./internal/runtime` and full `go test ./...` were all green (26 packages, 0 FAIL), judged a timing flake under parallel load and not introduced by this change |
| 6 | `just ci` (rerun, direct collection of the real exit code) | **exit 0** (fmt-check / vet / go test / headless-compile / ui-ci all passed; background task b5s2zi6fc) |
| 7 | `cd ui; pnpm e2e` | 5 passed / 1 skipped / **3 failed**. All 3 failures (language-setting, model-refresh, welcome-wizard) were expired strict-mode locators unrelated to this delivery: the failed elements (`Switch model` button, two `gpt-4o-mini` texts, `Configure model` title) are in Settings/wizard components untouched by this delivery, and `docs/TODO.md` §0.1 `E2E-STALE` already tracks the same failures (found on 2026-08-30; the 2026-08-31 remove-runtime-mock iteration reconfirmed the "same failures on a clean HEAD"). The key regression surface, `runtime.spec.ts` (real browser + real backend chat page), passed |

## Smoke policy note (3015 browser smoke test)

The full browser smoke test for diff rendering requires a real file-change run: producing that run requires a configured
provider key (write approvals/tool results are model-driven). This environment has no key, consistent with the VC-1e smoke-policy
note, so the following approach was used:

- The component-rendering path uses vitest SSR (`renderToStaticMarkup`) to render the real DiffView
  component directly (including i18n and statistics/toggle buttons), asserting the hunk header, `+TWO`, "1 line added, 1 line deleted,"
  aria labels, and Unified/Split button labels—covering the component's real render, not only the parser.
- Playwright e2e's runtime.spec completed the chat-page regression under a real browser + real backend,
  confirming that MessageBubble changes did not break existing interactions (copy/regenerate/hover action bar, etc.).
- The manual acceptance path for an environment with a key is in acceptance.md.
