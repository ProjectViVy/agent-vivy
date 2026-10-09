# Ordered RPC notifications

The wire already delivers `model.request`, `model.delta`, and `model.completed`
in order. `Peer.dispatch` starts an independent goroutine for each frame with a
method, so callback scheduling can reset the CLI's integrity accumulator after
text arrived. Mutual exclusion in `sdk/facerun` does not restore wire order.

Use one bounded FIFO and one notification worker per Peer. The read loop only
enqueues notifications (requests without IDs), resolves responses directly,
and launches requests with IDs concurrently. Never wait for queue capacity in
the reader: a notification callback can itself wait for a response. Overflow
terminates the connection with an explicit error rather than dropping events
or accumulating unbounded goroutines. Default capacity is 1024 notifications;
`Options.NotificationBuffer` allows transport owners to choose capacity.

The worker receives a peer-lifetime context. Teardown cancels active handlers,
discards queued notifications, and joins the worker before `ServeDone` closes.
Handlers must honor context cancellation; ID-bearing request semantics and
the existing bounded writer are unchanged. Ordering is connection-wide, not
per method or subscription, preserving dependencies between event types.

## Eino capability check

The pinned Eino is `github.com/cloudwego/eino v0.9.13`. This defect is in
Vivy's JSON-RPC transport dispatcher (`Peer.dispatch`), downstream of the
existing model observation and Journal path. No Eino API is imported or
changed: moving wire scheduling into model callbacks would violate the
RPC/domain boundary and fail to fix other notification consumers.

## Acceptance

- A blocked notification cannot be overtaken by later notifications.
- Requests and responses progress while the notification worker is blocked.
- A notification callback can make an RPC call even when notifications arrive
  before its response, including more than the writer's queue capacity.
- Model text and digest settle correctly through the actual RPC transport.
- Overflow and cancellation terminate cleanly without running queued handlers.
