# Verification — Split PROC-COMMIT into separate deliveries

Date: 2026-08-26

## Gate: `just ci` (before the split, repository root, against the merged tree)

EXIT=0:

- Go fmt-check / vet / test / headless-compile passed
- UI typecheck passed; **57 tests passed (12 files)** (including 6 cases in
  `demo-api.evolution.test.ts`, 5 in `use-welcome.test.ts`, and 6 in
  `chat-actions.test.ts`)
- `vite build` ✓ (only the existing chunk-size notice)

The split operation itself changed no file contents (only the git index and the
TODO.md entry line); after the split, the HEAD tree == the pre-split worktree, so
the result above is also the gate result for the final commit state.

Records of each delivery’s `just ci` / `just ui-e2e` / 3015 smoke test are in:
`docs/logs/2026-08-25-evolution-page/verification.md`,
`docs/logs/2026-08-25-welcome-wizard/verification.md`, and
`docs/logs/2026-08-26-chat-message-actions/verification.md`.

## Split correctness checks

- Hunk ownership check: each of the 6 hunks in `i18n/zh.ts`, `en.ts` and the 3
  hunks in `runtime.spec.ts` was confirmed to belong to one topic (no mixed
  hunks); en and zh hunk structures correspond one-to-one.
- Cross-topic contamination grep was 0: the `demo-api.ts`/`types.ts` diff had no
  Welcome/Chat entries; `ChatView.tsx`/`MessageBubble.tsx` had no
  Evolution/Welcome references; `ConversationSidebar.tsx`/`SettingsView.tsx`/
  `SkillsView.tsx` had no cross-topic changes.
- Before each commit, `git diff --cached` confirmed that staged content contained
  only that topic (hunk count, stat).
- After the split, `git status` had only `docs/TODO.md` (the PROC-COMMIT closure
  change), so the merged contents of the three feature commits were byte-for-byte
  identical to the pre-split worktree.

## Skip note

- This operation had no user-visible behavior change and no code changes, so
  browser smoke does not apply (smoke is covered by each delivery’s verification.md).
- Intermediate commit states were not each run through CI (see summary.md
  “Explicitly not done”).
