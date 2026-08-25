# AGENT-VIVY V0 — Document Index

> **Status:** Final document index for `diva-go/`. Single entry point for any reader.
> **Updated:** 2026-08-15
> **Owner:** 📋 John (PM) + user (大湿)

---

## 1. Read this first

This directory is the **decision and design dossier** for AGENT-VIVY V0. Implementation code lives in the separate git repo `diva-go/agent-vivy/` (single Go module, initialized 2026-08-07); this directory holds the contracts, decisions, references, and runbooks that the implementation team must honor.

## 2. Documents (read in this order)

| # | File | Purpose | When to read |
|---|---|---|---|
| 1 | `README.md` (this file) | Document map | First read |
| 2 | `AGENT-VIVY-DIRECTION.md` | Product positioning, project roles, staged missions (V0/V1/V2/V3), philosophy-vs-implementation boundary | Before any design work |
| 3 | `prd-agent-vivy-v0.md` | V0 functional requirements, decision log D-001..D-035, philosophy anchors, acceptance scenarios, milestones | Before any V0 implementation |
| 4 | `AGENT-VIVY-ASSEMBLY-OPTIONS.md` | V0 implementation strategy (Eino execution + Vivy thin shell + selective Crush reference), Keep/Adapt/Defer/Drop for V0 | When picking implementation approach |
| 5 | `REFERENCE-INDEX.md` | Per-project notes for all local reference projects (16 vendored + QwenPaw verified external); may-borrow / must-NOT-borrow rules | Before touching any reference code or schema |
| 6 | `ARCHITECTURE-SOP.md` | How to run BMad `bmad-create-architecture` for V0; per-step adaptations | Before running the architecture workflow |
| 7 | `GO-NOGO-PREFLIGHT.md` | Final pre-flight checklist; hard requirements, soft requirements, risk register, definition of "ready to start V0" | Before declaring V0 implementation GO |
| 8 | `OPEN-ITEMS.md` | Living index of remaining work (V0 archived 2026-08-25; see §0) | After V0; remaining HITL P1 / Channel / ACP |
| 9 | `AGENT-VIVY-STORAGE-ARCHITECTURE-ADDENDUM.md` | Storage architecture proposal; treated as **proposal-only** until ADR baseline reconciles (D-035) | When discussing storage design |
| — | `AGENT-VIVY-ARCHITECTURE-V0.md` | **MISSING — first work item**. ADR baseline ADR-001..008 v0 | (to be authored) |
| — | `eino-capability-verify.md` | **MISSING — second work item**. Verification record for addendum §6 Eino claims | (to be authored) |
| 10 | `agent-vivy/docs/architecture/VIVY-STUDIO.md` | **Studio 正本。** 独立应用；第一方日常开发 IDE；分发生命周期权威；其他获授权工具可直接开发（NG-26） | Before any Studio or post-V1 implementation |
| 11 | `agent-vivy/docs/architecture/SELF-EVOLVING-GATEWAY.md` | Species / kernel / packing narrative. Studio shape defers to `VIVY-STUDIO.md`. | Post-V1 evolution, packing, user plugins |
| 12 | `agent-vivy/docs/architecture/VIVY-GATEWAY-AND-STUDIO.md` | Compact English decision table (NG-1..NG-28, ST-*) | When you need the decision ids, not the essay |
| 13 | `agent-vivy/docs/architecture/VIVY-PLUGIN-SPEC.md` | User-plugin spec only (`plugins/`). First-party units are not plugins | When writing a user-defined capability |
| 14 | `agent-vivy/docs/architecture/VIVY-ASSEMBLY.md` | How Vivy is divided and packed: nouns + generation recipe | When deciding what is a tool vs a plugin vs the kernel |
| 14a | `agent-vivy/docs/architecture/VIVY-CHANNEL-PACK.md` | Proposal: first-party channels as cold-pluggable packed organs (kernel Host + `channels/` lib + config envelope) | When discussing Telegram/Feishu/etc. or "built-in plugins" |
| 15 | `agent-vivy/docs/architecture/VIVY-WORLDVIEW.md` | Why the product philosophy and the Vivy namesake are structurally the same. Does not replace PRD §5.0 | When the name, slogan, or species/Studio split needs a why |
| 16 | `agent-vivy/docs/research/DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md` | Capability comparison and gap analysis between DeepSeek Harness and agent-vivy (evidence-cited, dimension by dimension) | When deciding what the species should adopt, refuse, or defer from DSH |

## 3. Source artifacts in parent directories

