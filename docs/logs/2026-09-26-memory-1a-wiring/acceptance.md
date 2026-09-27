# Acceptance: how a human can tell MEM-1A is real

Product/maintainer view: `bml/` is no longer an inert library — the Vivy
runtime opens it at composition, serves bounded recall to the context host,
ingests completed runs with receipt-truthful idempotency, exposes memory CRUD
as control actions, and the `vivy/memory` UI reads live data through
`module.action.invoke`.

## 1 — The wiring is in the artifact

```text
rg -n 'vivy/memory-bml' internal/modules/defaults/catalog.go recipes/default.vivy.yml
rg -n 'memorymodule\.Open|assemblyHasModule.*vivy/memory-bml' internal/app/app.go
rg -n 'ActionList|vivy\.memory\.' internal/modules/memory/actions.go
rg -n 'ObserveRunWithReceipt|contextsource' internal/modules/memory/provider.go
rg -n 'module\.action\.invoke' plugins/vivy-memory/ui/vivy-memory/src/memory-client.ts
```

Two catalog records (`vivy/memory-bml`, `vivy/memory-bml-sync`) both pointing
at `internal/modules/memory`; `memory.Open` gated on the compiled manifest; 9
control actions; recall + receipt-aware ingest; UI client on
`module.action.invoke`.

## 2 — Behavior is test-backed

```text
go test ./internal/modules/...
cd plugins/vivy-memory/ui/vivy-memory && pnpm test
```

Observer idempotency: `provider_test.go` proves a redelivered EventID writes
one history record and returns the same receipt. CAS discipline: `update`,
`remove`, and `rules.write` carry `base_revision` and return `conflict` on a
stale token. Truthful degradation: closed/absent service yields explicit
`unavailable`/`failed`, never fake content.

## 3 — Trackers agree

```text
rg -n 'MEM-1A' docs/TODO.md docs/DEFER.MD docs/research/OPEN-ITEMS.md \
  docs/superpowers/plans/memory/index.md
```

`index.md` shows MEM-1A Done 2026-09-26 pointing here; `docs/TODO.md` §0.1,
`docs/DEFER.MD`, and `docs/research/OPEN-ITEMS.md` say the same. MEM-2 stays
Blocked (plan written after G1 evidence) even though its MEM-1A predecessor is
now landed.

## Known limits

- `defaultRecallLimit = 100` bounds recall fan-out; the ceiling is a service
  constant, not yet a contract-pinned profile number (T3 review nit, deferred).
- `AppendHistory` treats a same-ID redelivery as a duplicate without checking
  replayed content equality — flagged by review for follow-up, not a shipped
  defect under the current single-writer ingest path.
- Agent-requested memory tools (`std/tool@v1`) and the deferred `MEM-CAP`
  capabilities (AutoDream / Evolution / RAG) are out of MEM-1A scope.
