# AGENT-VIVY V0 — Go / No-Go Pre-Flight

> **Status:** Final pre-flight record before V0 implementation work begins.
> **Updated:** 2026-08-07
> **Owner:** 📋 John (PM) → user (mastwet)
> **Purpose:** Single-source checklist of everything that must be true, true-or-decided, or accepted as risk before V0 implementation starts.
> **Related:** All five sibling documents in `diva-go/`.

---

## 1. How to read this document

- **PASS** = condition holds; nothing else to do.
- **PENDING** = condition is partially satisfied; one named item must close before V0 begins.
- **RISK ACCEPTED** = condition is not fully met and we are proceeding anyway; the residual risk is recorded for the implementation team to monitor.
- **BLOCKED** = condition is not met and V0 must not begin until it is met.

## 2. Hard requirements (BLOCKED if not PASS)

| ID | Requirement | Status | Evidence |
|---|---|---|---|
| HR-1 | PRD exists, is at v0.5+, and is the single source of truth for product scope | **PASS** | `prd-agent-vivy-v0.md` v0.5, 477 lines |
| HR-2 | Philosophy anchors are recorded and quotable | **PASS** | PRD §5.0.1–§5.0.6 |
| HR-3 | Anti-clone boundary is recorded as a decision, not just as prose | **PASS** | PRD D-001, D-014..D-021 |
| HR-4 | Provider set is locked to a countable list | **PASS** | PRD D-018: exactly two (`openai`, `anthropic`); schema source D-022 |
| HR-5 | V0 storage backend is locked to one implementation | **PASS** | PRD D-026, D-031 (SQLite only; fsjournal is V1+) |
| HR-6 | Eino two-layer checkpoint bridge is recorded as the canonical pattern | **PASS** | PRD D-028..D-030 |
| HR-7 | Backend conformance suite has a defined minimum bar for V0 | **PASS** | PRD D-032 (≥16 cases V0; full 18 V1+) |
| HR-8 | Single Go module is locked | **PASS** | PRD D-006 |
| HR-9 | Eino dependency is licensed permissively | **PASS** | REFERENCE-INDEX §3.2: Apache-2.0 |
| HR-10 | The V0 reference Eino source is on disk and verifiable | **PASS** | `diva-go/.workspace/eino/` present |

## 3. Soft requirements (PENDING or RISK ACCEPTED)

| ID | Requirement | Status | Evidence | Close-by action |
|---|---|---|---|---|
| SR-1 | ADR baseline exists for the storage / lifecycle / events / provider / tools / UI / recovery surfaces | **PENDING** | D-035: `AGENT-VIVY-ARCHITECTURE-V0.md` and ADR-002..008 v0 not on disk | Author ADR baseline as the first implementation artifact |
| SR-2 | Addendum §6 Eino claims are verified against `diva-go/.workspace/eino/` | **PENDING** | D-034: QwenPaw does not validate §6 (no `eino` import in QwenPaw) | Verify Eino's `CheckPointStore`, interrupt/resume, payload serializer against vendored source |
| SR-3 | Addendum is formally adopted (or explicitly rejected) | **PENDING** | Addendum supersedes ADRs that don't exist; D-035 holds adoption until ADR baseline exists | Decide: adopt-as-is, adopt-with-edits, or treat as proposal-only after SR-1 closes |
| SR-4 | QwenPaw is either vendored or explicitly held external | **PENDING** | RI-OQ-5: QwenPaw currently only at `/tmp/QwenPaw` | Recommend vendoring into `diva-go/.workspace/qwenpaw/` for first concrete use |
| SR-5 | claude-code license status | **PENDING** | RI-OQ-1: no LICENSE file detected in `.workspace/claude-code/` | Confirm upstream license or treat as all-rights-reserved permanently |
| SR-6 | `rig` license form | **PENDING** | RI-OQ-2: custom Playgrounds Analytics copyright header | Human review before any future reuse |
| SR-7 | Go module proxy connectivity for Eino | **PENDING** | Previous `go test ./schema -run '^$' -count=1` failed with `proxy.golang.org` timeout | Configure `GOPROXY` mirror or pre-warm cache before M0 spike |
| SR-8 | Diva capability inventory exists | **PENDING** | PRD OQ-1: not authored | Required for V1+ capability proposals; not blocking V0 |

## 4. Risk register (RISK ACCEPTED with monitor)

| ID | Risk | Mitigation already in place | Monitor during V0 |
|---|---|---|---|
| RK-1 | Diva old implementation gets silently reintroduced | D-001, D-014..D-021; anti-clone check in ARCHITECTURE-SOP §3 Step 4 and Step 7 | Code review on every PR; `grep` for `agent-diva-` and `agent-diva-deep-governance` references |
| RK-2 | Scope creeps beyond V0 vertical slice | D-006 (single Go module), D-031 (single backend), D-018 (two providers) | Each milestone must pass conformance suite + FR coverage before next milestone starts |
| RK-3 | Eino lifecycle does not match Vivy Run state machine | D-026..D-030 record adapter boundary | M0 spike verifies cancellation, single terminal event, restart recovery before M1 starts |
| RK-4 | Provider YAML accidentally carries more than two providers | D-023 enumerates excluded entries | `schemas/providers/` review on every change |
| RK-5 | UI is built against mocks first | D-013 + PRD §6.2 + DIRECTION §4 | M1 must connect to real Go process before M2 |
| RK-6 | Approval logic enforced only in UI | D-029 + FR-7 acceptance criteria | Server-side authorization test denies UI override |
| RK-7 | Restart recovery leaks duplicate terminal events | D-032 conformance case 6 ("exactly one terminal outcome under racing terminalizers") | Conformance test must be red-then-green, not skipped |
| RK-8 | Personal-gateway philosophy drifts toward SaaS | D-014..D-016 | Any new capability touching multi-tenancy, hosted control plane, or cross-user data triggers capability proposal |
| RK-9 | Logs as first-class citizens silently degrades | D-017 + FR-5/FR-8/FR-11 | Run-detail UI view must read from journal, not from in-memory state |

