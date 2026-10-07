# A2A-05 Verification

- `go test ./internal/config ./internal/channelhost -run 'ChannelHTTP|A2AHTTPIsolationAndLifecycle' -count=1 -v`: green — strict config
  validation, constant-time credential compare, principal allowlist,
  stripped Authorization/forwarding headers, private binding injection.
- `go test ./internal/channelhost ./internal/app -run 'A2AHTTPIsolationAndLifecycle|Channel.*Lifecycle' -count=1 -v -race`: green — listener state machine
  (absent/inactive/starting/serving/draining/failed/stopped), bounded
  writer + Flusher, 1 MiB frame bound, public vs authenticated buckets,
  startup-failure reverse-order close, token-rotation revocation.
- `go test ./internal/app -run 'A2A' -count=1`: green — app wiring mounts
  the listener only with the `channel.a2a` grant + task surface; Inspect
  reports compiled/granted/wired/started and effective bounds with
  credentials redacted.
- `just ci`: green except `sdk/codeclient TestClientAgainstRealVivyCode`,
  the pre-existing main regression fixed by open PR #37 (unrelated to
  this branch; reproduced on clean origin/main).
