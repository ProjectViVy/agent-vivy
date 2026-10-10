# PR #46 Dependency and Durability Repairs

> **For agentic workers:** Use Superpowers systematic debugging and test-driven development. Independent problem domains run in isolated worktrees; shared queue/admission changes remain one lane.

**Goal:** Resolve the six reported failures without adding a queue database, scheduler, or background warming service.

**Architecture:** Preserve Service.Run/RunWithOptions, transactional admission, Journal authority, and generated Assembly. Extract optional module metadata into implementation-free packages consumed by both catalog and owners. Persist before publishing queue state or starting execution.

**Tech Stack:** Go 1.26.4, Eino v0.9.13, existing SQLite/PostgreSQL admission, JSON-RPC, TypeScript faces, SDK pack.

**Spec:** User's M1–M6 report; AGENTS.md and docs/architecture/VIVY-{MODULE-STANDARD,PORT-CATALOG,PLUGIN-SPEC,ASSEMBLY}.md.

## Constraints and review focus

- One authoritative dependency closure: full Laputa SHA, three Go module revisions, bootstrap verification, and refreshed source-bound evidence.
- Fail closed on replay and persistence errors; retain pending work on failed admission.
- Preserve ordered enqueue/upsert, dequeue/remove, and re-enqueue/reactivate transitions.
- Merge only compatible queued options; preserve attachments, context, thinking, and mode through RPC and faces.
- Resume with the original run context and a fresh Eino cancellation handle.
- Return settlement failures before stream EOF; keep bounded producer backpressure.
- Accept log growth; detect replacement and shrinking files with stable identity and offsets.
- Account maintenance calls; report unknown cost honestly; warming is opt-in pending measured same-prefix net benefit.

## Eino capability check

Use pinned adk.WithCancel, cancellation modes, Runner.Resume and ChatModelAgentResumeData for steering; schema.Pipe and existing observed model producer barrier for settlement. Warming uses existing model.Generate and run observer/usage projection. No parallel runtime or custom model transport.

## Task 1 — M1 dependency closure and optional catalog

- [x] Reproduce old-pin compilation and add bootstrap/Go revision consistency and optional-import regressions.
- [x] Review upstream ff3936f..4b2bec2 required API changes; pin exact reviewed full SHA and matching pseudo-versions.
- [x] Extract cognitive/memory/mask identities and action metadata without importing optional owners; preserve one definition per identifier.
- [x] Refresh source-bound conformance evidence after integrated source changes; verify exact-pin clean sibling, headless/embedded compilation, minimal/default pack and inspect.

## Task 2 — M2/M3 queue and cancellation (one lane)

- [x] Add regressions for replay failure/retry/concurrency, FIFO/reactivation, persistence failures, fast terminal/tail race, busy options, context cancellation and repeated steering.
- [x] Extend existing admission transaction to include dequeue/tail events before driving; publish only complete replay; return errors from mutations.
- [x] Carry full existing DTO, drain through RunWithOptions, batch contiguous compatible options, and replace face-local queues with kernel authority.
- [x] Atomically install a new Eino cancel handle for each resumed phase using the owning run context.
- [x] Run focused runtime/storage/RPC/face tests and record results.

## Task 3 — M4 model settlement

- [x] Add failing Generate/setup/chunk/EOF/error and downstream-close settlement tests.
- [x] Join provider and settlement errors; send normal EOF settlement failure through the bounded pipe before closure.
- [x] Run observer regressions and existing producer-barrier tests.

## Task 4 — M5 diagnostics cursors

- [x] Add failing append and truncation-above-offset regressions plus replacement coverage.
- [x] Validate cursor using stable opened-file identity, prior size, and offset, preserving bounded reads.
- [x] Run diagnostics and cross-platform compilation checks.

## Task 5 — M6 cache warming

- [x] Add failing maintenance accounting, default config, actual reusable-prefix and lifecycle tests.
- [x] Opt in to streaming warming; remove ineffective idle mode; retain existing coalescing/cancellation and mandatory observation.
- [x] Record actual warm usage, prefix and available pricing evidence; remove unsupported profitability estimates and represent unknown cache-write pricing honestly.
- [x] Run focused warming/config/usage tests.

## Task 6 — integration

- [x] Review isolated changes, integrate focused commits, refresh evidence once for final internal source identity.
- [x] Run just ci, race checks for queue/observer, and real pack/inspect paths; final complete CI and source-bound producer gate PASS at `a44bb3f9`.
- [x] Record summary, verification, acceptance and limitations; commit focused deliverables without pushing.

### Integration follow-up — recipe-accurate recall gate

The first full Go gate exposed six existing recall tests using only cognitive-factory presence as their guard. The unchanged default recipe/assembly deliberately omits `vivy/diva-memory`; the DIVA recipe selects it. M1–M6 did not remove that provider. Repair the guard against actual recalled-source inventory and add a required CI execution of all six tests under the official SDK-generated DIVA recipe overlay. Preserve recipe ownership and generated Assembly; add no hand-wired production providers. Reuse the generator with an optional recipe parameter and remove temporary overlay inputs after each gate.

- [x] Reproduce default composition mismatch and verify the exact original guard defect.
- [x] Add capability-accurate recall test guards and required generated DIVA CI gate.
- [x] Execute the six original assertions under the actual DIVA generation (all PASS, zero skips), refresh final evidence, and complete required integrated gates.

### Integration follow-up — full restoration marker boundary

- [x] Reproduce the extra 8 encoded bytes when an acknowledged steer later demotes to follow_up.
- [x] Reserve the larger future full restoration marker before enqueue ACK while persisting the actual steer track; reject oversize without mutation.
- [x] Verify both sides of the boundary and a real missing-checkpoint fallback, focused race, independent read-only review, final source resealing, clean packing and complete CI. See `docs/logs/2026-10-10-queue-payload-boundary/` and the final integration record.
