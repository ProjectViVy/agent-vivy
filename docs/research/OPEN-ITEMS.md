# AGENT-VIVY V0 — Open Items Index

> **Status:** Living index of all open work items, ordered by what blocks V0 implementation vs what can wait.
> **Updated:** 2026-08-07
> **Owner:** 📋 John (PM) + user (大湿)
> **Purpose:** Single place to see "what still needs doing" without re-reading every sibling document.

---

## 1. Conventions

- **Priority** — `P0` blocks V0 from starting; `P1` blocks specific milestones (M0–M4); `P2` should be done before V1; `P3` informational.
- **Status** — `PENDING` (not started), `IN PROGRESS`, `BLOCKED` (waiting on someone), `DONE`, `DEFERRED`.
- **Owner** — `John` (PM), `user` (大湿), `Winston` (architect when activated), `team` (whoever picks it up).
- **Closes** — the parent decision / open question / risk this item resolves.

## 2. P0 — Blocks V0 from starting

| ID | Item | Status | Owner | Closes |
|---|---|---|---|---|
| P0-1 | Author `AGENT-VIVY-ARCHITECTURE-V0.md` with ADR-001..008 v0 (stack, storage, run lifecycle, events, provider, tools & approval, UI, recovery). Each ADR ≤ one screen; preamble quotes the four philosophy anchors; ADR-002 reconciles with addendum §1–§3, §5. | PENDING | John → Winston | D-035, SR-1 |
| P0-2 | Author `eino-capability-verify.md` verifying addendum §6 claims against `diva-go/.workspace/eino/`. Specifically: (a) does Eino expose a `CheckPointStore` interface with Get/Set? (b) what is the default payload serializer? (c) is interrupt/resume available at ADK / Runner level, and does it have a public hook for Vivy's journal commit? | PENDING | John → Winston | D-034, SR-2 |
| P0-3 | User confirmation that **PRD v0.5 is final**. Either "v0.5 is final" or list of remaining D-/FR edits. | PENDING | user | HR-1 finalization |
| P0-4 | User decision on **SR acceptance posture**: close SR-1, SR-2, SR-4, SR-7 first, or accept as RISK ACCEPTED with the monitors in `GO-NOGO-PREFLIGHT.md` §4. | PENDING | user | V0 GO/NO-GO gate |

## 3. P1 — Blocks specific V0 milestones

| ID | Item | Status | Owner | Blocks |
|---|---|---|---|---|
| P1-1 | Resolve `GOPROXY` / pre-warm Eino module cache before M0 spike | PENDING | user | M0 spike (SR-7) |
| P1-2 | Author provider YAML bundle spec (schema derivation rules, version policy, `provenance` field schema). Implement two bundles: `openai.yaml`, `anthropic.yaml`. | PENDING | John | M1 (provider layer), OQ-8 |
| P1-3 | Define Vivy-owned JSON schema for the ten `RunEvent` types (PRD FR-5). Make it the single contract that Eino event stream and UI event stream both speak. | PENDING | John → Winston | M1 (events) |
| P1-4 | Define `ProviderRef` Go interface boundary; ensure non-`internal/provider/` packages cannot import provider-specific code. | PENDING | John → Winston | M1 (architecture boundary) |
| P1-5 | Decide where QwenPaw lives: vendor into `diva-go/.workspace/qwenpaw/` or hold external and re-fetch by URL. | PENDING | user + John | SR-4 |
| P1-6 | Compose backend conformance test harness scaffolding (16 cases for V0 per D-032). The tests will fail red on M0; turn green on M2 / M3. | PENDING | team | M2, M3 |

## 4. P2 — Should be done before V1

| ID | Item | Status | Owner | Closes |
|---|---|---|---|---|
| P2-1 | Diva capability inventory document: list every Diva capability, tag with `Keep / Adapt / Defer / Drop`, link to capability proposals. | PENDING | John | OQ-1, PRD §6.3 |
| P2-2 | First capability proposal after V0 ships. | DEFERRED | John + user | OQ-2 |
| P2-3 | QwenPaw filesystem journal backend probe spec — standalone artifact; out of V0 scope. | DEFERRED | John | D-031 |
| P2-4 | Long-term 板块 map (provider / runtime / session / memory / tools / events / ui) for V3 redesign. | DEFERRED | user + architect | OQ-9 |
| P2-5 | Decide `.workspace/` versioning policy (committable vs `.gitignore`). | PENDING | user | RI-OQ-4 |

## 5. P3 — Informational / ambient

| ID | Item | Status | Owner |
|---|---|---|---|
| P3-1 | Verify upstream LICENSE for `claude-code` (currently no LICENSE file found locally). | PENDING | user |
| P3-2 | Human review of `rig` LICENSE (custom Playgrounds Analytics form). | PENDING | user |
| P3-3 | Evaluate whether any "Defer" reference project is actually a better V0 application-assembly reference than Crush. | PENDING | user + future architect |
| P3-4 | Review whether Crush's FSL-1.1-MIT allows deeper reuse than V0 currently plans. | DEFERRED | user |
| P3-5 | Decide whether the storage architecture addendum should be (a) adopted as-is, (b) adopted with edits after ADR baseline reconciles, or (c) treated as proposal-only with D-026..D-033 capturing the substance. | PENDING | user + John (after SR-1 closes) |

## 6. Done items (archive)

| ID | Item | Closed by | Date |
|---|---|---|---|
| D-001 | Author PRD v0.1 with positioning and scope. | John | 2026-08-06 |
| D-002 | Add §5.0 philosophy anchors + D-014..D-016. | John (user direction) | 2026-08-06 |
| D-003 | Add D-017..D-021 for logs / curated providers / large-modular / philosophy boundary. | John (user direction) | 2026-08-06 |
| D-004 | Author REFERENCE-INDEX.md covering 17 projects (16 vendored + QwenPaw verified external). | John | 2026-08-06/07 |
| D-005 | Author ARCHITECTURE-SOP.md for BMad `bmad-create-architecture` adaptation. | John | 2026-08-06 |
| D-006 | Author GO-NOGO-PREFLIGHT.md as the final pre-flight record. | John | 2026-08-07 |
| D-007 | Review storage addendum; capture substance in PRD D-026..D-033; flag Eino (§6) and ADR baseline (§10) gaps as D-034, D-035. | John | 2026-08-06 |
| D-008 | P9-verify QwenPaw: license (Apache-2.0), source structure, key files. Resolve D-033 to `.r1`. | John | 2026-08-07 |

## 7. Revision Notes

- v1 (2026-08-07): Initial open-items index. 4 P0 items block V0 start; 6 P1 items block specific milestones; 5 P2 items should close before V1; 5 P3 items are ambient. Done items archived as D-001..D-008.