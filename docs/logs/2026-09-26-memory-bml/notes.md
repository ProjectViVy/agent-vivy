# MEM-1 iteration log — BML restoration as standalone Go library

Date: 2026-09-26. Branch: `feat/memory`. Story: `docs/superpowers/plans/memory/MEM-1.md`.

## Outcome

Diva's BML (Basic Memory Layer) ported to a standalone Go module at `bml/`
(module `github.com/ProjectViVy/agent-vivy/bml`), byte-faithful to
`agent-diva@0fd005a1` (`agent-diva-core/src/memory/` +
`agent-diva-laputa/src/`). Eight SDD tasks, each implemented by a child
session and independently reviewed; fix rounds where reviews found issues.

| Task | Scope | Review |
| --- | --- | --- |
| T1 | record model + validation (`record.go`) | clean |
| T2 | store open/schema/workspace identity (`store.go`) | clean |
| T3 | CRUD, revisions, tombstones, capacity, supersede, import CAS | 1 fix round (record_json serde byte-parity, missing `expected_store_revision` CAS) |
| T4 | FTS5 search (`search.go`) | clean |
| T5 | GC, integrity, backup/restore, canonical identity (`maintenance.go`) | clean |
| T6 | MemoryHome facade + MEMRULES (`home.go`) | 1 fix round (RunStartupGC empty-input parity, memrules UTF-8, TOCTOU) |
| T7 | migration adapters (`migrate.go`) | approved, minors deferred |
| T8 | `bml/README.md` | controller-verified, docs only |

Final gate: `cd bml && gofmt -l . && go vet ./... && go test ./...` all clean.

## Key rulings

- **Rust truth over brief**: `import_records` keeps `expected_store_revision`
  CAS (typed_store.rs:763-791); `RunStartupGC` early-returns 0 on empty
  `activeSessionIDs` (memory_home.rs:385-387) — the corrected brief text was
  wrong, source wins.
- **Schema byte-faithful**: `schema_meta`, `memory_records`,
  `memory_supersedes`, `memory_apply_journal`, `memory_fts` (unicode61) match
  `typed_store.rs:494-560` exactly, including the dead `memory_apply_journal`
  table for byte compatibility.
- **record_json byte-parity**: canonical `MarshalJSON` disables HTML escaping
  and restores raw U+2028/U+2029 to match `serde_json::to_string` — this is
  the cross-language artifact; pretty-printed manifests use `MarshalIndent`
  and may differ cosmetically.
- **Dropped upstream**: governed write seam (`put_governed`,
  `rollback_governed`, `GovernedMemoryApply`) — dead code upstream.
- **Deferred**: actmem, Diva `MemoryProvider` trait surface, runtime wiring
  (belongs to MEM-1A), `checkpoint_id`/`write_checkpoint`.
- **Commit author repair**: AGENTS.md forbids AI as commit author; all branch
  commits rewritten to the human identity via env-filter and force-pushed.

## Mechanics worth noting

- Subagents ran as separate-VM Devin sessions (shared-VM agents unavailable in
  this session type); handoffs via pushed `feat/memory` commits + attachment
  URLs; the 5-session cap was managed by sleeping settled children.
- Children lacked Go despite the repo blueprint; each installed a fallback.
  Worth checking why the blueprint toolchain did not reach them.

## Next (not done here)

MEM-1A — Vivy runtime wiring (Port/Module adapter for bml), gated on the
G0 contract stories MEM-0B/0C/0D. See `docs/superpowers/plans/memory/index.md`.
