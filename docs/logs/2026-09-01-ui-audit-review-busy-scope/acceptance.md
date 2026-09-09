# Acceptance

## How to verify

1. Open `http://127.0.0.1:3015` and create at least two pending approvals or
   questions (or use existing records).
2. Open the Review sheet from the shield entry on the chat page.
3. Click "Approve" for record A: row A becomes translucent and unavailable, and
   its button shows Processing…; at the same time:
   - the other rows are no longer gray-locked—clicking record B immediately
     shows B's details (previously the entire queue froze);
   - the top "Refresh" button remains clickable during the response (disabled
     only while loading/refreshing).
4. Select B while A is responding: B's action buttons are disabled (preventing
   a silent no-op); once A finishes, B's buttons unlock and can be used to
   approve or reject immediately.
5. The Review sheet cannot be closed with Esc, by clicking outside, or with the
   close button while a response is in flight (the existing protection remains);
   it becomes closable again when the queue is clear.
6. No bilingual regression: copy such as Processing… remains unchanged.

## Concurrency semantics

- Repeated submissions for the same record remain blocked (per-ID single-flight
  in the store + button disabling as a second safeguard).
- Different records can respond concurrently (independent RPCs + independent
  per-ID optimistic state updates).
