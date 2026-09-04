# Acceptance

1. Feed reasoning chunks `这`, `是一句`, and `话` before provider EOF. Each
   chunk becomes durable and observable immediately, and the shared TUI shows
   one continuous `这是一句话` reasoning block.
2. Send substantially more than 64 run notifications through a peer with a
   bounded writer queue. Every notification arrives once; capacity pressure
   waits or cancels instead of silently terminating the subscription.
3. Feed one answer/reasoning chunk larger than the event payload target.
   Reassembling its emitted deltas reproduces every CJK and emoji rune.
4. Render long Chinese, Latin words, repeated spaces, blank lines, and a ZWJ
   emoji. Wrapping respects terminal cell width without inserting spaces or
   splitting the emoji cluster. ANSI/control input cannot move the cursor.
5. Cancellation still unblocks a backpressured durable notification, and
   ordinary non-durable `Notify` still returns `ErrOverloaded` when full.
6. Close a peer while its subscribed run is idle. The stream goroutine, Bus
   subscription, and control-plane subscription entry are released without
   waiting for another event.
7. Resume an approval/question run whose provider remains open after its first
   answer chunk. That chunk is durable before EOF, exactly once, and cannot
   appear before the preceding `tool.finished` event.
8. Close a peer or cancel its parent context before the subscribe response can
   enter a full queue. Both the subscription table entry and deferred
   after-response callback are removed.
