# Verification

Date: 2026-08-26

## Gate: `just ci` (repository root)

Result **EXIT=0** (including the rerun after two rounds of user-feedback
convergence): Go fmt-check / vet / test / headless-compile, UI typecheck
(`tsc --noEmit`), **57 passed (12 test files)** (including 6 cases in the new
`src/lib/chat-actions.test.ts`), and `vite build` succeeded (only the existing
chunk-size notice).

## E2E: `just ui-e2e`

Result **2 passed (23.2s, including the rerun after user-feedback convergence):**

- All new assertions in `runtime.spec.ts` passed:
  - Assistant-message action bar: Copy visible, Regenerate **enabled**, Rewind /
    Fork **disabled**;
  - User messages (ChatGPT reference): action row defaults to `opacity: 0`, then
    becomes `opacity: 1` after `hover()` (hover reveal is pinned as the e2e
    contract); Edit **disabled**, Copy present, Rewind / Fork **absent**;
  - After granting clipboard permissions
    (`grantPermissions(['clipboard-read','clipboard-write'])`) and clicking Copy,
    the button changes to “Copied”, and
    `navigator.clipboard.readText()` reads back `mock reply to: hello vivy`.
- `welcome-wizard.spec.ts` regression passed.

## Live-path smoke: http://127.0.0.1:3015 (split Vite, real control plane)

- The action bar renders on every message in an existing conversation: user
  messages (Copy + Edit placeholder + Rewind / Fork placeholders), assistant
  messages (Copy + enabled Regenerate).
- **Copy**: after clicking, the button changes to “Copied”. Note: the IAB
  embedded-browser guest renderer rejects the Clipboard API, so the
  `execCommand` fallback succeeds (introduced specifically for this scenario and
  verified in the real browser).
- **Regenerate**: clicking Regenerate on the last assistant message follows
  preflight → turn/start → streaming reply, with message count 6 → 8 (resent user
  input + appended new answer), matching the append-only Journal mapping.
- Found and fixed during smoke testing: visible DOM returned page coordinates while
  CUA clicks used viewport coordinates, so buttons outside the scroll area did not
  respond; after scrolling them into the viewport, all clicks landed.
- **Second convergence check** (removed timestamp + moved Edit button to the left
  of the bubble): DOM geometry asserted button right edge 1150 ≤ bubble left edge
  1156 (`leftOfBubble: true`), with vertical centers 111.4 vs 111.4 exactly
  aligned; the snapshot showed timestamps only on assistant messages
  (16:31/16:32/03:39), and no timestamp on user messages.

## Skip note

- `go test` was not run separately (`just ci` covers it; there were no Go-side changes).
