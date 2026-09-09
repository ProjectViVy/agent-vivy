# TUI-STREAM-N5 — Tightening the defensive contract for durable streams

## Summary

The five P3 carryovers from the 2026-09-04 lifecycle audit were closed out one by one:

1. **`stream.Decode` requires identity** (`sdk/tui/stream/events.go`):
   `subscription_id` and `run_id` are now required; missing either is rejected.
   The durable server always sends both fields; the sole production consumer,
   `sdk/tui/live/events.go`, is unaffected. This prevents out-of-domain or
   partially malformed notices from advancing this face's cursor.
2. **Release terminal subscriptions with empty replay**
   (`internal/rpc/control.go` `streamRun`): when a resubscribe's `after_seq`
   already covers the terminal record, the previous empty replay could hang on
   the bus until peer Close. After the initial replay, the code now checks
   `Runs.GetRun`: a terminal run returns immediately; a nonexistent run also
   returns immediately; an active run or query error keeps the live-wait path.
   Correctness follows the commit-order invariant of `emitTerminal`: the
   terminal event is committed to the Journal before the run state flips, so a
   terminal row implies that the terminal record is replayable.
3. **Remove `Inbox.Take()`** (`sdk/tui/stream/inbox.go`): the public Take,
   which discards the replay fence, is a footgun; production code uses only
   `TakeWithOverflow`/`TakeBatch`, tests use `TakeWithOverflow`, and the API no
   longer exposes an entry point that hides overflow.
4. **REPL `streamLost` run epoch / move to shared Inbox**: absorbed by a later
   refactor—the current `Live` (`sdk/tui/live/controller.go`)
   `recordStreamError` isolates by subscription key and uses
   `retiredSubscriptions` to block stale failures; notices uniformly pass
   through shared Inbox/Projection. No code action remains.
5. **`Live.Close` ownership boundary**: the original design was retained and
   re-verified—`Close` sets `closed`, increments `subscriptionRequest++`, retries
   unsubscribe for 2s, cancels ctx, and closes inbox; a subscribe response
   that was never fully delivered relies on the outer peer Close for cleanup,
   and `enqueueNotice` returns directly after ctx.Done, with no leak path. This
   is a design confirmation, not a defect.

## Explicitly not done

- No change to the JSON-RPC protocol shape (early subscription termination
  appears to the client as stream termination; the durable client uses
  terminal events as the source of truth and does not depend on connection
  lifetime).
- The `streamFailures` map's LRU limit remains 4.

## Filing

- Board: the `docs/TODO.md` §0.1 TUI-STREAM-N5 row → §10 completion log
  (noting that items 4/5 were absorbed by refactoring / confirmed as design).
