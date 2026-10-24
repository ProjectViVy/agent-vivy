# AGENT-VIVY Documentation Index

> Documentation for the Vivy species kernel, the independent VIVY CODE
> terminal, and the first-party Studio overlay.
> Canonical product rules: `architecture/VIVY-STUDIO.md`.
> Updated: 2026-10-24

## 1. Reading order

1. `TODO.md` §0 — current posture and open work (the living board)
2. `COMPLETE.MD` — completed TODO history
3. `DEFER.MD` — deferred, superseded, and declined work
4. `architecture/VIVY-STUDIO.md` — canonical product contract
5. `plans/README.md` — active program plans, epics, and Story packages
6. `research/README.md` — decision and design archive (pre-V0)

## 2. Top-level files

Only boards and this index live at the top level. Everything else has a
category directory.

| File | Purpose |
|---|---|
| `README.md` | This index: reading order, directory map, placement rules |
| `TODO.md` | Living board: open work is in §0.1; completed history in `COMPLETE.MD`, deferred/superseded/declined work in `DEFER.MD` |
| `COMPLETE.MD` | Historical archive of completed, closed, and superseded TODO work |
| `DEFER.MD` | Deferred, superseded, and declined work moved off the open board; nothing here is scheduled |

## 3. Directories

| Directory | Contents | Entry point |
|---|---|---|
| `architecture/` | Canonical, normative product contracts (VIVY-STUDIO, VIVY-ASSEMBLY, VIVY-PLUGIN-SPEC, VIVY-MODULE-STANDARD, VIVY-PORT-CATALOG, storage, logging, HITL review, memory, SCX) | Start at `architecture/VIVY-STUDIO.md` |
| `specs/` | Detailed designs that plan packages implement | `specs/README.md` |
| `plans/` | Program plans, epics, Story packages, and one-off implementation plans | `plans/README.md` |
| `research/` | Research dossiers, comparisons, audits, and decision inputs | `research/README.md` |
| `dev/` | Current development runbooks (real-provider smoke, sandbox) | `dev/README.md` |
| `logs/` | Per-delivery records named `YYYY-MM-DD-slug/`, each with `summary.md`, `verification.md`, and `acceptance.md` | newest directory first |
| `archive/` | Closed tracks and historical records kept for traceability; not current guidance | `archive/README.md` |

## 4. Where a new document goes

- Normative product or protocol contract → `architecture/`
- Detailed design behind a plan package → `specs/`
- Program plan, epic, or Story package → `plans/<package>/`
- Research, comparison, audit, or decision input → `research/`
- Delivery record for a shipped change → `logs/YYYY-MM-DD-slug/{summary,verification,acceptance}.md`
- Daily runbook (dev loop, provider smoke, sandbox) → `dev/`
- Closed or superseded document → `archive/`; never delete evidence

## 5. Maintenance rules

- Register every new document in the relevant directory index with a one-line
  purpose. The top-level index only routes; it does not duplicate package
  state.
- Package state (current Story, dependency edges) lives in that package's
  `index.md`; do not restate it here.
- Documents referenced by code comments or the repository-root `README.md`
  keep stable paths; when one must move, update the references in the same
  commit.
- `research/` follows its own maintenance rules (see `research/README.md`).
- Archived documents are records: append corrections, do not rewrite their
  claims. Path-only link updates are allowed when files move.
