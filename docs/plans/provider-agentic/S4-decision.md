# S4 — Decision

**Goal:** a signed go/no-go record for the migration, decided by S0–S3 evidence,
with the thresholds fixed **before** the evidence arrives.

**Status:** `UNSCHEDULED`. This document is the decision record's template and
its rules; the decision itself is what the stage produces.

**Depends on:** S0, S1, S2, S3.

---

## 1. Why the thresholds are written down first

A spike that decides after seeing its own results will decide whatever its
results happen to support. The value of this stage is that the criteria below
are fixed now, so a `GO` has to be earned and a `NO-GO` cannot be talked away.

## 2. Admissible outcomes

Exactly three. The stage must produce one of them. Unresolved evidence is named
as a blocker with a bounded resolution path, never silently counted as a pass.
These are spike outcomes about scheduling migration, not product release
approval; S3 §2.7 must pass before landing/release acceptance.

| Outcome | Meaning | What it implies |
|---|---|---|
| **`GO`** | Every threshold in §3 is met | The migration is scheduled as a phased program using S3's decomposition |
| **`GO-CONDITIONAL`** | Some thresholds are unmet, and every unmet one is named as a blocker with its own resolution path | The migration is scheduled **behind** those blockers; the program is not started until they close |
| **`NO-GO`** | A threshold is unmet and no resolution path exists within the pinned surface | The `DEFERRED-INDEFINITE` marking stays. The reason is recorded, and the deferral's restart condition is rewritten so the same investigation is not repeated from scratch |

## 3. Thresholds, fixed before evidence

A `GO` requires all four. Anything less is `GO-CONDITIONAL`; a threshold that
cannot be met at all is `NO-GO`.

### T1 — Dependency truth (from S0)

- The module-graph delta is classified in full, with no line left uncategorised.
- Like-for-like module probes preserve the application baseline constraints;
  additions/removals/bumps are attributable to the intended replacements.
- The component binary probe is measured and labelled as such, not claimed as
  the shipped product delta. The owner explicitly accepts its cost tradeoff
  against required user benefit and continuing the current deferred behavior.
- Full-product binary and relevant runtime cost acceptance are assigned to the
  migration gate in S3 §2.7. Unknown spike cost evidence is a named conditional
  blocker, not proof of affordability; no numeric target is invented here.
- The pseudo-version pin is legal under an existing precedent.
- No transitive pin conflicts with an existing pin.

### T2 — Component contract (from S1)

- Every capability in `S1-component-contract.md` §6 (C1–C11) has either a cited
  API or an explicit, evidenced absence.
- One verdict per component, from `ADOPT` / `ADOPT-WITH-ADAPTER` / `DEFERRED`.
  Every applicable required capability must be satisfied for `GO`; an evidenced
  absence or `DEFERRED` verdict is a blocker, not a completed-contract pass.
  `ADOPT-WITH-ADAPTER` permits only the scoped thin config/per-request projection,
  never a custom protocol codec, message bridge or replacement runtime.
- **No required capability needs a Vivy-owned protocol converter or message
  bridge.** If it does, this is `NO-GO` (decision D9) — not a `GO-CONDITIONAL`,
  because the bridge is the residue this program exists to avoid.
- Every Eino middleware Vivy depends on has a `NewTyped` entry point, or the
  absence is named as a blocker.

### T3 — Local closed loop (from S2)

- Channels A1–A4 (text, tool call, reasoning, usage) pass on **all three**
  protocols. A channel that passes on two of three is a blocker, not a
  pass rate.
- Failure path A5 passes on all three: every failure returns an error, and none
  returns a silently empty assistant message.
- Isolation A6 passes: the three protocols coexist in one process without shared
  state.
- Every assertion that could not be expressed is listed, and each is either a
  named `GO-CONDITIONAL` blocker or a `NO-GO` reason.

### T4 — Migration shape (from S3)

- The residue-deletion set in `S3-migration-inventory.md` §3 has **no remaining
  consumer** for any deleted item.
- The preserved list is confirmed untouched — in particular the declarative
  public `Profile` gains no capability field.
- The migration shape is decided: Shape A, or Shape B only if a two-generation
  interval is representable **without** a Vivy-owned bridge. If Shape B needs a
  bridge, the answer is Shape A or `NO-GO`.
- Every risk row has a runnable detection plan, and S3 §2.7's product-path,
  durable-run upgrade/rollback and cost checks have named migration phase gates.
  Their future execution is not represented as completed spike evidence.

## 4. The option that is rejected outright

Recorded here so it is a decision rather than an omission:

> **Keep the `ToolCallingChatModel` generation for the two adapters that already
> work, and introduce `openai-responses` on its own generation.**

This is the superficially cheap answer — it avoids the 145/24 + 312/42
occurrence migration — and it is **rejected**. It contradicts decisions D1 and
D6: it makes two message generations permanent, which makes the adapter table a
mixed-generation table, which makes capability and generation both facts that a
row must carry, which is precisely the state-and-condition vocabulary this
program deletes. Under the existing single typed model seam, it would also require a
`Message`↔`AgenticMessage` bridge that no supported API provides
(`S1-component-contract.md` §4.4). This rejects that migration shape within the
current architecture; it is not proof that all possible dual-runner designs are
impossible or that migration is cheaper than continuing to defer Responses.

A `NO-GO` is the honest alternative to this option, not a step toward it.

## 5. Decision-record contents

The signed record must contain, in this order:

1. **The outcome** and the date.
2. **The evidence table** — T1–T4, each threshold with its measured result and a
   pointer to the S-file section that holds it.
3. **Named blockers**, one row each: what is missing, which threshold it fails,
   what would close it, and whether closing it is in scope for this program.
4. **Shape A / Shape B decision**, with the bridge-search evidence behind it.
5. **What is deleted** — the residue-deletion set, confirmed, with the preserved
   list restated so a later reader cannot mistake preservation for oversight.
6. **What is added** — the new binding, the generation change, and the measured
   dependency/binary cost.
7. **The phase schedule** — S3's decomposition, adopted or amended, each phase
   with its gate (`just ci` plus the phase-specific acceptance) and its own
   `docs/logs/` record.
8. **Upgrade and rollback** — reverting code restores embedded provider data,
   but does not establish compatibility of durable run state. The existing
   `VersionedCheckpointStore.Get` rejects checkpoints from a different Eino
   version; v0.9.20/v0.9.21 checkpoints therefore cannot be resumed directly
   across either direction. Record the pending-run drain/finish or safe restart
   policy, user-visible outcome and synthetic upgrade/downgrade checks from
   S3 §2.7. Inventory Journal/history changes as well. Do not add a checkpoint
   converter, bypass version checks, or claim there is no persisted state effect.
9. **The deferral's disposition** — either the `DEFERRED-INDEFINITE` marking is
   removed from the provider path on landing, or, on `NO-GO`, its restart
   condition is rewritten in `docs/DEFER.MD` to point at the specific missing
   capability rather than at the migration as a whole.

## 6. Signature

A human signs. A stage may not sign its own conclusion, and an agent may not
turn a `GO-CONDITIONAL` into a `GO` by declaring a blocker closed without the
evidence that closed it.

## 7. Exit evidence

The signed record from §5, complete. If the outcome is `NO-GO`, the exit
evidence additionally includes the amended `docs/DEFER.MD` row and a one-line
statement of what upstream change would restart the work.
