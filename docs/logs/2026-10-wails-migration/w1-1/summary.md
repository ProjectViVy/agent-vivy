# W1 iteration 1 — Public VIVY Go host and lifetime

Implements story W1 of the DIVA Next Wails migration
(`agent-diva/docs/plans/diva-next/wails/W1.md`, contract `backend-separation-contracts.md` §W3-1):
a native Go embedding surface that replaces the C ABI shape for agent-diva's
future Wails host, without touching the existing C ABI or CLI behavior.

## What changed

- **New public package `sdk/host/v1`** (package `host`): `Open(ctx, Options)`,
  `(h *Host) Call`, `(h *Host) Next`, `(h *Host) Close` with the verbatim
  W3-1 signatures — `Options{ConfigPath, WithoutEars}`, `Notification`,
  `EventBatch`, and the normalized `*host.Error{Kind, Code, Message, Data}`.
  The error kind→code map is the contract's (invalid_input −32602, closed
  −32081, already_initialized −32082, timeout −32083, cancelled −32084,
  transport_lost −32085, internal −32086, incompatible_generation −32087);
  kind `rpc` preserves the upstream code and safe `Data` verbatim.
- **Ownership**: one live Host per process (`already_initialized` otherwise);
  any failed `Open` releases the claim; a timed-out `Close` retains it so the
  runtime is never reopened over live resources. `Close` applies the initial
  5 s desktop budget when the caller context has no deadline and is
  idempotent/concurrent-safe.
- **Sealed generation required**: `Open` rejects a missing or corrupt
  embedded generation manifest with `incompatible_generation` before any
  composition; config path must be absolute and is loaded/validated inside
  VIVY (`config.Load`), returning `invalid_input` for bad input.
- **Call boundary**: encoded request bound at 4 MiB before dispatch; caller
  cancellation is honored; an admission-time timeout reports the deadline
  without claiming a mutation outcome. Trusted dispatch is unchanged — the
  host still dials `app.DialControl`, so the ActionHost caller identity and
  grants (`face/embedded`) apply verbatim.
- **Blocking `Next`**: `internal/embedded` gained a wake channel plus a
  blocking `Next(ctx, limit)` that returns on event, sticky gap, close, or
  ctx cancellation and drains a batch (0 → 500, max 500, queue capacity
  10 000 with oldest-drop + sticky gap; producers never block). `Poll` is
  retained for the C ABI transition. Exactly one reader is admitted at a
  time (enforced in `sdk/host/v1`, `invalid_input` on violation).
- **Bounded context-aware close**: `embedded.CloseContext(ctx)` and
  `app.CloseContext(ctx)` run the existing teardown on one goroutine
  (ordered: cancel callers → peer close → join peer Serve pump → app close;
  app order unchanged: automatic work → action host → channels → lifecycle
  services → worker → WaitIdle → observer → cognitive → MCP → assembly →
  memory → diagnostics → backend). On ctx expiry the error names the
  component still unwinding while teardown continues in the background.
  `Close()` wrappers preserve the previous synchronous semantics for the
  CLI and C ABI.
- **Join primitive**: `controlrpc.Peer.ServeDone()` exposes a channel closed
  when a started `Serve` loop fully unwinds, so the host can join the pump
  instead of racing it.
- **Owned logging once**: `Open` initializes the runtime's redacted,
  daily-rotated file logging under `config.LogDirectory()` exactly once per
  process (`slog.SetDefault`, console sink honors `logging.stdout`).
  CLI/face entry points are untouched and gain no duplicate writers.

## Evidence

- `sdk/host/v1/host_test.go` — boundary tests: sealed open + trusted
  `initialize` call, missing/corrupt generation rejection, invalid input,
  failed-open claim release, second-owner rejection, post-close rejection,
  4 MiB bound, RPC error identity preservation, caller cancellation, limit
  validation, single-reader enforcement, concurrent Call/Next/Close race.
- `sdk/host/v1/consumer_external_test.go` — a temp external module that
  imports only `agent-vivy/sdk/host/v1` and compiles: proof no
  `internal/*` type leaks through the API (the toolchain rejects such
  imports outside the module).
- `internal/embedded/host_events_test.go` — Next mechanics: empty wait,
  first notification, max batch, overflow sticky gap, gap-only wakeup,
  close while waiting, buffered delivery on close, concurrent
  producer/drain with a never-stalling producer, Poll still non-blocking.
- `internal/embedded/host_lifecycle_test.go` + `internal/app/close_context_test.go`
  — deadline-aware close: timeout names the unfinished component, teardown
  completes in the background and later close observes success,
  idempotence, expired-deadline handling.

## Verification

- `go test -race ./sdk/host/v1 ./internal/embedded ./internal/app ./internal/logging` — all green.
- `just ci` — see verification.md.
- External consumer compile — see verification.md.

## Boundaries kept

No product release; VIVY remains independent of Wails; `Poll` and the
C ABI (`cmd/vivy-shared`) are untouched (removal is W6); no internal
config/app/storage/Eino type crosses the API; `FrozenCore`/restart
semantics remain a W5 finding, not declared fixed here.
