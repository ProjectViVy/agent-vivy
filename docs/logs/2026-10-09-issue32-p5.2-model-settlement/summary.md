# P5.2 mandatory model settlement

## Outcome

Implemented R4 on `feat/issue32-remediation`. Code commit: `900ca7db` (`fix(runtime): propagate mandatory model settlement failures`).

The existing `Service.persistAndPublish` path now delegates to an error-returning
`persistAndPublishErr` implementation. It retains the projection lock,
publication ordering, and classified best-effort run terminal behavior while
returning the original Journal or deletion cause to mandatory model settlement.
`runModelCallObserver.End` wraps that cause in `modelSettlementError`.

Successful Generate calls now fail when their mandatory `model.call.finished`
append fails. Generate and stream failures preserve both the provider/chunk
cause and the settlement cause. Stream EOF settles before closing the consumer
pipe; stream failure settles before forwarding the joined error. The existing
retry seam recognizes settlement failures and refuses to replay a provider
call, including when the joined provider error looks like context overflow.

No second Journal writer or model loop was introduced. The producer still uses
Eino v0.9.13 and `Pipe(8)`; downstream closure is covered through deterministic
channels, with End once and upstream close observed.

## Evidence

- Real observer fault injection covers final usage and `model.call.finished`
  appends, original-cause unwrapping, one failed terminal, and quota exemptions.
- A real Generate wrapper returns the storage sentinel and does not return a
  successful message when `model.call.finished` fails.
- The real overflow path makes one provider call, emits no retry-start event,
  and ends `run.failed` after settlement failure.
- Main, child, and summary source routes are covered. A canceled caller still
  settles via the detached bounded persistence context.
- Runtime focused, race, and full package checks passed; details are in
  [verification](verification.md).

Aggregate `just ci`, SDK/consumer conformance, and P5.1 native Windows execution
remain P7 integration gates. A permanently unavailable Journal may also reject
the best-effort terminal append; no finish record is fabricated in that case.
