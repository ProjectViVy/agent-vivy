# Acceptance

- A human cannot see this directly: it is a lifecycle-correctness fix.
- Tell: stop a starting feishu ear (Stop while the gateway handshake is
  unanswered) and Start returns promptly with a connect error instead of
  hanging until the process context ends. The regression test pins it:
  `go test -race -run TestStopDuringFirstConnectReturns ./plugins/feishu`
  (from the plugin module) fails with "Start did not return after Stop"
  on the old code.
