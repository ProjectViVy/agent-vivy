# MEM-1B — Agent-facing memory_* tools + MEM-1A defect fixes

Iteration: 2026-09-26. Scope: `docs/superpowers/plans/memory/MEM-1B.md` —
expose the BML store to the agent itself as six `std/tool@v1` providers
under a third catalog record `vivy/memory-bml-tools`, and land the six
cheap defect fixes from the MEM-1A whole-branch review in the same pass.
Executed as subagent-driven development: two tasks, each implemented and
independently reviewed; one fix round.

## What landed

| Task | Outcome | Commits |
| --- | --- | --- |
| T1 | Six MEM-1A review defects fixed in `internal/modules/memory`: (1) deterministically-undecodable `run.completed` events (empty RunID, malformed payload) now ack `DeliveryCompleted` with no write — the failure mode is permanent, and reporting `DeliveryFailed` wedged the cursor and all later ingest; store errors still report `DeliveryFailed`. (2) `vivy.memory.update` accepts `evidence_refs` and preserves the record's current evidence when the field is omitted (read-modify-write under the same `base_revision` CAS). (3) `sanitizeIDPart` collisions closed: deterministic ids now carry a 4-byte sha256 suffix of the raw RunID (`run-history-<san>-<sha8>-<seq>`), so `a/b` and `a\b` dedupe independently. (4) `Query` rejects host-controlled cursors whose offset+limit would exceed the store's uint32 bound instead of wrapping (`errInvalidRequest`). (5) `marshalResult` output-cap failure reports the snake_case `output_too_large` reason consistent with the contract vocabulary. (6) `module.Open` is idempotent — a second call or a race returns the existing service via `CompareAndSwap` instead of orphaning the first `Home` | `526b818c` |
| T2 | Six agent-facing `std/tool@v1` providers in `internal/modules/memory/tools.go` — `memory_add`, `memory_get`, `memory_list`, `memory_search`, `memory_update`, `memory_remove` — one `memoryTool` adapter per id backed by the composition-owned `Active()` service; third `boundRecord("vivy/memory-bml-tools", ...)` in `internal/modules/defaults/catalog.go` with `ProviderConstructor="ToolProviders"`, `ProviderCollection=true`, and `Requires core/tool-host@v1 → vivy/tool-host`; added to `recipes/default.vivy.yml`; generated assembly regenerated; conformance digests re-pinned | `709d7183` |
| T2 fix | `memory_list`/`memory_search` reject `limit=0` — a zero page size cannot advance and would emit empty pages whose `next_cursor` replays the incoming cursor forever | `1c32a14e` |

## SDD ledger

| Task | Scope | Review |
| --- | --- | --- |
| T1 | MEM-1A defect fixes | Approved |
| T2 | memory_* tool providers | Approved, 1 fix round (`limit=0` rejection) |

## Key rulings

- **Write effect is the MEM-0D authority surface.** `memory_add`,
  `memory_update`, and `memory_remove` declare `Effect: tool.EffectWrite`,
  which flips `domain.ToolSpec.Readonly=false` in `bindGeneratedTools` and
  routes them through the existing runtime approval gate — nothing extra to
  build. The three reads declare `EffectRead` and auto-execute.
- **Third record, same package.** `vivy/memory-bml-tools` is its own
  `boundRecord` whose `ToolProviders()` collection returns
  `[]tool.Provider`; it is not merged onto the sync or action records.
  Used-marking follows the `vivy/protected-tools` / `vivy/mcp-host`
  precedent: `Requires {PortRef: core/tool-host@v1, Provider: vivy/tool-host}`.
- **Closed enums per profile §6.** The schemas accept only `kind=long_term`,
  `trust=user_asserted`, `provenance=user_input`, and scope
  `machine-memory-home`; any other claimed value is rejected
  (`memory_invalid_request` / `kind_forbidden`), never silently rewritten —
  trust and provenance are authority-assigned by the store.
- **Shared outcome envelope.** `tool.Result.Text` carries the same
  `MemoryCrudOutcome` JSON as the control actions so agent and operator
  paths agree; reads extend it into `toolPage` with a `next_cursor`
  decimal-offset continuation (envelope extension, same shape).
- **Update `reason` is advisory-only.** The store records no removal-style
  reason on update; the field is accepted for upstream schema parity but not
  persisted.
- **Service resolved at call time.** No-arg constructors + `Active()`;
  a closed or absent store returns the explicit `bml_unavailable` outcome,
  never a panic or fabricated entries. Input capped at `maxActionInput`
  (32 KiB) before decode.

## Deferred (documented, not fixed in this slice)

- `memory_distill`: upstream it only files a pending governed Skill request;
  Vivy has no governed-request seam yet (MEM-3 territory). Same for the
  `actmem_*` family.
- The four remaining MEM-1A review nits: WriteRules check-then-act
  atomicity (needs Home-level locking design), `memory-bml-sync`-only
  recipe asymmetry, `AppendHistory` content-equality on same-ID redelivery,
  and the shared `ownerModule.Descriptor()` id — all latent or unreachable.
- `defaultRecallLimit = 100` remains a service constant, not yet
  contract-pinned (carried from MEM-1A).

## Next (not done here)

MEM-2..5 remain Blocked on the gates named in
`docs/superpowers/plans/memory/index.md`.
