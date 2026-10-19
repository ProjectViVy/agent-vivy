# Approval card auto-open and auto-dismiss

Date: 2026-10-20
Base: `aa030fc7` (`main` after the quick approval card, compact control, and
thinking-level null fix).

## Request

- Open the quick approval card automatically when a pending review appears.
- Close it automatically after the user's decision succeeds.

## Change

`ui/src/components/chat/ChatInput.tsx`:

- A `previousPendingCount` ref plus an effect opens the card when the pending
  count rises from zero (including the initial load) and closes it when the
  count returns to zero.
- A user-closed card is not forced back for the same batch: the effect only
  reacts to a count increase, so manual close stays closed until another
  pending item arrives.
- `onRespond` now awaits `respondReview` and closes the card only after the
  request and the authoritative review refresh succeed. A failed decision keeps
  the card mounted, restores the buttons, and shows the store error in the card,
  matching the async-confirmation lifecycle.

`ui/src/components/chat/ChatInput.approval.test.tsx` (new) covers auto-open on
mount, auto-open when a pending item arrives, close after approval, staying open
with the error after a failed decision, and manual close not reopening.

No store, RPC, or published Face contract changes; the open flag remains local
to the composer.

## Explicitly not done

- No change to the approval center, review sheet, or `/approvals`.
- No auto-open for historical/non-pending reviews.
- The `07150adb` foreign-session label follow-up remains unapplied.
- Full `just ci` was not run; the owner's earlier instruction to stop it still
  applies.
