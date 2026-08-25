# Port selected agent-diva repository rules into agent-vivy

Date: 2026-08-25
Status: complete (docs / process only)

## What changed

Root `AGENTS.md` now carries a trimmed port of agent-diva's process rules,
taken from that repo's main `AGENTS.md` (also mirrored under
`ui/agent-diva-source/AGENTS.md`).

Ported:

- Documentation placement (`docs/` vs root entry points)
- Iteration logs under `docs/logs/YYYY-MM-DD-slug/`
- Living backlog = `docs/TODO.md` §0.1 (not a root `TODOLIST.md`)
- Validation gate (`just ci`) plus user-visible smoke
- Secrets / structured errors / no-key-in-logs
- Native vs gateway model-id rule
- A short mandatory Rulebook

Also pointed `vivy-kernel-ci` and `ui/AGENTS.md` at the same log protocol.

## Explicitly not ported

- `LOCK.md` parallel mutex
- Root `TODOLIST.md` and `/new-command` index
- Auto-commit after every update
- `[I strictly follow the rules]` reply prefix
- Rust-workspace / GUI Tauri smoke rules
- BML / Laputa / Memory product rules (Vivy defers that family)

Log directory layout follows Vivy's existing folders (flat date-slug with
`summary` / `verification` / `acceptance`), not agent-diva's nested
`v0.0.1-slug` tree.

## Scope

Process docs only. No kernel, plugin, or UI behavior change.
