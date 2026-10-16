# Provider Agentic Program — Spike Plan (Route B)

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to execute one spike stage at a time.

**Goal:** Introduce `openai-responses` as a fully executable protocol by moving
the model seam onto Eino's `AgenticModel` / `schema.AgenticMessage` generation,
so that all three protocol families live on one message generation and the
`DEFERRED-INDEFINITE` machinery that exists to describe one non-executable row is
deleted rather than kept.

**Architecture:** One message generation, one sealed binding table, zero
conversion code. A protocol family's differences may exist only inside the
constructor of a pinned Eino component; Vivy holds data declarations and one
`case`. See `DESIGN-CONCEPT.md`.

**Tech Stack (target):** Go 1.26.4, `eino v0.9.21`,
`eino-ext/components/model/agenticopenai v0.2.4`,
`eino-ext/components/model/agenticclaude v0.1.7`, generated Assembly wiring,
`just ci`.

**Spec:** this directory — `DESIGN-CONCEPT.md`, `S0-dependency-truth.md` …
`S4-decision.md`.

**Provenance:** designed in one owner session on 2026-10-17, continuing the
`PROVIDER-AGENTIC-MIGRATION` commitment recorded in
`docs/plans/provider-registry/EINO-CAPABILITY.md` §5 (2026-09-18). It is
documentation-only until a human schedules the spike.

---

## Program status

| Stage | Deliverable | State | Depends on |
|---|---|---|---|
| **S0** | Dependency truth: pins, module-graph delta, binary cost | `UNSCHEDULED` | — |
| **S1** | Component contract: the Eino capability check for the new generation | `UNSCHEDULED` | S0 |
| **S2** | Local `httptest` minimal closed loop, zero network | `UNSCHEDULED` | S1 |
| **S3** | Per-file migration inventory, residue-deletion set, risk table | `UNSCHEDULED` | S2 |
| **S4** | Decision record: `GO` / `GO-CONDITIONAL` / `NO-GO` | `UNSCHEDULED` | S0–S3 |

A human changes an `UNSCHEDULED` stage to scheduled. The stages are a **spike**:
they may produce throwaway code inside a lane worktree, and they may end in
`NO-GO`. They may not change production code; the migration is a separate
program that S3 and S4 define.

## Critical path

```text
S0 dependency truth (measure, do not argue)
  -> S1 component contract (every required capability has a cited API)
  -> S2 local httptest loop (text / tool-call / reasoning / usage, all three protocols)
  -> S3 migration inventory + deletion set + risk table
  -> S4 decision
```

---

## The one iron rule

> A protocol family's differences may exist only inside the constructor of a
> pinned Eino component. Vivy holds data declarations and one `case`, and no
> other protocol-specific code.

Any cross-generation or cross-protocol conversion code is forbidden: it is the
mechanism that creates residue. `DESIGN-CONCEPT.md` §2 explains why this is
achievable now and §6 lists the conditions under which it is false.

## Standing orders

Adapted from `docs/plans/provider-registry/README.md:32-50`, plus this program's
own.

- **Eino-native first.** For any provider/model capability, inspect and cite the
  pinned Eino/EinoExt API. If it is absent, mark the capability
  `DEFERRED-INDEFINITE` and add no custom substitute (root `AGENTS.md`). A
  Vivy-owned protocol converter is a custom substitute.
- **Import quarantine.** Only `internal/runtime/` and `internal/provider/` may
  import `github.com/cloudwego/eino*`
  (`internal/app/importlint_test.go:86-102`).
- **One message generation.** After the migration, `schema.Message` does not
  appear on the run path. Mixing generations inside one agent is impossible —
  Eino publishes no `*schema.Message` ↔ `*schema.AgenticMessage` conversion
  (`S1-component-contract.md`).
- **One data write point.** All provider configuration data stays in
  `internal/provider/data/`. A spike never edits it.
- **Data supplies data, never capability.** An embedded vendor entry cannot
  introduce an adapter, a transport, or executable behaviour.
- **No dependency change in this repository from a spike.** S0 measures inside a
  throwaway module under `%TEMP%`; this repository's `go.mod`/`go.sum` are
  untouched.
- **No spike scratch under `internal/`.** An untracked file there rewrites the
  `internal` source digest and breaks the sealed-Generation gate
  (`docs/TODO.md:116` `PROVIDER-PROFILE-DIGEST-PIN`).
- **Never hand-edit `internal/generated/assembly/zz_default.go`.**
- **Secrets never leave the environment boundary.** No key value appears in a
  fixture, a plan, a log, or an event payload (D-010).
- **Human scheduling only.** A human changes an `UNSCHEDULED` stage to
  scheduled, and a human signs S4.
- **One deliverable per commit**, with its iteration log under `docs/logs/` and
  its stated gate.
- **No push** without explicit owner authorization.

---

## Decision ledger

Closed by the owner session that produced this plan. A stage may not reopen a
decision; if implementation proves one wrong, stop and amend this ledger first.

