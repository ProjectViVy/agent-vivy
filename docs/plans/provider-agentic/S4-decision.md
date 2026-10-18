# S4 — Decision

**Goal:** a signed go/no-go record for the migration, decided by S0–S3 evidence,
with the thresholds fixed **before** the evidence arrives.

**Status:** `SIGNED 2026-10-18 — GO-CONDITIONAL` by the owner (§8.10). §1–§7
are the rules as reviewed; the record is §8. The migration is scheduled behind
the open blockers in §8.3.

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

## 8. Decision record — SIGNED 2026-10-18

### 8.1 Outcome

**`GO-CONDITIONAL`**, proposed 2026-10-17. No threshold is unmet *without* a
resolution path, and no required capability needs a Vivy-owned converter or
bridge, so `NO-GO` does not apply under §2. Several thresholds are unmet, so
`GO` does not either. The migration program is scheduled **behind** blockers
B1–B7 and does not start until they close.

**Owner decisions 2026-10-18:** B1 consent half approved (Eino core upgrade
OK); B2 accepted. **B1 closed the same day**: the pending-run policy is in
`docs/architecture/hitl-review-center.md` and phases P0/P1 landed
(`docs/logs/2026-10-18-eino-core-upgrade/`). B3 and B4 accepted by the owner
the same day; B4's defect stays a reported bug
(`STREAM-TRUNCATION-SILENT-SUCCESS`, `docs/TODO.md`). Remaining: B5 (P2 work), B6, B7.

The evidence also narrows what the migration buys (B2). Owners should read
B2 before treating this as an endorsement.

### 8.2 Evidence table

| Threshold | Result | Where |
|---|---|---|
| T1 delta classified, like-for-like | **Met.** +10 / −2 / ~5 modules, identical on both baselines, every line attributed | S0 §7.3 |
| T1 component probe measured, labelled | **Met.** +5,185,536 B (+15.2 %) linux/amd64 | S0 §7.5 |
| T1 owner accepts cost vs benefit | **Met 2026-10-18** — owner accepted B2 | S0 §7.5 |
| T1 product cost assigned to migration gate | **Met** (P4) | S3 §7.6 |
| T1 pseudo-version legal | **Met** (`go.mod:7,18-19`) | S0 §7.7 |
| T1 no transitive pin conflict | **Met.** Build + provider/runtime tests unchanged on both baselines | S0 §7.6 |
| T2 C1–C11 cited or evidenced absent | **Met**, plus C12–C14 added | S1 §9.3 |
| T2 every applicable capability satisfied | **Unmet in two cells**: Responses C6 replay (B3), Claude C8 total (B5); C1 vendor reach unproven for compatible gateways (B7) | S1 §9.3, S2 §9.3.7 |
| T2 verdicts | `ADOPT-WITH-ADAPTER` ×3, thin option projections only | S1 §9.4 |
| T2 no Vivy-owned converter/bridge | **Met** | S1 §9.5 |
| T2 `NewTyped` for every Eino middleware | **Met** (7/7) | S1 §3, §9.1 |
| T3 A1, A2 on all three | **Met** | S2 §9.2 |
| T3 A3 on all three | **Unmet**: Responses replay of `encrypted_content` (B3) | S2 §9.2 |
| T3 A4 on all three | **Unmet**: Claude total under output-only delta (B5) | S2 §9.2 |
| T3 A5 on all three | **Unmet**: truncated stream reports success on all three (B4) — pre-existing on the current stack too | S2 §9.3.1 |
| T3 A6 | **Met** under `-race` | S2 §9.2 |
| T3 inexpressible assertions listed | **Met** (none) | S2 §9.4 |
| T4 deletion set has no remaining consumer | **Met** (consumers enumerated, two keep-as-is) | S3 §7.3 |
| T4 preserved list untouched; Profile gains no field | **Met** | S3 §7.3 |
| T4 shape decided | **Met: Shape A** | S3 §7.5 |
| T4 every risk has a runnable detection; §2.7 gates assigned | **Met** (R1–R21; P0–P4) | S3 §7.4, §7.6 |

