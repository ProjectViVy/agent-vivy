# 2026-10-17 · Provider agentic spike plan (Route B)

## What changed

Authored a decision-complete **spike plan document set** for lifting
`openai-responses` out of `DEFERRED-INDEFINITE`, by moving the model seam onto
Eino's `AgenticModel` / `schema.AgenticMessage` generation.

**Documentation only — no production code, no dependency, and no configuration
was changed in this delivery.** The spike itself (S0–S4) is specified, not
executed; scheduling it is a separate human decision.

Created:

- `docs/plans/provider-agentic/DESIGN-CONCEPT.md` — the adopted pattern
  ("single generation, sealed binding"), the three verified upstream facts that
  make it viable, the three structural decisions, the rejected hub-and-spoke
  codec alternative with its measured cost, the three verifiable
  "no residue" rules, and the boundary conditions under which the concept is
  false.
- `docs/plans/provider-agentic/README.md` — program plan, ten-decision ledger,
  the `UNSCHEDULED` S0–S4 status board, critical path, standing orders, and the
  relationship to the provider-registry program.
- `docs/plans/provider-agentic/S0-dependency-truth.md` — the runnable method for
  measuring pins, the module-graph delta and binary cost in a throwaway module,
  plus the four questions S0 must answer with numbers.
- `docs/plans/provider-agentic/S1-component-contract.md` — the mandatory Eino
  capability check for the new generation: per-component contract tables with
  module-cache citations, the generational structure of the ADK, the measured
  middleware surface, the eleven-capability checklist, and the explicit
  "what the pinned surface cannot do" list.
- `docs/plans/provider-agentic/S2-local-httptest-loop.md` — the zero-network
  minimal closed loop: three hand-written SSE fakes, six assertion channels,
  four transferred test-discipline rules, and the scratch-location constraint.
- `docs/plans/provider-agentic/S3-migration-inventory.md` — measured scale,
  per-group per-file inventory, the residue-deletion set with a checked consumer
  per item, the preserved list, a twelve-row risk table, and the two migration
  shapes priced.
- `docs/plans/provider-agentic/S4-decision.md` — the decision record: three
  admissible outcomes, the outright-rejected option, thresholds fixed before
  evidence, and the record's required contents.
- `docs/logs/2026-10-17-provider-agentic-plan/{summary,verification,acceptance}.md`
  — this record.

Modified:

- `docs/DEFER.MD` — the `PROVIDER-AGENTIC-MIGRATION` row's status is now
  `DEFERRED · SPIKE PROGRAM AUTHORED 2026-10-17 · SCHEDULING PENDING`, and its
  notes record that the restart condition was met and that its `v0.2.2` version
  evidence is superseded by the new program. Its restart condition is now
  "the owner schedules S0–S4, and S4 returns `GO` or `GO-CONDITIONAL`".
- `docs/plans/provider-registry/EINO-CAPABILITY.md` — a five-line supersession
  banner: the dated 2026-09-18 record is superseded for current-generation facts
  by `docs/plans/provider-agentic/S1-component-contract.md`. Retained as history.
- `docs/plans/provider-registry/README.md` — decision `D6` amended (that ledger's
  own rule requires a decision to be amended, not silently reopened): the lift
  path is being taken, and the new program is named.

## Decisions recorded

The ten decisions are in `docs/plans/provider-agentic/README.md`. The ones that
are genuinely load-bearing:

1. **One message generation, sealed binding table, zero conversion code.** A
   protocol family's differences live only inside a pinned Eino component's
   constructor; Vivy holds data declarations and one `case`.
2. **The adapter table becomes bindings, not states.** `Adapter.State`,
   `CapabilityState` on the adapter path, `ErrAdapterDeferred`, the wire
   `Executable`/`State` fields, and the UI badge path are deleted — they exist to
   describe one non-executable row.
3. **Moving `anthropic-messages` to `agenticclaude` is the one irreversible
   decision**, and S4 must decide it explicitly.
4. **A spike may not add a dependency to this repository's `go.mod`/`go.sum`.**
5. **If S1 or S2 finds a required capability absent, the response is an S4
   finding, never a Vivy-owned workaround.**

## Scope

- **In scope:** the seven plan documents, the iteration log, and the three
  pointer edits above.
- **Out of scope (explicitly not done):** any Go, TypeScript, YAML, JSON Schema,
  `go.mod`/`go.sum`, Docker, or configuration change; executing or scheduling any
  spike stage; adding a dependency; pushing the branch.

## Deliberate non-actions

Two omissions are decisions recorded here so they cannot be mistaken for
oversight:

1. **No new `docs/TODO.md` §0.1 row.** The deferral row in `docs/DEFER.MD`
   already makes the work discoverable; a second entry would be a third copy of
   the same fact.
2. **`docs/TODO.md:117` left untouched.** Its `OPEN` cell duplicates
   `docs/DEFER.MD:105`'s `DEFERRED` cell, but the 2026-09-26 deferral sweep left
   roughly thirty such pairs across the two files. Re-sorting one row would make
   the board less consistent, not more; it is a board-wide condition outside this
   program's scope.

## Corrections made while authoring

Two inherited claims were checked and did not survive, and were corrected in the
plan rather than carried forward:

1. **The 2026-09-18 record's obstacle is inapplicable at the assembly layer.**
   It rejected the migration partly because Eino publishes no
   `Message`↔`AgenticMessage` conversion. That is still true (the only
   `ConvAgentic*` symbols are callback converters), but the ADK is generically
   typed: `adk/chatmodel.go:280` is `Model model.BaseModel[M]` and the untyped
   ADK names are aliases specialised to `*schema.Message` (`:417`, and
   `handler.go:259`). `NewTypedChatModelAgent[*schema.AgenticMessage]` therefore
   accepts an `AgenticModel` with no bridge.
2. **The reasoning-UI citation was wrong.** The claim that
   `ui/src/components/chat/MessageBubble.tsx:167` holds a reasoning disclosure
   that never receives data is false — that file has no reasoning code. The
   surface actually lives in `ui/src/components/chat/ReasoningRow.tsx:23`,
   rendered by `ChatView.tsx:199` from a `model.reasoning_delta` event folded in
   `run-rows.ts:224` and accumulated in `store.ts:412`. It is complete and
   tested; **the gap is on the backend**, which is a more precise and more
   useful statement of the same finding.

## Records

- Plan: `docs/plans/provider-agentic/`
- Backlog: `docs/DEFER.MD` (`PROVIDER-AGENTIC-MIGRATION`)
- Branch: `docs/provider-agentic-plan` (not pushed)
