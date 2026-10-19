# Verification — approval quick-card merge

Environment: Windows, `node v24.19.0`, `pnpm 10.11.0`, `just 1.46.0`.
Commands ran from the repository root.

## Focused tests

```text
pnpm --dir ui exec vitest run \
  src/components/approvals/ApprovalQuickCard.test.tsx \
  src/components/chat/ChatInput.compact.test.tsx \
  src/components/chat/ChatInput.queue.test.tsx \
  src/components/chat/ChatInput.commands.test.tsx
→ 4 files, 41 tests passed (6.8s)

pnpm --dir ui typecheck
→ tsc --noEmit, exit 0
```

## Full UI and i18n gates

```text
pnpm --dir ui test
→ 81 files, 638 tests passed (28.6s)

node scripts/check-i18n-completeness.js
→ PASS: en=1582 keys / 162 placeholders; zh=1582 keys / 162 placeholders

node scripts/check-i18n-cross-face.js
→ PASS: 13 shared semantic units

node --test scripts/check-i18n-cross-face.test.js
→ 8 tests passed

pnpm --dir ui build
→ vite v7.3.6, 2583 modules; dist/assets/index-BVbr17nz.js (1,846.05 kB)
  and dist/assets/index-DmdYSmj2.css (154.54 kB)
```

## Real split-path smoke

Against the running split pair (`just dev`, backend `:8787`, Vite `:3015`), a
headless Playwright page opened the chat, skipped the welcome wizard, and:

- found the composer shield button (`aria-controls="approval-quick-card"`);
- clicked it and rendered the quick card with `快速审批`, the empty state
  `当前没有待处理事项`, and the `审批中心` link;
- recorded no browser console or page errors.

A real pending approval was not triggered because this dev home has no provider
API key; Approve/Deny and the pending-selection logic are covered by
`ApprovalQuickCard.test.tsx` (9 tests).

## Not run

- `just ci`: stopped by the owner before this merge and not restarted.
- `ui/e2e/runtime.spec.ts` / `approvals-nav.spec.ts`: not rerun for this
  cherry-pick; the feature's own upstream record includes its browser smoke.
- The `07150adb` follow-up (foreign-session label) was not applied.