## 5. Open Questions inherited (carry forward to V0 implementation)

| ID | Question | Owner | When |
|---|---|---|---|
| OQ-1 | Diva capability inventory: which Diva capabilities are candidates for AGENT-VIVY proposals? | 📋 John | After V0 ships, before V1 capability proposals |
| OQ-2 | First capability proposal after V0 ships | 📋 John + user | V1 |
| OQ-3 | Long-term architecture redesign beyond thin V0 shell | user + future architect | After V0 evidence |
| OQ-4 | Provider breadth order beyond OpenAI + Anthropic | 📋 John + user | V1 capability proposal |
| OQ-5 | Tooling breadth (file ops, shell, MCP, web fetch, etc.) | 📋 John + user | V1 capability proposal |
| OQ-6 | Memory and context policy beyond simple replay | 📋 John + user | V1 capability proposal |
| OQ-7 | Desktop UI / Tauri wrapper | user | V1 capability proposal |
| OQ-8 | Provider YAML bundle spec — schema derivation details and version policy | 📋 John + user | Before M1 (provider layer spike) |
| OQ-9 | Long-term module map (provider / runtime / session / memory / tools / events / ui) | user + future architect | After V0 |
| RI-OQ-1 | claude-code LICENSE upstream confirmation | user | Before any reuse |
| RI-OQ-2 | rig LICENSE human review | user | Before any reuse |
| RI-OQ-3 | Whether any "Defer" reference is actually a better V0 reference than Crush | user + future architecture session | Before M1 |
| RI-OQ-4 | Whether `.workspace/` is versioned in morediva or `.gitignore`d | user | Before M1 |
| RI-OQ-5 | Whether QwenPaw is vendored into `.workspace/` | user + 📋 John | Before SR-4 closes |

## 6. Definition of "ready to start V0"

V0 implementation may begin when **all** of the following are true:

- HR-1..HR-10 are **PASS** (currently: ✅ all PASS).
- SR-1 and SR-2 are **closed** (currently: PENDING).
- RK-1..RK-9 mitigations are in place (currently: ✅ all in place; monitors active from M0).
- The user has explicitly accepted the soft-requirement pending list as either (a) blocking start, or (b) RISK ACCEPTED with a monitor.

The first user action item that closes the gate is the **ADR baseline + Eino §6 verification** work item (SR-1 + SR-2).

## 7. Document inventory at `diva-go/`

| File | Purpose | Owner | Status |
|---|---|---|---|
| `AGENT-VIVY-DIRECTION.md` | Product positioning and staged evolution | user + 📋 John | Recorded, v1 |
| `AGENT-VIVY-ASSEMBLY-OPTIONS.md` | V0 implementation strategy and rationale | 📋 John | Recorded, v1 |
| `prd-agent-vivy-v0.md` | V0 PRD (functional requirements, decision log, philosophy anchors) | 📋 John | v0.5, final-pending-user |
| `REFERENCE-INDEX.md` | Local `.workspace/` reference projects + QwenPaw (verified) | 📋 John | v1, RI-OQ-1..5 open |
| `ARCHITECTURE-SOP.md` | How to run BMad `bmad-create-architecture` for V0 | 📋 John → 🏗️ Winston | v0.1, draft |
| `GO-NOGO-PREFLIGHT.md` | This file: final pre-flight checklist before V0 starts | 📋 John | v1, just written |
| `AGENT-VIVY-STORAGE-ARCHITECTURE-ADDENDUM.md` (incoming) | Storage architecture proposal; treated as proposal-only until SR-1 / SR-3 close | (author: external) | Captured in PRD D-026..D-033; not authoritative |
| `AGENT-VIVY-ARCHITECTURE-V0.md` (to be written) | ADR baseline: ADR-001..008 v0 | 📋 John → 🏗️ Winston | **MISSING — first work item** |
| `eino-capability-verify.md` (to be written) | Eino §6 verification record (D-034 close-out) | 📋 John → 🏗️ Winston | **MISSING — second work item** |

## 8. What the user (mastwet) owns before V0 begins

1. **PRD final confirmation.** Either (a) "v0.5 is final, no more D-/FR edits", or (b) a list of remaining changes.
2. **Soft-requirement acceptance.** Either (a) close SR-1, SR-2, SR-4, SR-7 first, or (b) accept them as RISK ACCEPTED with the monitor listed in §4.
3. **Open-question dispositions.** Decide which of OQ-1..OQ-9, RI-OQ-1..RI-OQ-5 the user wants addressed in M0 / M1 vs deferred.

## 9. What John owns before V0 begins

1. **SR-1**: Author `AGENT-VIVY-ARCHITECTURE-V0.md` (ADR baseline, ADR-001..008 v0). Closes D-035.
2. **SR-2**: Author `eino-capability-verify.md` (Eino CheckPointStore / interrupt / resume verification record). Closes D-034.
3. **SR-3**: Re-issue a clean acceptance or rejection decision on the addendum after SR-1 closes.

## 10. Revision Notes

- v1 (2026-08-07): Initial pre-flight record. Reads off `prd-agent-vivy-v0.md` v0.5 + D-001..D-035 + all sibling documents in `diva-go/`. Ten HRs PASS; eight SRs catalogued; nine RK mitigations active.
