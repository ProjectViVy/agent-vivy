# Composer quick-approval card

Date: 2026-10-17
Theme: replace the composer's review-center jump with a small movable approval
popup that shows the gated command and offers Approve/Deny in place, while the
approval center stays available as a second decision surface.

## What changed

### The shield button opens a quick card, not the center

`ui/src/components/chat/ChatInput.tsx` no longer opens the review-center Sheet
from the composer shield. It toggles a new `ApprovalQuickCard` anchored to the
composer: `absolute right-0 bottom-full mb-3`, right-aligned above the input
row. The card takes no layout space and never overlaps the textarea or the
composer toolbar (measured in the real browser: card bottom 625 px, composer top
636 px, textarea top 679 px at 1280×800).

`ui/src/components/approvals/ApprovalQuickCard.tsx` owns the presentation:

- header: title, remaining count, close button, and a `GripVertical` drag handle
  (`cursor-grab touch-none select-none`, pointer-capture drag) with `Escape`
  closing the card;
- body: `approvals.quick.command` plus the pending `preview` verbatim in a
  bounded `<pre>` (`max-h-40 overflow-auto whitespace-pre-wrap break-words
  font-mono text-xs`), the first risk finding, and the store's `reviewsError` as
  an inline `role="alert"`;
- footer: Approve / Deny for `kind === 'approval'`, plus a ghost button that
  opens the existing approval center.
- `clampOffset` keeps the dragged card at least 8 px inside the viewport and a
  resize listener re-clamps it; questions (`kind === 'question'`) get no blind
  Approve/Deny and route to the center instead.

`pickPendingReview(reviews, activeSessionId)` selects the oldest pending review
of the active session, falling back to the oldest pending review overall when
that session has none, so the card always decides the entry the composer's run
is waiting on rather than an arbitrary one.

### One decision path, three surfaces

The card calls the same `respondReview(id, { action })` the approval center
uses. Per-id `reviewBusyIds` guards double submits; the store reloads
`reviews` after every response, so the badge, the card, the in-chat center Sheet
and the `/approvals` page render the same record. Nothing new was added to the
store or the SDK face contract.

### Copy

New `approvals.quick.*` keys in `ui/src/i18n/{en,zh}.ts` (title, command,
remaining, empty, openCenter, close, drag, questionHint), classified as
face-specific web keys in `scripts/i18n-cross-face-contract.json`.
`chatInput.reviewCenter` was retitled to "Quick approval / 快速审批" because the
button no longer opens the center; `layout.reviewCenter` (the Sheet title) is
unchanged.

## Deliberate deviation from the plan

The approved plan put the open flag and drag offset in the store. They are
component-local `useState` in `ChatInput` instead: every `useVivyStore` field is
also part of the published `FaceStoreState` contract in `sdk/ui/src/module.ts`,
and `ui/src/lib/ui-sdk-face-compat.test.ts` / `ui-build-provenance.test.ts`
fail closed on any extra key. A transient cosmetic offset does not justify
shipping a new published SDK field plus its fixture updates, so the placement
survives close/reopen within one composer mount and resets on session switch
(`ChatInput` is keyed `composer-${sessionId}`) or route change.

## Explicitly not done

- No autofocus, focus trap, or outside-click dismissal: the card is a floating
  status surface, not a modal, and stealing focus from the composer would break
  typing while a run is blocked.
- No auto-open: the card appears only when the user clicks the shield, per the
  requested behavior. The badge still signals a pending decision.
- No change to `ApprovalsView`, `ReviewCard`, `_layout.tsx` (the Sheet still
  keys on `reviewCenterOpen`), the review RPCs, or the approval policy.
- No new store field, no backend change, no second source of truth: the card is
  a projection of the existing `reviews` state.