| # | Decision |
|---|---|
| **D1** | The model seam moves to `model.AgenticModel` (`BaseModel[*schema.AgenticMessage]`). Exactly one message generation exists on the run path afterwards. |
| **D2** | The adapter table becomes **bindings, not states**: a row is (family name, data declarations, pinned Eino constructor). `CapabilityState` on the adapter path is deleted. |
| **D3** | `openai-responses` enters as the third executable binding, via `agenticopenai.NewResponsesModel`. There is no fourth adapter and no reasoning-only adapter. |
| **D4** | `anthropic-messages` moves to `agenticclaude.New`. This is the one irreversible decision in the program; S4 decides it explicitly. |
| **D5** | `sdk/port/support.go`'s generic 5-state port support and `sdk/port/providerprofile.Profile` are **untouched**. They belong to the plugin-platform axis, not the provider path; the public Profile gains no capability field. |
| **D6** | No hub, no codec, no bridge, no residue/snapshot namespace, no capability matrix, and no new port. The pinned components perform all protocol conversion. |
| **D7** | Whether the titler models stay on the older generation is decided by evidence in S3, not here. It is a real question because titling is text-only. |
| **D8** | A spike may not add a dependency to this repository's `go.mod`/`go.sum`. |
| **D9** | If S1 or S2 finds a required capability absent, the response is an S4 finding, never a Vivy-owned workaround. |
| **D10** | If a two-generation interval is not representable in the seam's single return type, the migration is all-or-nothing; S3 must say so outright rather than leave it open. |

---

## Relationship to the provider-registry program

This program **reuses** that program's model and does not change its status
board:

- The four-layer model (Adapter / Vendor / Endpoint / Model), the adapter names
  (`openai-completions`, `openai-responses`, `anthropic-messages`), the single
  data write point `internal/provider/data/`, the bidirectional consistency gate,
  and `D15`'s fail-closed rule all stay as they are.
- It **takes up** that program's `D6`: `openai-responses` was shipped
  `DEFERRED-INDEFINITE` with "an explicit owner commitment to migrate onto
  `AgenticModel` as its only lift path". This program is that lift path.
  `docs/plans/provider-registry/README.md` `D6` records the amendment.
- It **supersedes for current-generation facts** that program's
  `EINO-CAPABILITY.md`, which cites `eino v0.9.13` / `agenticopenai v0.2.2` and
  never considered the Anthropic side. The dated document is retained with a
  supersession banner; the live capability check is
  `S1-component-contract.md`.
- The Port evidence anchors `sdk/internal/assembly/evidence.go:77-80` and
  `sdk/internal/conformance/reproduction_test.go:476` cite
  `internal/modules/defaults/providers.go#ProviderProfiles` and
  `TestDefaultProviderProfilesMatchExistingRuntimeFamilies`. **This program does
  not rename them**, and the adapter set stays three, so
  `sdk/internal/testdata/default-generation.expected.json:168-172` is not
  edited.

### Backlog bookkeeping (and one deliberate non-action)

The deferred row `PROVIDER-AGENTIC-MIGRATION` (`docs/DEFER.MD:105`) is the single
backlog entry for this work; its restart condition ("the owner explicitly
re-prioritizes") is now met and the row's status carries a pointer to this
program.

Two things were deliberately **not** done, and are recorded here so the omission
is a decision rather than an oversight:

1. No new row was added to `docs/TODO.md` §0.1. Adding one would create a third
   copy of the same fact. The deferral row already makes the work discoverable.
2. `docs/TODO.md:117` was left untouched. Its `OPEN` cell duplicates
   `docs/DEFER.MD:105`'s `DEFERRED` cell, but the 2026-09-26 deferral sweep left
   roughly thirty rows in both files; that is a board-wide, pre-existing
   condition outside this program's scope, and re-sorting one row of it would
   make the board less consistent, not more.

---

## Expected net effect

**Deleted** (itemised with a consumer check in `S3-migration-inventory.md`):
the adapter capability state and its error, the deferred branch in adapter
resolution, the `DEFERRED-INDEFINITE` member on the provider path, the
`Executable` / `State` catalog wire fields, the UI capability badge and its
non-executable row path, the `schema.Message` generation on the run path, and
the test clusters that assert deferral.

**Added:** one binding (`NewResponsesModel`), one message generation, and the
dependent-session tooling S0 measures.

**Problems removed:** a capability state vocabulary that exists to describe
exactly one row; a cross-cutting concept whose only lift required a migration
spanning `internal`; and a UI row that renders as disabled for a reason the user
cannot act on.

**Problems explicitly retained:** `AgenticModel`'s upstream Beta marking; the
`DEFERRED-INDEFINITE` vocabulary in `sdk/port/support.go` (a different axis); the
45-vendor catalog and its custom-base-URL expressiveness question
(`DESIGN-CONCEPT.md` §6.7).

---

## Stage index

| File | Contents |
|---|---|
| `DESIGN-CONCEPT.md` | The adopted pattern, the three verified facts that make it viable, the three structural decisions, the rejected hub alternative with its measured cost, the three "no residue" rules, and the boundary conditions under which the concept is false. |
| `S0-dependency-truth.md` | Runnable method for measuring pins, the module-graph delta, and binary cost inside a throwaway module; the four questions S0 must answer with numbers. |
| `S1-component-contract.md` | The mandatory Eino capability check for the `AgenticModel` generation: per-component contract rows with module-cache citations, the ADK typed-agent surface, the middlewares lacking a typed variant, and the explicit "what the pinned surface cannot do" list. |
| `S2-local-httptest-loop.md` | The zero-network minimal closed loop: three hand-written SSE fakes, one ADK typed agent per protocol, four assertion channels, one failure path each, and the anti-self-validation and anti-silent-fallback rules. |
| `S3-migration-inventory.md` | Per-file migration inventory with measured scale, the residue-deletion set with per-item consumer checks, the risk table, and the two migration shapes priced. |
| `S4-decision.md` | The decision record: admissible outcomes, the option that is rejected outright, the thresholds fixed before evidence, and the decision-record contents. |
