# Acceptance evidence

Deterministic tests use disposable SQLite stores and in-process Eino model doubles
through real Service admission, Journal, checkpoints and usage projection.

- Exhausted shared model-call budget admits no paid provider call or request.
- Admitted refresh records one maintenance request, paid usage settlement and
  finish, including completion tokens; no transcript event or foreground credit.
- Exhausted diagnostic quota cannot drop paid settlement or finish.
- Request/usage/finish append failures fail the existing Run and wake blocked
  foreground work. Failed admission makes no paid call; failed usage persistence
  still attempts finish. The Run has exactly one terminal.
- Cancellation keeps WaitIdle blocked until paid Generate unwinds and settles.
  Returned usage remains accounted for on both provider failure and cancellation.
- Steering cancels and drains the old refresh before resuming the same Run with
  a fresh warmer; cancellation and WaitIdle include its replacement leg.
- Concurrent maintenance projection keeps both calls active until their own
  finishes, preserves independent usage/call IDs, and retains foreground text.
- Existing off/streaming/idle, savings, timeout, observer, budget, queue, steering,
  cancellation and approval checks pass in the focused selection.

These establish owner behavior, not provider cache hits, latency or net savings.
Required independent Linux integration gates and human real-model acceptance are
not claimed by this lane.
