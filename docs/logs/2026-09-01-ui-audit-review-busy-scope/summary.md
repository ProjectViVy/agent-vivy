# Summary

## Topic

UI-AUDIT-REVIEW-BUSY-SCOPE: change the busy lock for Review responses from the
entire queue to individual items.

## Background and audit finding

The 2026-08-31 UI audit confirmed that when `reviewBusyId: string | null` was
non-null, `ApprovalsView` disabled every list row, refresh, and action button—so
while one approval was responding, the user could not even inspect another
record. The store's `respondReview` also serialized every response through a
global early return.

## Changes

- `ui/src/lib/store.ts`: `reviewBusyId: string | null` →
  `reviewBusyIds: string[]`; `respondReview` is single-flight per ID (repeated
  responses for the same ID still early-return), while different IDs can run in
  parallel; `finally` removes by ID. Optimistic state mapping and
  `loadReviews()` are both idempotent and concurrency-safe by ID/full refresh.
- `ui/src/components/approvals/ApprovalsView.tsx`:
  - list rows use `disabled={busyIds.includes(review.id)}`—while one response is
    running, the other rows remain browsable/selectable;
  - detail actions (textarea + approve/reject/answer/cancel) are locked by
    `selectedBusy = busyIds.includes(selected.id)`;
  - the refresh button is gated only by `phase` (it is a read operation and can
    be refreshed at any time).
- `ui/src/routes/_layout.tsx`: the Review-sheet close guard
  (closeDisabled/Escape/outside-click/close interception) now uses
  `reviewBusyIds.length > 0`; semantics are unchanged: the sheet cannot close
  while any response is in flight.

## Explicitly not done

- The backend `review/respond` semantics are unchanged; the UI's natural
  interaction rate limits concurrency.
- `reviewEpoch` loadReviews debouncing remains as-is—each concurrent response's
  `loadReviews()` is an idempotent GET, and the last persisted result is the
  source of truth.
