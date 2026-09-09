# AGENT-VIVY V0 — Open Items Index

> **Status:** Living index. V0 is closed; remaining work matches `docs/TODO.md` §0.1.
> **Updated:** 2026-08-30
> **Owner:** 📋 John (PM) + user (mastwet)
> **Purpose:** Single place to see "what still needs doing" without re-reading every sibling document.
> **Archive:** `docs/logs/2026-08-25-todo-board-archive/summary.md`

---

## 0. Currently open (2026-08-25)

Canonical table: `docs/TODO.md` §0.1.

| ID | Status |
|---|---|
| CH-0 Super-channel contract | DONE (2026-08-30; C0 docs) |
| CH-A Host + telegram + dingtalk plugins | OPEN (depends on C1–C3 kernel/SDK) |
| CH-B feishu / qq / discord plugins | OPEN |
| CH-C wecom | OPEN (bind surface still the blocker; not this batch) |
| ACP-1 remote control implementation | DEFERRED |
| HITL-P1-1..7 | OPEN / DEFERRED |
| MEM-1 Memory / Laputa family | DEFERRED |
| P2-1 Diva inventory | OPEN |
| P2-3 / SR-4 QwenPaw fsjournal | DEFERRED / OPEN |
| P2-4 V3 module map | DEFERRED |
| P3-1 / P3-2 licenses | OPEN |
| UI-TREE child-run visualization | DEFERRED |

## 1. Conventions

- **Priority** — `P0` blocks V0 from starting; `P1` blocks specific milestones (M0–M4); `P2` should be done before V1; `P3` informational.
- **Status** — `PENDING` (not started), `IN PROGRESS`, `BLOCKED` (waiting on someone), `DONE`, `DEFERRED`.
- **Owner** — `John` (PM), `user` (mastwet), `Winston` (architect when activated), `team` (whoever picks it up).
- **Closes** — the parent decision / open question / risk this item resolves.

## 2. P0 — Blocks V0 from starting

V0 started and closed. All four rows are archived in §6.

## 3. P1 — Blocks specific V0 milestones

V0 milestones M0–M4 are closed. P1-1..P1-4 and P1-6 archived in §6.
P1-5 (QwenPaw location) remains as SR-4 / P2-3 — it never blocked V0.

## 4. P2 — Should be done before V1

| ID | Item | Status | Owner | Closes |
|---|---|---|---|---|
| P2-1 | Diva capability inventory document: list every Diva capability, tag with `Keep / Adapt / Defer / Drop`, link to capability proposals. | **DONE 2026-09-02** | John | OQ-1, PRD §6.3 — `diva-capability-inventory.md` (68 lines, Keep 29 / Adapt 15 / Defer 18 / Drop 6) |
| P2-2 | First capability proposal after V0 ships. | DONE | — | V1 MA-1..MA-4 shipped 2026-08-08; Channel Pack proposal 2026-08-25 is the next unadopted proposal |
| P2-3 | QwenPaw filesystem journal backend probe spec — standalone artifact; out of V0 scope. | DEFERRED | John | D-031 |
| P2-4 | Long-term module map (provider / runtime / session / memory / tools / events / ui) for V3 redesign. | DEFERRED | user + architect | OQ-9 |
| P2-5 | Decide `.workspace/` versioning policy (committable vs `.gitignore`). | DONE | — | `/.workspace/` gitignored |

## 5. P3 — Informational / ambient

| ID | Item | Status | Owner |
|---|---|---|---|
| P3-1 | Verify upstream LICENSE for `claude-code` (currently no LICENSE file found locally). | **DONE 2026-09-02** — proprietary upstream | `license-review-2026-09-02.md` |
| P3-2 | Human review of `rig` LICENSE (custom Playgrounds Analytics form). | **DONE 2026-09-02** — standard MIT; misreading corrected | `license-review-2026-09-02.md` |
| P3-3 | Evaluate whether any "Defer" reference project is actually a better V0 application-assembly reference than Crush. | PENDING | user + future architect |
| P3-4 | Review whether Crush's FSL-1.1-MIT allows deeper reuse than V0 currently plans. | DEFERRED | user |
| P3-5 | Decide whether the storage architecture addendum should be (a) adopted as-is, (b) adopted with edits after ADR baseline reconciles, or (c) treated as proposal-only with D-026..D-033 capturing the substance. | DONE | ADR-001..008 + D-026..D-033; addendum remains historical |

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
| P0-1 | `AGENT-VIVY-ARCHITECTURE-V0.md` ADR-001..009 | team | 2026-08-08 |
| P0-2 | `eino-capability-verify.md` | team | 2026-08-07 |
| P0-4 | SR acceptance posture recorded on TODO board | user + John | 2026-08-07 |
| P1-1 | GOPROXY + module cache | team | 2026-08-07 |
| P1-2 | Provider YAML bundles | team | 2026-08-07 |
| P1-3 | RunEvent JSON Schema | team | 2026-08-07 |
| P1-4 | ProviderRef boundary | team | 2026-08-07 |
| P1-6 | Conformance 16 cases (B5) | team | 2026-08-07 |
| P2-2 | First post-V0 capability (MA-1..MA-4) | team | 2026-08-08 |
| P2-5 | `.workspace/` gitignored | team | 2026-08-25 |
| P3-5 | Storage addendum vs ADR baseline | team | 2026-08-08 |
| V0 | M0–M4 board | team | 2026-08-07 |
| HITL-P0 | HITL-01..07 | team | 2026-08-12 |
| ST | ST-0..ST-8 Studio lifecycle | team | 2026-08-16 |

## 7. Revision Notes

- v1 (2026-08-07): Initial open-items index. 4 P0 items block V0 start; 6 P1 items block specific milestones; 5 P2 items should close before V1; 5 P3 items are ambient. Done items archived as D-001..D-008.
- v2 (2026-08-25): Archived V0/P0/P1 closed work. Remaining: HITL P1, Channel Pack C0+, ACP implementation, Memory family, QwenPaw/fsjournal, Diva inventory, ambient licenses. See `docs/TODO.md` §0.1.
