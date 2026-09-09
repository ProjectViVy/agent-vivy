# AGENT-VIVY Documentation Index

> This directory contains all documentation for the Vivy species kernel and first-party Studio.
> Canonical product rules: `architecture/VIVY-STUDIO.md`.
> Updated: 2026-08-25

## 1. Recommended Reading Order

1. `TODO.md` §0 — Current posture and remaining work (the living board)
2. `architecture/VIVY-STUDIO.md` — Canonical product contract
3. `IMPLEMENTATION-PLAN.md` — V0 architecture implementation and translation
4. `research/README.md` — Decision and design archive index (pre-V0 archive, with its own reading order)

## 2. Top-Level Active Documents (Stable Paths; Do Not Move)

The following files are referenced by the repository-root `README.md` and `internal/runtime/` code comments; keep their top-level paths unchanged:

| File | Purpose |
|---|---|
| `TODO.md` | Living board: remaining work is in §0.1; closed tracks are archived in `logs/` |
| `IMPLEMENTATION-PLAN.md` | V0 implementation plan and architecture translation (TODO architecture reference) |
| `AGENT-VIVY-ARCHITECTURE-V0.md` | ADR baseline (ADR-001..009+), recording the V0 shape |
| `GOAL-AGENT-HARNESS-ROADMAP.md` | Harness strengthening roadmap (H0–H10) |
| `eino-capability-verify.md` | Eino v0.9.13 capability verification record (A1, checkpoint-bridge GO) |
| `secret-redaction-audit.md` | Secret-redaction audit (E3 / AS-9) |
| `v1-minimal-agent-proposal.md` | V1 minimal agent-layer capability proposal (MA-1..MA-4) |

## 3. Subdirectories

| Directory | Contents |
|---|---|
| `architecture/` | Canonical product contracts: VIVY-STUDIO, SELF-EVOLVING-GATEWAY, VIVY-ASSEMBLY, VIVY-CHANNEL-PACK, VIVY-PLUGIN-SPEC, VIVY-WORLDVIEW, VIVY-GATEWAY-AND-STUDIO, ACP-REMOTE-CONTROL-PROPOSAL, hitl-review-center |
| `dev/` | Development process reports and implementation records: `NEW_UI_ARCHITECTURE.md`, `PHASE1..4` reports, `real-provider-smoke.md`, `sandbox.md` + `SANDBOX-IMPLEMENTATION-SUMMARY.md`, `ui-migration/` (six UI migration documents; entry point `ui-migration/UI_MIGRATION_README.md`) |
| `research/` | Decision and design archive (pre-V0): direction, PRDs, assembly options, reference index, GO/NO-GO, open items, DSH gaps, HITL UI research, and more; see `research/README.md` for the index |
| `logs/` | Acceptance/release records archived by date (`2026-08-12-hitl-release-closure`, `2026-08-16-studio-lifecycle`, `2026-08-25-todo-board-archive`) |

## 4. Maintenance Rules

- New documents must be registered in this index with a one-line purpose; process reports go in `dev/`, and date-based acceptance records go in `logs/<date-topic>/`.
- Documents referenced by code comments or the root `README.md` must not be moved.
- The `research/` directory follows its own maintenance rules (see `research/README.md` §6).
