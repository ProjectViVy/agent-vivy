# Documentation directory reorganization

## What changed

- Merged the two plan trees: `docs/superpowers/plans/` now lives in
  `docs/plans/`, and `docs/superpowers/specs/` became `docs/specs/`.
  `docs/superpowers/` no longer exists.
- Archived closed-track and historical documents under `docs/archive/`:
  - `AGENT-VIVY-ARCHITECTURE-V0.md`, `IMPLEMENTATION-PLAN.md`,
    `GOAL-AGENT-HARNESS-ROADMAP.md`, `v1-minimal-agent-proposal.md`,
    `eino-capability-verify.md`, `secret-redaction-audit.md`
  - `dev/PHASE1..4` reports, `dev/NEW_UI_ARCHITECTURE.md`,
    `dev/SANDBOX-IMPLEMENTATION-SUMMARY.md`, `dev/ui-migration/`
  - `plans/nudge/` (ND-0..ND-4; `plans/epic-nudge/` is the live package)
- Renamed the nonconforming log directory `2026-10-wails-migration` to
  `2026-10-04-wails-migration`.
- Rewrote `docs/README.md`: top level now lists only the boards and this
  index, routes through one directory map, and states where each kind of new
  document belongs.
- Added directory indexes: `docs/plans/README.md`, `docs/specs/README.md`,
  `docs/archive/README.md`, `docs/dev/README.md`.
- Updated references across 103 tracked files, including Go source comments,
  for every moved path. Recomputed relative Markdown links in 44 moved files
  so they resolve from the new depth.

## Scope and explicit non-goals

- The 461 delivery directories under `docs/logs/` stay flat and keep their
  `YYYY-MM-DD-slug` names; the AGENTS convention and 122 referencing files
  make month grouping a separate, higher-risk change.
- Historical records keep their content. Only paths and links changed.
- No runtime, UI, or product-contract behavior changed.

## Not done

- Full `just ci` (docs-only change).
- Windows-native acceptance (not applicable).