### 8.3 Named blockers

| # | What is missing | Fails | What closes it | In scope? |
|---|---|---|---|---|
| **B1** · *CLOSED 2026-10-18: consent approved by owner; policy and P1 delivered (`docs/logs/2026-10-18-eino-core-upgrade/`)* | Owner consent to move Eino core off v0.9.13, plus a pending-run policy across the version boundary. `EINO-CORE-UPGRADE` (`docs/DEFER.MD:30`) is an owner decision to stay on v0.9.13 until such a policy exists | T1/T4 (R13, R20) | Owner lifts the deferral; P0 writes the drain/finish/restart policy; P1 proves it with synthetic upgrade/downgrade checkpoints | Yes (P0/P1) |
| **B2** · *ACCEPTED by owner 2026-10-18* | Owner acceptance of cost vs benefit (why the cost is large: S0 §7.5). **Cost**: +5.2 MB component probe, a second OpenAI client (`openai-go/v3` + `azcore`) that does not replace `go-openai` (S0 Q1/Q2), and a 23-file run-path + 42-file test migration. **Benefit**: `openai-responses` becomes executable, and the deferral vocabulary is deleted. Reasoning visibility is **not** a new benefit; DeepSeek and Claude already surface it (S2 §9.3.6) | T1 | Owner signs with this tradeoff stated, or declines (then §8.9's NO-GO wording applies by owner choice) | Owner decision |
| **B3** · *ACCEPTED by owner 2026-10-18: rely on `store` (API default)* | Streamed Responses reasoning cannot be replayed statelessly (`encrypted_content` dropped, `agenticopenai v0.2.4 responses_event_convertor.go:595-606`) | T2 C6, T3 A3 | Either the owner accepts `store` = server default (OpenAI retains responses), or an upstream fix maps `item.EncryptedContent` on the streaming `output_item.done` path. No Vivy workaround (D9) | Owner decision / upstream |
| **B4** · *ACCEPTED by owner 2026-10-18 as pre-existing; stays a reported bug, not fixed* | Truncated stream is reported as success on every adapter, current and agentic | T3 A5 | Pre-existing defect, tracked as `STREAM-TRUNCATION-SILENT-SUCCESS` (`docs/TODO.md` §0.1). The owner may accept it as "no regression" for this program; the fix (upstream terminal-event check, or a domain-level finish-reason check) is decided on that row | Outside the program |
| **B5** | Claude `TotalTokens` wrong when `message_delta` carries output tokens only | T2 C8, T3 A4 | Vivy's usage projection derives total = prompt + completion (domain projection, not a codec), plus a P2 test; optionally confirm the live wire form with authorized credentials | Yes (P2) |
| **B6** · *handed to the upstream maintainer (needs real keys)* | Claude caching semantics change (`AutoCacheControl` breakpoints → top-level `cache_control`) | T2 C12 (added) | P4 cache-read check on a two-turn run with an authorized key; owner accepts the measured behavior | Yes (P4) |

| **B7** · *handed to the upstream maintainer (needs real keys)* | Chat-completions user `content` is always an array of parts on the agentic component, a plain string today (S2 §9.3.7, R21). Unknown which of the 45 compatible vendors reject it | T2 C1 (vendor reach) | An authorized live check on DeepSeek and each compatible vendor family, or an upstream change that collapses a single text part to a string (as the library already does for tool results). No Vivy codec (D9) | Needs credentials + owner approval; upstream |

### 8.4 Shape

**Shape A** (S3 §7.5). A two-generation interval is not representable through
`Ref.Model`'s single return type without a bridge, and the bridge search
(S1 §9.2) finds none.

### 8.5 What is deleted

S3 §7.3, confirmed by consumer search: `ErrAdapterDeferred`, the deferred
branch of `checkAdapter`, `Adapter.State`, `CapabilityDeferredIndefinite` and
the rest of `modelhost.CapabilityState` on the provider path, the
`modelhost/profile.go:122` deferral status branch, the RPC `Executable`/`State`
adapter fields, the UI `ProviderAdapterState` type and `declaredCapability`,
the `reconcile.go` deferral comment, `resolvingChatModelWithTools`, the
text-only `runtime.WrapModel` bridge (test-only today), and `schema.Message`
on the run path.

**Preserved** (not oversight): `sdk/port/support.go`,
`sdk/port/providerprofile` Profile (no field added), the generated default
expectation, `ProviderProfiles` evidence anchors, runtime profile states
(`COMPILED`/`UNCONFIGURED`/`READY`/`UNAVAILABLE`), `ErrAdapterUnknown`,
`AdapterSupportsCapability`, and `profileSelectionError`.

### 8.6 What is added

One binding (`agenticopenai.NewResponsesModel`), the `AgenticMessage`
generation, `agenticopenai v0.2.4`, `agenticclaude v0.1.7`, `openai-go/v3
v3.44.0`, the Azure SDK modules of S0 §7.3 (two linked), Eino core v0.9.21.
Measured component cost +5,185,536 B; product cost measured at P4.

### 8.7 Phase schedule

S3 §7.6 adopted as proposed: **P0** decisions → **P1** Eino bump alone on the
current generation → **P2** Shape A → **P3** residue + RPC/UI → **P4**
product acceptance. Each phase lands with `just ci`, its own gate and its own
`docs/logs/` record.

### 8.8 Upgrade and rollback

- Reverting code restores embedded provider data; it does **not** make durable
  run state compatible.
- **Two boundaries**, not one: the Eino version (P1, caught by
  `VersionedCheckpointStore.Get`, `checkpoint.go:107`) and the message
  generation (P2, **not** caught by the version check if P1 shipped first —
  R20). Both directions are fail-closed today only for the first.
- Required before P1/P2 land: a written pending-run disposition (drain/finish
  or restart, with the user-visible outcome) for each boundary in both
  directions; synthetic old-format checkpoints in tests (never production
  journals); no checkpoint converter; no bypassed version checks.
- P1 clarification: version refusal in historical code is not cleanup safety.
  Upgrade to the fixed v0.9.21 binary is supported; historical rollback requires
  completing and verifying cleanup under the current binary first. Actual
  binary tests with paused Approval/Question/Plan are required; see
  `docs/architecture/hitl-review-center.md` and
  `docs/logs/2026-10-18-pr80-recovery/verification.md`. This does not close
  the P2 message-generation boundary or B5/B6/B7.
- Journal: no schema change (domain events only, S3 §7.2 2.5); the projection
  changes, so reopening a pre-migration session must render identically
  (P4 check).

### 8.9 Disposition of the deferral

- On landing (after P3): `PROVIDER-AGENTIC-MIGRATION` and the
  `DEFERRED-INDEFINITE` marking on the provider path are removed.
- If the owner declines at B1 or B2: record it as a `NO-GO` by owner choice and
  rewrite the `docs/DEFER.MD` restart condition to: *"restart when the owner
  accepts the Eino core upgrade with a pending-run policy and the cost in
  S0 §7.5, or when upstream `eino-ext/components/model/openai` gains a
  Responses path on the ChatModel generation."*

### 8.10 Signature

| Role | Name | Date | Decision |
|---|---|---|---|
| Owner | songbai-vv | 2026-10-18 | `GO-CONDITIONAL` — migration scheduled behind B5 (P2 work), B6 and B7; the B6/B7 live-key checks are left to the upstream maintainer |

Signed by the owner in the 2026-10-18 session; the record was prepared by an
agent. Per §6, no agent may close a remaining blocker without the evidence that
closes it.
