# Summary — P2-1: full Diva capability inventory (Keep/Adapt/Defer/Drop)

## What changed

- `docs/research/diva-capability-inventory.md` (new): an on-site inventory of
  `morediva/agent-diva` (Rust, 17 crates), producing a capability inventory of 68 rows ×
  10 domains: **Keep 29 (28 delivered + 1 awaiting proposal: message edit/rewind/fork =
  UI-CHAT-ACT) · Adapt 15 · Defer 18 · Drop 6**.
  - Each row includes Diva evidence paths (README / AGENTS-ARCH.md / LAPUTA.md / crates /
    `providers.yaml` grep for 47 presets / `#[tauri::command]` grep for 176+1 commands) and
    a comparison with Vivy's state on 2026-09-02 (delivery is determined from TODO §10 +
    `docs/logs/`).
  - Replaces the V0 placeholder inventory in §5 of
    `AGENT-VIVY-ASSEMBLY-OPTIONS.md` (that document's §5 remains a V0 decision record and
    is no longer the entry point for the full inventory).
  - §12 reiterates the ASSEMBLY-OPTIONS §6 re-entry rule: **tag ≠ implementation
    authorization**; Defer→implementation / Adapt→implementation must first go through a
    capability proposal, and must not start from Diva crate boundaries, Tauri command
    names, or the old schema.
- `docs/TODO.md`: moved the P2-1 line to DONE + §10 record.
- `docs/research/OPEN-ITEMS.md`: moved P2-1 to DONE.

## Method note

The inventory was completed by a read-only exploration agent (25 tool calls, with
file-by-file evidence); unverifiable entries (the wtf tool, neuro_link channel, and
internal core/ modules) are explicitly marked "unverified" and handled with conservative
tags (Drop / Defer). The agent-diva TODOLIST.md future vision (Workbench/PEN/Mirror, A2A,
etc.) was identified as a **planned backlog rather than delivered capability** and is not
included in the inventory.

## What was explicitly not done

- Do not change any TODO priority; do not automatically open work for Keep rows that are
  not delivered in the inventory.
- 1.10 (message edit/rewind/fork) keeps the existing UI-CHAT-ACT schedule (design slice
  first).

## Scope

Docs-only; zero code.
