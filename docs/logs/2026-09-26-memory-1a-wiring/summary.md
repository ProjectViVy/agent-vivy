# MEM-1A — Vivy runtime wiring for BML

Iteration: 2026-09-26. Scope: `docs/superpowers/plans/memory/MEM-1A.md` —
wire the `bml/` library into the Vivy runtime as a T1 module (bounded recall,
committed-experience ingest, explicit memory CRUD + status) and rewire the
`vivy/memory` UI view from demo stubs to `module.action.invoke`. Executed as
subagent-driven development: four tasks, each implemented and independently
reviewed; fix rounds where reviews found issues.

## What landed

| Task | Outcome | Commits |
| --- | --- | --- |
| T1 | Memory module skeleton + composition wiring: `internal/modules/memory` (`module.go`, `service.go`); two catalog records `vivy/memory-bml` (control-action collection) and `vivy/memory-bml-sync` (one provider for `std/context-source@v1` + `std/observer/run@v1`) in `internal/modules/defaults/catalog.go`; `memory.Open(ctx, cfg)` called once in `internal/app/app.go` after storage open and only when the compiled manifest contains `vivy/memory-bml`, closed on shutdown; both records added to `recipes/default.vivy.yml`; generated assembly regenerated | `47a920a9` |
| T2 | 9 control-action providers (`actions.go`): `vivy.memory.list/search/get/add/update/remove/rules.read/rules.write/status`, four-state outcome contract, input/output caps (`32<<10`/`256<<10`), `base_revision` CAS on `update`/`remove` | `e378a10e` |
| T2 fix | `rules.write` switched to content-digest CAS: `base_revision` is a stable sha256 of current MEMRULES content, so `rules.read`→edit→`rules.write` round-trips under CAS without a synthetic revision row | `fa9c54ca` |
| T3 | Sync providers (`provider.go`): `Provider` with `ID() = "vivy.memory.bml"` serving both ports; `Query` recall → `Service.Search` → `Candidate` mapping per profile §record-envelope; `ObserveRunWithReceipt` ingests `run.completed` into a `history`-kind record via `Service.AppendHistory` — idempotent on EventID (deterministic record/receipt IDs), truthful `DeliveryReceipt` (`completed` only after commit, `failed` on error, identical receipt on redelivery) | `e511242b` |
| T4 | `vivy-memory` UI rewired to real backend: `view.tsx` now calls `module.action.invoke` against `vivy/memory-bml` via `memory-client.ts` (MaskClient precedent); demo path untouched for other views | `15d61bbf` |
| T4 fix | Neutral load-error banner instead of a crash-style surface when `module.action.invoke` fails | `93c89c3d` |

## SDD ledger

| Task | Scope | Review |
| --- | --- | --- |
| T1 | module skeleton + composition | Approved |
| T2 | control-action providers | Approved, 1 fix round (`rules.write` content-digest CAS) |
| T3 | recall + run-ingest providers | Approved; deferred nits: `defaultRecallLimit = 100` bounds search fan-out (hard ceiling not yet contract-pinned); `AppendHistory` redelivery returns `applied=false` without comparing replayed content to the stored record — a same-ID/different-payload redelivery is treated as a duplicate, noted for follow-up |
| T4 | UI rewire | Approved, 1 fix round (load-error banner) |

T1–T4 all Approved; 2 fix rounds total.

## Key rulings

- **Two records, one package** (plan constraint): the assembly emits one ctor
  per Port family, so the action plane (`vivy/memory-bml`) and the sync plane
  (`vivy/memory-bml-sync`) are separate catalog records backed by the same
  `internal/modules/memory` package.
- **No-arg constructors + `Active()`**: providers resolve the composition-owned
  service at call time and return explicit `unavailable`/`failed` outcomes when
  closed — never fabricated candidates or records.
- **No `std/status-source` provider**: the generated `RuntimeAssembly` has no
  status-provider field; status is the `vivy.memory.status` control action.
- **No `std/tool@v1` memory tools in this slice**: agent-requested
  recall/correction tools remain follow-up scope.

## Next (not done here)

MEM-2..5 remain Blocked on the gates named in
`docs/superpowers/plans/memory/index.md`.
