# PR #36 review fixes

## Outcome

The PR now preserves the channel plugin boundary while closing four correctness
gaps found during review:

1. Outbound media capability discovery now unwraps the generated Provider
   target instead of type-asserting the host wrapper.
2. Durable-delivery recovery keeps rows untouched when a plugin is missing,
   disabled, or failed to start.
3. Channel ingress durably arms and registers its delivery target before the
   runtime may publish a terminal event.
4. Session deletion removes channel-delivery rows in both SQLite and
   PostgreSQL.

The branch was merged with current `main` without dropping the workspace
changes. SQLite migration 23 remains workspace and 24 is channel delivery;
PostgreSQL schema 21 remains workspace and 22 is channel delivery. Conformance
IDs remain monotonic through CN-31.

## Plugin architecture record

- No new Module, source pin, Port, Grant, or Recipe constraint was introduced.
- Existing Port: `std/channel@v1`; public channel Providers remain T2 sources
  selected by the generated Assembly.
- Sole Host Consumer: `internal/channelhost`. The kernel remains the authority
  for sessions, Journal provenance, runtime entry, and delivery durability.
  Cardinality stays many channel Providers to one Host.
- Provider lifecycle is unchanged. Unavailable Providers remain inactive;
  recovery parks their durable work for a later healthy start.
- Definition, SDK ABI, Provider descriptors, Inspect projection, and generated
  Assembly wiring are unchanged. Host, storage, conformance, and app wiring
  tests carry the new evidence.
- Implementation plan:
  `docs/superpowers/plans/2026-09-15-pr36-review-fixes.md`.
- Eino capability check: not applicable. This change sequences Vivy-owned
  delivery-ledger state around the existing `Service.RunWithOptions` entry;
  it adds no model, agent-loop, tool-orchestration, or Eino adapter capability.

## More, Fast, Good, Frugal

The production path adds one synchronous callback before runtime persistence.
It introduces no polling, worker, queue, dependency, plugin ABI expansion, or
second run path. The legacy injected `RunFunc` remains for tests and embedders;
the app uses the race-free prepared seam.

The browser gate also raises the Playwright baseline server startup budget from
30 to 120 seconds. Two Windows runs showed the selected packed server healthy
while the separate `go run` baseline missed the fixed 30-second cold-start
budget.

## Explicitly not done

- No PR merge was performed.
- No public channel contract or generated Assembly file was hand-edited.
- No platform-specific plugin behavior was broadened.
- The reply-content query shape was not redesigned; it is a separate
  performance concern, not required for these correctness fixes.
