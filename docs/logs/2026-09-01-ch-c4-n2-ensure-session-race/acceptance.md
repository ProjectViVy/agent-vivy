# Acceptance

How a human can tell CH-C4-N2 is closed:

1. `go test ./internal/channelhost -race -count=5` — the two new tests
   (`TestEnsureSessionConcurrentSameChat`,
   `TestConcurrentInboundSameChatDispatch`) pass repeatedly under the
   race detector.
2. `just ci` stays green with the new tests in the default run.
3. Product face (unchanged by design): sending several Telegram
   messages to a fresh chat in quick succession still yields a single
   `channel/fake/<chat>` session — now proven under concurrent dispatch
   rather than assumed.
