# Verification — approval card auto-open and auto-dismiss

Environment: Windows, `node v24.19.0`, `pnpm 10.11.0`. Commands ran from the
repository root.

## Focused tests

```text
pnpm --dir ui exec vitest run \
  src/components/chat/ChatInput.approval.test.tsx \
  src/components/approvals/ApprovalQuickCard.test.tsx
→ 2 files, 14 tests passed (5.2s)
```

The new lifecycle suite (5 tests) covers:

- opens automatically for a pending review at mount;
- opens when a pending review arrives after mount;
- closes after `review/respond` succeeds and the authoritative list refreshes;
- stays open with the error when the decision request fails;
- does not force the card back after the user closes it.

## Full UI gates

```text
pnpm --dir ui test
→ 82 files, 643 tests passed (27.6s)

pnpm --dir ui typecheck
→ tsc --noEmit, exit 0

pnpm --dir ui build
→ vite v7.3.6, 2583 modules; dist/assets/index-CPMBqVKW.js (1,846.15 kB)
```

## Real split-path smoke

The split pair was already running at `:3015` / backend `:8787` with the
isolated dev home. To exercise the unchanged real `/rpc` review path without a
provider key, a temporary pending `questions` row was inserted into the
isolated dev database, then removed after the check:

- loading `http://127.0.0.1:3015` rendered `#approval-quick-card` without
  clicking the shield (`AUTO_CARD_COUNT 1`);
- the card showed `用户问题` / the seeded prompt / `审批中心`;
- the shield reported `aria-expanded="true"`;
- no browser console or page errors were recorded;
- the temporary row and smoke script were deleted afterward.

The approve/deny close path was covered by the component lifecycle test rather
than by faking a durable approval: `review/list` deliberately hides pending
approvals whose owning run is not still durable, so a seeded approval row would
not be a faithful live test.

## Not run

- `just ci`: stopped by the owner earlier and not restarted.
- Full Playwright e2e; the live split-path auto-open check is above.
