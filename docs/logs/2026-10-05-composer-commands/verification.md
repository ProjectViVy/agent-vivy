# Verification

## Regression proof

- Work dock tests failed against the original always-present header and entry buttons, then passed after the layout/entry changes.
- Composer discovery/execution tests first failed for missing tags and commands. Focused composer, ChatView, and store regressions subsequently passed: 84 tests across three files after the review fixes.
- Review regressions exercised the real store admission gate with a delayed Goal cancellation notification, Shift+Enter's actual change event, stale exact submissions, terminal Goals, and session-scoped rewind presets. The rewind reproduction was corrected from the wrong action label, then demonstrated failure with the old retained-preset behavior and passed with session scoping.
- Delayed Work mutation tests failed for an older creation response replacing newer round progress and an A→B→A session response; both passed with version/epoch guards.
- `go test ./internal/app -run '^TestAppPublishesLiveWorkToControlSubscriber$' -count=1` failed with no live notifications, then passed after binding the existing runtime WorkSink to the RPC WorkBus.

## Browser acceptance

`PLAYWRIGHT_CHROMIUM=/usr/bin/chromium pnpm --dir ui exec playwright test --config playwright.composer.config.ts`

PASS: 4 stories, 11.9 seconds. Real backend at `127.0.0.1:8787`, Vite at `http://127.0.0.1:3015`, deterministic loopback model, disposable SQLite/workspace/skills under `.workspace/composer-e2e`. No tenant data or provider secrets used.

- Empty session, slash menu, unsent tag, authoritative Plan entry, reload persistence, exit.
- Real bounded Goal creation, live round/block status, position above composer, clear.
- Native `submit_plan` interruption, exact pending review, `/goal` handoff retaining its submission identity.
- Enabled skill selection, explicit named user request, actual Eino `skill` tool loading the backend's skill body, 390×844 layout without horizontal overflow.

Initial browser failures identified the missing WorkSink composition binding. The mobile test originally attempted the desktop-only New session button after narrowing the viewport; it now creates the session before checking the mobile composer. Final run passed all four stories with no Vite error overlay or browser page errors during page loading. Desktop and mobile screenshots were visually inspected.

## Review and final gate

Read-only review identified four actionable issues: stopped-Goal queue race, rewind-preset session leakage, consumed Shift+Enter newline, and impossible creation over a terminal Goal. All were corrected with regression coverage. The additional runtime bus binding reuses the existing domain WorkEvent path; no new loop, event bus, RPC schema, SDK contract, or Eino implementation was introduced.

`just ci`: the first run passed type checking, 72 UI files / 556 tests, production build, and catalog completeness, then failed cross-face classification for the 24 new Web keys. The keys are now explicitly registered in the existing cross-face contract. The second run passed the UI/I18N gates, vet, backend suites, and SDK pack/eval suite (409.629 seconds), then failed the source-bound conformance reproduction because the WorkSink fix changes the internal source tree. `go run ./sdk/internal/cmd/source-hash internal ""` recomputed `addcd91fed39d8ce4e0beef52adea29538237268cc1e4f158245dfb4ca7674ee`; the five internal-rooted entries in the existing conformance bundle were rebound to that digest, retaining all suite/check identities. The final frozen-source `just ci` passed with exit code 0: formatting, type checking, 72 UI files / 556 tests, production build, both I18N gates, Go vet/tests, headless compilation, and all plugin/face modules. SDK pack/eval passed in 410.888 seconds; executable conformance reproduction and the Generation matrix passed in 74.732 seconds. The environment uses Go 1.26.4 and the existing task-local just/PowerShell/pnpm shims.

No release deployment or external publication is part of this delivery.

Task-owned browser fixtures under `.workspace/composer-e2e` and generated `ui/test-results` were removed after acceptance. The four screenshots remain in this delivery record.
