# Acceptance

The CLI must no longer fail a correct model stream because `model.request`
resets its accumulator after a later text delta. Existing byte-length and
SHA-256 validation remain enabled.

Run from the repository root:

```sh
go test ./sdk/facerun -run TestRunVerifiesModelIntegrityThroughOrderedRPCNotifications -count=1
go test ./internal/rpc -run 'TestPeerNotificationsDoNotOvertakeBlockedHandler|TestPeerNotificationHandlerCanCallRPCThroughBurst|TestPeerCloseCancelsNotificationAndDiscardsQueue|TestPeerNotificationOverflowClosesWithoutBlockingReader' -count=1
GIT_CONFIG_GLOBAL=/dev/null just ci
```

The first test uses real JSONL peers and the production face runner, not a
direct `onEvent` call. While the first request callback is held, later deltas,
completions, and terminal events remain queued in wire order; a later
ID-bearing request still responds. Releasing the callback produces two exact
UTF-8 messages, successful digest checks, completed status, and no diagnostics.

The RPC tests also prove that a notification callback can make a nested RPC
call, receive a response after a 256-notification burst, and then drain that
burst in order. Close cancels an active callback and discards queued work.
Queue saturation returns an explicit error and closes the peer rather than
letting later events overtake earlier ones.

Normal delivery preserves connection-wide notification order. Shutdown is
abortive, not a promise to deliver pending notifications; callbacks must honor
cancellation. A peer with more than its configured notification capacity
waiting behind one callback is disconnected. Requests with IDs intentionally
retain concurrent execution.
