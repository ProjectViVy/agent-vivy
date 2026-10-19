# Approval quick-card merge

Date: 2026-10-20
Base: `0a8b8ad2` (`main`: clean agentic-provider line plus the welcome wizard,
thinking-level null fix, and context-meter compact control).

## Source

The feature lived on the old pre-split line:

- local and `origin/feat/approval-quick-card` both at
  `0f2c4658 feat(ui): open a quick approval card above the composer`
- checkout: `C:\Users\com01\Desktop\agent-vivy-approval-quick-card`
- upstream donor snapshot: `origin/remake/chat-ui` contains
  `ui/src/components/approvals/ApprovalQuickCard.tsx`; full old history is in
  `origin/remake/archive-20261019`.

The branch is based on the old line, so it was cherry-picked onto the clean
`main` as `7f77082b` rather than merging the donor history.

## What changed

The composer shield no longer jumps straight into the review-center sheet. It
toggles a compact card anchored above the input row (right aligned, no layout
space) that names the gated command, shows the remaining count and the first
risk note, and offers Approve/Deny plus a link to the approval center.

The card calls the same `respondReview` path as the center, so the badge, the
card, the in-chat sheet and `/approvals` read the same review records. The open
flag and drag offset stay component-local to the composer; nothing new enters
the store or the published Face contract.

Files:

- `ui/src/components/approvals/ApprovalQuickCard.tsx` (new)
- `ui/src/components/approvals/ApprovalQuickCard.test.tsx` (new)
- `ui/src/components/chat/ChatInput.tsx`
- `ui/src/i18n/{en,zh}.ts` (`approvals.quick.*`)
- `ui/src/i18n/surfaces.test.tsx`
- `scripts/i18n-cross-face-contract.json`
- `docs/logs/2026-10-17-approval-quick-card/` (upstream record)

## Conflict resolved

`ChatInput.tsx` conflicted in the block that derives `pendingReviews`,
`pendingReviewCount`, `quickReview` and `currentDraft`. The clean base had the
context-meter compact control in the same component but no quick-card state;
the resolution keeps both: the upstream quick-card memo/ref block is inserted
after `activeSessionId`, while the compact-control state and meter remain
intact.

## Explicitly not done

- The follow-up `07150adb fix(ui): keep composer thinking label and payload
  consistent; name foreign-session approvals` is **not** on
  `feat/approval-quick-card` and was not included. It remains available on the
  old line if the foreign-session label is wanted.
- No real pending-approval browser flow was available without a provider key;
  the live smoke covered the trigger and empty state, component tests cover
  Approve/Deny.
- Full `just ci` was not run; the owner's earlier instruction to stop it still
  applies.
