# W1-1 acceptance

From the owner's seat, W1 is acceptable when:

1. `sdk/host/v1` exposes exactly the W3-1 surface — `Open`, `Call`,
   `Next`, `Close`, `Options`, `Notification`, `EventBatch`, `Error` — and
   compiles from a module outside agent-vivy (see
   `consumer_external_test.go`).
2. Only a sealed Generation opens: missing/corrupt embedded manifests are
   startup errors (`incompatible_generation`), and exactly one live Host
   may own a process (`already_initialized` for a second `Open`).
3. Trusted dispatch is unchanged: `initialize` over the in-process pipe
   still returns `protocol_version: vivy.rpc.v1` with the embedded
   ActionHost caller identity; RPC errors keep their numeric code and
   safe Data under kind `rpc`.
4. Events arrive without polling: `Next` blocks and returns batches
   (limit 0 → 500, max 500), reports a sticky gap after overflow, wakes on
   gap-only and close, and admits exactly one reader.
5. Close is bounded and honest: within the 5 s desktop budget it either
   finishes or names the component still unwinding; a timed-out close
   keeps the ownership claim; later closes observe completion;
   `Close`/`CloseContext` are idempotent and concurrent-safe.
6. Runtime diagnostics exist in embedded mode: the rotated redacted
   `vivy.log.*` file appears under the config log dir and the open
   milestone is recorded (asserted in `TestOpenSealedAndTrustedCall`).
7. Gates: `go test -race ./sdk/host/v1 ./internal/embedded ./internal/app
   ./internal/logging` green and `just ci` green.

Deferred by contract: W2 (modfile/packaging), W5 (FrozenCore/restart
verification), W6 (C ABI removal). Nothing here declares those findings
fixed.