- `diva-go/agent-vivy/` — **implementation repo** (independent git repo, single Go module `agent-vivy`). Consumes Eino exclusively as online dependencies; `.workspace/eino/` below is a read-only API reference only. Holds `docs/IMPLEMENTATION-PLAN.md` (architecture translation) and `docs/TODO.md` (M0–M4 work breakdown).
- `diva-go/.workspace/` — vendored reference projects used by V0 design.
  - `crush/` — Go application-assembly patterns reference.
  - `eino/` — V0 execution framework (read-only reference; NOT imported by `agent-vivy`).
  - `deepseek-harness/upstream/` — official DSH clone; Studio engine evidence, not a species dependency.
  - (other projects are tracked in `REFERENCE-INDEX.md` §3).
- `diva-go/` — this directory: design dossier only.
- `_bmad-output/planning-artifacts/prds/prd-agent-vivy-v0.md` — authoritative PRD copy maintained by the BMad workflow. `diva-go/prd-agent-vivy-v0.md` is a mirror for the dossier; both files must remain identical.

## 4. How decisions are made and recorded

1. **User direction** sets the high-level direction (philosophy, capability candidates, scope).
2. **PRD §13 Decision Log** records every direction with verbatim user quote (P3 principle).
3. **Corrections** are appended as `.r1`, `.r2` (P13 principle), never silently edited.
4. **Architecture baseline** (to be authored) will own its own ADRs; PRD decisions and ADRs must stay synchronized.
5. **Reference projects** are cataloged in `REFERENCE-INDEX.md`; their license and intent are the gate for any borrowing.

## 5. Cross-reference: how PRD decisions map to source files

| PRD Decision | Source artifact |
|---|---|
| D-001 (new product line, not Diva clone) | `AGENT-VIVY-DIRECTION.md` §1, `prd-agent-vivy-v0.md` §2 |
| D-002 (philosophy anchor: personal gateway) | `prd-agent-vivy-v0.md` §5.0.1 |
| D-017 (logs first-class) | `prd-agent-vivy-v0.md` §5.0.2 |
| D-018 (two providers, YAML pre-baked) | `prd-agent-vivy-v0.md` §5.0.3 |
| D-020 (large-modular decomposition) | `prd-agent-vivy-v0.md` §5.0.4 |
| D-022..D-025 (provider YAML from Diva) | `prd-agent-vivy-v0.md` §5.0.3 + `REFERENCE-INDEX.md` source-pointer |
| D-026..D-033 (storage contract, Eino bridge, conformance suite) | `prd-agent-vivy-v0.md` §13 + `AGENT-VIVY-STORAGE-ARCHITECTURE-ADDENDUM.md` (proposal-only) |
| D-034 (Eino §6 verification pending) | `OPEN-ITEMS.md` P0-2 |
| D-035 (ADR baseline missing) | `OPEN-ITEMS.md` P0-1 |

## 6. Maintenance rules

- New documents added to this directory must be listed in §2 with a one-line purpose.
- PRD changes must update both `_bmad-output/planning-artifacts/prds/prd-agent-vivy-v0.md` and `diva-go/prd-agent-vivy-v0.md` in the same commit / save.
- Decision-log additions must use P3 (verbatim user quote) and P13 (`.r1` suffix for corrections) conventions.
- Reference project entries must record verification (P9); unverified claims must be flagged.
- Anti-clone boundary (PRD D-014..D-021) applies to every document in this directory.

## 7. Revision Notes

- v1 (2026-08-07): Initial document index. Reflects the dossier as it stands at end of pre-V0 planning session.
- v2 (2026-08-07): Added `diva-go/agent-vivy/` as the implementation repo entry (§1, §3). The V0 skeleton is initialized there as a single Go module consuming Eino as online dependencies; `.workspace/eino/` is now marked read-only reference. Implementation plan + TODO board live in `agent-vivy/docs/`.
- v3 (2026-08-14): Linked next-generation Gateway + Studio architecture proposal (`agent-vivy/docs/architecture/VIVY-GATEWAY-AND-STUDIO.md`). Proposal only; V0 contracts unchanged.
- v4 (2026-08-14): Added the full discussion write-up `agent-vivy/docs/architecture/SELF-EVOLVING-GATEWAY.md` as the narrative source; Studio file remains the compact decision record.
- v5 (2026-08-15): Studio corrected to an independent application. Canonical `agent-vivy/docs/architecture/VIVY-STUDIO.md`. Development venue switches to Studio after ST-6. Species-side Studio card / Promote authority frozen.
- v6 (2026-08-23): Studio remains the first-party daily development IDE, but is no longer an exclusive execution venue. Other authorized tools work directly in the repository with their own capabilities.
