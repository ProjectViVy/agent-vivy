# Verification

Date: 2026-08-27

Scope: chat-input toolbar port (Agent-DIVA → Vivy, UI only). Changes are in
`ui/src/components/chat/ChatInput.tsx`, `ui/src/lib/store.ts`,
`ui/src/routes/_layout.tsx`, `ui/src/i18n/zh.ts`, and `ui/src/i18n/en.ts`.

## Gate: `just ci` (repository root)

Result **EXIT=0**: Go fmt-check / vet / test / headless-compile all passed
(internal/app 3.229s, etc.; the rest cached); UI `tsc --noEmit` passed;
vitest **15 files / 105 tests passed** (including 9 i18n-entry parity cases in
`index.test.ts`—a hard check that zh/en structures match—and 4 `store.test.ts` cases);
`vite build` succeeded (only the existing >500 kB chunk-size notice). No component-level
unit tests were added on the UI side (ui has no @testing-library foundation; automation was
not introduced for this small change, with coverage from typecheck + vitest + a real-path
smoke test).

## Real-path smoke test: http://127.0.0.1:3015 (split Vite :3015 + real control plane :8787)

Prerequisites: :3015 (PID 9276) and :8787 (PID 22900) were already running (the existing
split `just dev` pair), so the smoke test used the current Vite dev server directly
(source changes took effect immediately). It was driven by `@playwright/test`
(chromium-1234, headless), with `vivy.ui.welcome.completed=1` set first to skip the
initial wizard. The throwaway script was destroyed and not checked in.

Result **22/22 PASSED** (`SMOKE RESULT: PASSED`):

- All 8 toolbar items were present in the DOM in DIVA order: mode trigger (Agent mode),
  attachments, thinking mode, AutoDream, open companion, permission trigger (Smart),
  History, and Approvals.
- 「Draw」 and 「Smart」 buttons were confirmed removed.
- Mode dropdown: after opening, menu items were 「Agent mode (execute tasks directly) /
  Plan mode (plan first, then execute) / Ask mode (read-only analysis mode)」; selecting
  「Plan mode」 updated the trigger text to 「Plan mode」.
- Thinking dropdown: menu items Auto / On / Off.
- Permission dropdown: menu items 「Cautious (all operations require confirmation) / Smart
  (automatically allow low-risk operations) / Trusted (confirmation required only for
  high-risk operations)」; selecting 「Cautious」 updated the trigger.
- Three stubs: clicking Attachments → 「Attachments not connected yet」, AutoDream →
  「AutoDream not connected yet」, and companion → 「Desktop companion not connected yet」
  (a bottom `aria-live` notice bar disappearing after about 1.8s).
- History (clock) → the right-side 「Sessions」 Sheet opened; Approvals → the right-side
  「Approvals」 Sheet opened.
- Bilingual: after setting `vivy.language=en` and reloading, button names were Agent mode /
  Thinking mode / History / Approvals / Smart, with `documentElement.lang=en`; restoring zh
  worked normally.

The smoke test found and corrected one test-script-only issue: the first i18n step did not
wait for the chat area to remount after reload (the fixed 2.2s wait occurred before
rendering), causing a false assertion failure. Waiting for the textbox with `waitFor`
before asserting passed; the product feature itself had no issue, and a DOM probe confirmed
lang=en and complete English copy on the toolbar buttons.

## Skipped items

- `go test` was not run separately (`just ci` covered it; this round had no Go-side
  changes).
- `just ui-e2e` was not run (two existing e2e specs already fail because of stale copy;
  see `docs/TODO.md` §0.1 `UI-E2E-STALE`, unrelated to this iteration). This iteration
  used a real-path Playwright smoke test instead; it can be run as regression coverage
  after UI-E2E-STALE is fixed.
