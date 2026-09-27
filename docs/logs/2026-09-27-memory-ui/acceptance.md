# Acceptance: how a human can tell MEM-1C is real

Product view: `/memory` is no longer a read-only list. An operator can add,
edit (revision-CAS guarded), and tombstone records; read and edit the
MEMRULES handbook under a digest CAS; and see backend availability plus the
startup revision at a glance. Every failed outcome shows the real backend
reason code — the UI never fabricates success.

## 1 — The surface covers the live contract

```text
rg -n "memory\\.(add|update|remove|rules\\.(read|write)|status|get)" \
  plugins/vivy-memory/ui/vivy-memory/src/
```

All nine `vivy.memory.*` actions have a UI path: list/search drive the
master list, get backs the CAS rebase, add/update/remove ride the dialogs,
rules.read/rules.write drive the MEMRULES tab, status drives the strip.

## 2 — Writes are CAS-honest

`memory.update` / `memory.remove` carry `base_revision` taken from the entry
the operator opened. On `memory_revision_conflict` the dialog stays open,
shows the conflict copy, and offers refresh — `vivy.memory.get` swaps in the
fresh record (keyed remount `id@revision`), or closes the dialog when the
record is gone. `rules.write` uses the `rules.read` digest token the same
way.

## 3 — Nothing is fabricated

`status: "failed"` renders `reason` inline (`bml_unavailable`,
`memory_not_found`, `memory_kind_forbidden`, `memory_invalid_request`,
`memory_io_error`, `output_too_large`); invoke rejects render the thrown
message. `MemoryEntry` fields render straight off the DTO — id, content,
trust, provenance, evidence_refs, revision, timestamps.

## 4 — Gates green

`pnpm run stage:ui` (module digest verified), `pnpm typecheck` clean,
`pnpm test` 409/409, i18n en/zh complete for all 33 units.
