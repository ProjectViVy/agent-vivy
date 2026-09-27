# Acceptance — 2026-09-14 channel hardening batch

How a human can tell it worked, in product terms:

1. **A crash no longer eats a channel reply.** Send a message to a
   configured, allowed channel ear and kill the process after the model
   finishes answering but before the reply reaches the platform (hard to
   time by hand — the automated proof is
   `TestRestartRedeliversArmedIntentOfCompletedRun`). On the next start the
   reply arrives in the chat. From the user's view: an answer that used to
   vanish on a bad-timing restart now shows up, at most duplicated once
   (at-least-once is the documented trade).

2. **A dead ear cannot retry forever.** If the platform stays unreachable,
   the delivery intent is parked as `failed` after three attempts (visible
   in the `channel_deliveries` table and the structured log) instead of
   retrying on every boot. The rest of the organism is unaffected.

3. **Shutdown stops dropping replies.** Stop the process while a reply is
   mid-send: `StopAll` now waits (bounded by the 5s shutdown grace) for the
   in-flight Send instead of killing it, and anything that did not finish
   redelivers on the next start.

4. **The provenance ledger stays bounded and truthful.** `channel.inbound`
   provenance events older than 30 days are pruned once per process start
   (log line `channel inbound events pruned rows=N` on the first start
   after 30 days of channel traffic; a warn appears instead if pruning
   fails — startup never blocks on it). Message provenance is now a closed
   vocabulary: only `ui`, `channel`, or `headless` can be stamped onto a
   turn; anything else fails the turn loudly before it is persisted, so a
   typo in a future caller cannot half-label history.

5. **DingTalk survives silent network drops.** With the dingtalk ear
   running, a network path that dies without a close (Wi-Fi roam, NAT
   expiry) recovers by itself: within roughly the ping interval plus one
   redial tick (~30s + 3s in production defaults) the ear reconnects,
   logged as `dingtalk: stream reconnected` after the warned failures.
   Before this change that ear stayed deaf until process restart.

6. **The contract tells the truth.** `VIVY-CHANNEL-PACK.md` §12 now
   describes exactly what the code journals (identifiers-only payload,
   `chanin_` pseudo-runs, 30-day retention, at-least-once outbound) and
   the `ui | channel | headless` source ruling — an auditor reading the
   contract and querying the database get the same story.

## What a human can check quickly

- `just ci` green on `feat/channel-hardening`.
- Boot the organism with any config: startup logs show the five compiled-in
  channels with their usual configured/not-started notes, no new errors,
  and (once events age past 30 days) the prune line.
- `sqlite3 <data>/vivy.db "SELECT run_id, state, attempts FROM
  channel_deliveries"` — empty after healthy operation; rows only exist
  while a reply is armed/pending/failed.
