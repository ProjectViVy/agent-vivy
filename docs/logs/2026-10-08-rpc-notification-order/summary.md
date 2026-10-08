# Ordered RPC notifications

## Change

`internal/rpc.Peer` now executes notifications in connection-wide wire order
through one bounded FIFO and one worker. ID-bearing requests remain concurrent;
the transport read loop still resolves responses directly. A notification
callback can therefore call RPC while later notifications wait behind it.

`Options.NotificationBuffer` defaults to 1024. Saturation returns
`ErrNotificationOverloaded` from `Serve` and closes the connection rather than
silently dropping a notification, blocking response dispatch, or growing
unbounded goroutines. Teardown cancels the worker context, joins the worker,
and discards queued payloads before closing `ServeDone`. Handlers must return
when their context is cancelled.

Regression coverage includes exact burst ordering, blocked cross-method
notifications, nested bidirectional calls, close, overflow, and option defaults.
A JSONL-backed `facerun.Run` test holds the first `model.request` callback while
the reader receives two complete UTF-8 model streams and a later RPC request.
Both completions retain their byte lengths and SHA-256 checks, and the run
settles with the exact text and no protocol diagnostics.

The existing outbound-backpressure test now checks the deadline error and
unchanged full queue instead of a wall-clock duration measured after timer
creation; the latter produced a false failure under race instrumentation.

The five internal Provider conformance source pins were refreshed to the live
canonical internal-tree digest. Their executable reproduction gate passed.
`go generate ./internal/generated/assembly` produced no wiring changes.

## Scope

No production `facerun` logic, Journal format, integrity algorithm, Eino code,
request dispatch policy, dependencies, UI, authentication, or grants changed.
This is a local source delivery, not a release or a pushed PR.

## Review

Reviewed notification reentrancy, cancellation, overflow, response resolution,
and queue ownership against the direct call sites. Queue draining belongs to
the unwound read loop, after joining the worker, so a concurrent close cannot
leave one final in-flight enqueue retained after `ServeDone`.

The pinned Eino v0.9.13 is not the owner of this transport scheduler. Moving
ordering into its model callbacks would leave other RPC consumers exposed and
cross the established RPC/runtime boundary.
