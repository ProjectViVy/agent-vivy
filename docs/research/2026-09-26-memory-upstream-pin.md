# Memory upstream pin and drift reconciliation

MEM-0A (G0) research note for issue #33. Pins every upstream contract the
issue cites to an exact revision and reconciles documented drift between
upstream docs and upstream code. Evidence = inspected file lines at the
pinned revision; anything not directly inspected is marked *inferred*.

## Pinned revisions

| Upstream | Pinned sha | Commit date | Branch inspected | URL |
| --- | --- | --- | --- | --- |
| `ProjectViVy/agent-diva` | `0fd005a105d8987df02ae7b796a3c591e04b9ca3` | 2026-08-21 | `main` (`origin/HEAD`; matches issue pin `0fd005a1`) | https://github.com/ProjectViVy/agent-diva/tree/0fd005a105d8987df02ae7b796a3c591e04b9ca3 |
| `ProjectViVy/laputa` | `1e402835912009fd19fed6442f56084e98a59b12` | 2026-09-26 | `main` (`origin/HEAD`) | https://github.com/ProjectViVy/laputa/tree/1e402835912009fd19fed6442f56084e98a59b12 |
| `mem0ai/mem0` | `94c3fe9f238f3dbf29c9ce98643bd71eb13077cd` | 2026-09-25 | `main` (`HEAD`; release bump — Python SDK 2.2.1, TS SDK 3.3.1) | https://github.com/mem0ai/mem0/tree/94c3fe9f238f3dbf29c9ce98643bd71eb13077cd |
| `NevaMind-AI/memU` | `2c050bc9681a4c0aff1af211a000e73d14f33356` | 2026-09-21 | `main` (`HEAD`) | https://github.com/NevaMind-AI/memU/tree/2c050bc9681a4c0aff1af211a000e73d14f33356 |
| `TencentCloud/TencentDB-Agent-Memory` | `bd88cc83870bf9e7dbd2ec36aa13608d2295c7f4` | 2026-09-26 | `main` (`HEAD`) | https://github.com/TencentCloud/TencentDB-Agent-Memory/tree/bd88cc83870bf9e7dbd2ec36aa13608d2295c7f4 |

Notes on the pins:

- agent-diva `main` HEAD is exactly `0fd005a1`, so the issue's reference
  baseline is still current. The clone's `CLAUDE.md` (line 43) describes a
  divergent working branch `agent-diva-pro`; that branch is not on the
  remote (`origin` lists only `main`, `dev`, `oldmain`) — the pin on `main`
  is correct, but the pro-line divergence the plan flags is real and local.
- laputa was cloned fresh for this note; `1e402835` is `main` HEAD at
  inspection time (2026-09-26).
- The three third-party repos are pinned at remote `HEAD` at inspection
  time. mem0's hosted docs site (`docs.mem0.ai`) is rolling and unpinned;
  the repo pin covers the SDK surface.

## Task 1 — agent-diva BML pin and inventory

Local clone: `~/reference/agent-diva` @ `0fd005a1`.

### BML surface inventory

The BML logical layer is `agent-diva-laputa::bml` (a namespace re-exporting
the storage core per `bml/mod.rs:22-34`). Exported surface:

| Element | Evidence (`agent-diva-laputa/src/` unless noted) |
| --- | --- |
| `MemoryHome` facade — sole production memory authority | `bml/mod.rs:5`, `bml/mod.rs:24`; struct at `bml/memory_home.rs:86` |
| `MemoryHomeError` codes (`bml_unavailable`, `memory_revision_conflict`, `memory_kind_forbidden`, `memory_not_found`, `memory_invalid_request`, `memory_io_error`) | `bml/memory_home.rs:39-70` |
| `MemRulesDocument` / `MemRulesSource{Default,File}` | `bml/memory_home.rs:72-83`, re-exported `bml/mod.rs:24` |
| `TypedMemoryStore`, `StoredMemoryRecord`, `MemorySearchHit`, `MemoryStoreMetadata`, `MemoryStoreIntegrity`, `WorkspaceIdentityMigration{Manifest,State}`, `MAX_MEMORY_CONTENT_BYTES`, `MAX_MEMORY_RECORDS` | `bml/mod.rs:31-35`; `StoredMemoryRecord` at `typed_store.rs:96` |
| Offline-migration adapter vocabulary (`adapt_laputa_section`, `adapt_legacy_markdown`, `MemoryMigrationPlan/Manifest`, `MemoryRollbackManifest`, `MemoryRecordMigration`, `MemoryAdapterContext/Output`, `compare_normalized_records`) | `bml/mod.rs:26-30`, `memory_records.rs` |
| FTS5 schema: `schema_meta`, `memory_records`, `memory_supersedes`, `memory_apply_journal`, `memory_fts` (`fts5`, `tokenize='unicode61'`) | `typed_store.rs:494-561` |
| Store-wide revision counter + per-record `record_revision` CAS | `typed_store.rs:494-501`, `memory_records.record_revision` at `typed_store.rs:508`; CAS callers `memory_home.rs:277`, `memory_home.rs:341` |
| Scope model `MemoryScope{tenant_id, workspace_id, session_id}` | `agent-diva-core/src/memory/record.rs:78-82` |
| Provenance `MemoryProvenance{source, source_id, content_digest, captured_at, correlation}`; `MemoryProvenanceSource` incl. `LaputaAppliedSection`, `UserInput`, `SessionSync`, `AutoDream` | `record.rs:86-92`, `record.rs:38-49` |
| Record kinds incl. `Identity…LongTerm…SessionCheckpoint` with `#[serde(other)] Unknown` | `record.rs:17-33` |
| Revision/tombstone semantics: `MemoryTombstone{target_record_id, reason_digest, actor_id, created_at}`; `supersedes` list; validation rules `TombstoneContainsContent`, `TombstoneTargetMismatch`, `SelfSupersedes` | `record.rs:96-101`, `record.rs:118-119`, `record.rs:180-184` |
| Trust/sensitivity axes `MemoryTrust` (incl. `AppliedAuthority`), `MemorySensitivity` | `record.rs:54-74` |
| `MemoryProvider` trait (read side `system_prompt_block`/`prefetch`/`sync_turn`; CRUD `memory_add`/`list`/`get`/`search`/`update`/`remove`; ACTMEM ops; `memory_rules`; session checkpoint; `on_session_end`; `memory_distill`) | `agent-diva-core/src/memory/provider.rs:418-620` |
| L1 startup index (bounded pointer-only injection, `DEFAULT_L1_INDEX_LINES = 30`) | `record.rs:314-349`; cached projection in `memory_home.rs:394-418` |

### Governed-apply vs ordinary mutation — proof

Issue claim: "ordinary memory mutation is separate from persona
governed-apply." **Confirmed** at the pin:

- The `bml` module doc states it outright: "The cognitive clean break
  removed the governed-apply pipeline: memory CRUD is direct and
  unapproved" (`bml/mod.rs:9-11`).
- Ordinary call chain: tool `MemoryAddTool` → `provider.memory_add(...)`
  (`agent-diva-tools/src/memory_add.rs:78`) → `MemoryHome::memory_add`
  (`memory_home.rs:583-605`) → `add_record` → `store.put(record,
  metadata.store_revision, None)` (`memory_home.rs:233`). `memory_update`
  and `memory_remove` land on the same ungoverned `put`/`put_tombstone`
  path (`memory_home.rs:277`, `memory_home.rs:341`). No approval, no
  proposal, no journal row.
- The governed seam still exists in storage but is dead code:
  `TypedMemoryStore::put_governed` (`typed_store.rs:933`) is doc-commented
  "This seam remains unregistered until the GMH-24 write cutover";
  `rollback_governed` (`typed_store.rs:952-999`) deletes by
  `memory_apply_journal.proposal_id`. A repo-wide grep at the pin finds
  zero production callers — the only references are the definitions, the
  `bml/mod.rs:16` note ("storage-core internal and has no callers"), and
  `tests/bml_boundary_guard.rs:7-15`, which lists `.put_governed(` /
  `.rollback_governed(` as *forbidden* patterns for non-BML modules.
- Persona change is a separate file-based flow, not a BML write:
  `PersonaService::create_request` (`persona/service.rs:259-321`) writes a
  `PersonaChangeRequest` in `Pending` state
  (`persona/types.rs:229-248`); a human-facing `accept_request`
  (`service.rs:346-385`) is what applies the Markdown document.
  `PersonaService` roots at `config_dir/persona/` (`service.rs:26-28`),
  not the memory database. `MemoryRecordKind::Identity` is even rejected
  from production BML (`memory_home.rs:220-222`, test
  `memory_home.rs:1066-1076`).

Adjacent stale doc: `README.md:278-282` still advertises Laputa as
"proposals, governed apply, rollback, audit" over BML — the governed apply
and rollback store path described there is the retired seam above.

### MemoryHome path drift

Issue claim: README describes profile-local `.laputa/` while code uses
`config-dir/memory/memory.sqlite3`. **Confirmed — README/AGENTS are
stale, code wins:**

- Resolved path in code: `MemoryHome::with_l1_budget` builds
  `memory_dir = config_dir.join("memory")` and
  `database = memory_dir.join("memory.sqlite3")`
  (`bml/memory_home.rs:107-111`); the module doc says
  "`{config_dir}/memory/memory.sqlite3`" (`bml/mod.rs:4-5`).
- `README.md:274-276` and `AGENTS.md` still call the production authority
  the profile-local `.laputa/memory.sqlite3`.
- `.laputa/memory.sqlite3` survives only as a legacy *offline migration
  source*: `LaputaPaths::memory_database()` is commented "Legacy workspace
  typed memory database (offline migration source only)"
  (`layout.rs:47-50`).
- The boundary is tested: `paths_are_machine_home_and_database_is_lazy`
  creates a stray `workspace/.laputa/memory.sqlite3` and asserts it is
  ignored while the real DB lands at `config/memory/memory.sqlite3`
  (`memory_home.rs:1022-1038`).

## Task 2 — laputa pin and inventory

Local clone: `~/reference/laputa` @ `1e402835` (2026-09-26, `main` HEAD).

### Module map

- Three Go modules at repo root: `laputa/` → `github.com/dashimaki/laputa`,
  `mentle/` → `github.com/dashimaki/mentle`, `garden/` →
  `github.com/dashimaki/garden` (each module's `go.mod` line 1; the Go
  namespace is `dashimaki`, not the GitHub org `ProjectViVy`).
- `mentle/README.md:1` titles the module `# mempalace-go` — the
  mempalace-go name is retained; the README describes a Go port of
  MemPalace exposing memory operations as MCP tools over stdio
  (`mentle/README.md:5-9`).
- `garden/internal/recall/fast.go` does depend on Mentle facade types:
  `import "github.com/dashimaki/mentle/facade"` (`fast.go:12`), the
  `Searcher` interface is defined in facade terms
  (`fast.go:16-17`), the view carries `facade.MemoryCard` /
  `facade.EvidenceFragment` (`fast.go:38-39`), and the query path calls
  `Searcher.SearchCards(ctx, facade.CardQuery{...})` and
  `Searcher.ReadEvidence(ctx, facade.EvidenceQuery{...})`
  (`fast.go:96`, `fast.go:121`). Confirmed.

### GovernedService.Mutate readiness — re-inspection result

**Changed: the code the issue reviewed no longer exists at the pin.**

- `laputa/governance/` contains no Go files at all — only `AGENTS.md`
  planning docs under `governance/`, `governance/rhythm/`,
  `governance/scheduler/`, `governance/store/`, `governance/wakeup/`,
  `governance/web/`.
- Repo-wide grep at `1e402835` finds no `GovernedService`, no `func
  Mutate`, and no governed `RollbackRef`. The only `RollbackRef` left is
  `HostArtifact.RollbackRef`, a JSON field on an evolution host-artifact
  record (`garden/internal/evolution/bundle.go:87`) — unrelated to
  governed-state rollback.
- The removal is deliberate and guarded: ADR-0013 "Supersedes as
  implementation guidance: the current `laputa/governance` JSON section
  engine … generic governance HTTP mutation routes"
  (`docs/architecture/0013-…:8`), and
  `garden/internal/architectureguard/scanner.go:134-168` fails the build on
  `laputa/governance` imports, `NewGovernedService`, `WorldProjector`,
  `WorldClaim`, and `/v2/governance` routes.
- The replacement surface is the Markdown persona authority
  `laputa/persona` (`service.go`, `types.go`): `KindWorld`/`WORLD.MD`
  exists as a document kind (`types.go:29`, `types.go:76`) but has no
  Frozen Core projection (`types.go:107`), `Service` keeps per-kind
  `history/` and a `requests/` review queue (`service.go:30`,
  `service.go:64-65`), and revisions are content-hash + revision checked
  (`service.go:150-158`, `service.go:215-246`).

Consequence for G0: the issue's atomicity/audit findings
(state-write-before-audit-append, hash-only rollback) were made against
the deleted JSON governance engine. They do not transfer to
`laputa/persona` — that surface needs its own readiness verdict
(MEM-0D input): Markdown writes plus JSONL history, whether a failed
history append leaves a mutated document, and whether `requests/`
acceptance is atomic with document write. If a governed Mutate is still
needed, it must be re-pointed at whatever replaces the retired engine;
there is no current implementation to re-verify.

### Cognitive-partition ADR status

- The cited file `docs/architecture/0002-laputa-cognitive-partition-decision.md`
  is **archived**: it now lives at
  `docs/archive/2026-08-14-laputa-clean-break/0002-laputa-cognitive-partition-decision.md`
  (status accepted, dated 2026-08-02). Active ADRs start at 0006.
- ADR-0012 "Laputa Markdown Clean Break and EvoMap Capability Boundary"
  (accepted, 2026-08-14) explicitly supersedes ADR-0002 as active guidance
  (`docs/architecture/0012-…` header block).
- MEMRULES/WORLD status against shipped code: `WORLD.MD` shipped under the
  new semantics — it is a persona document kind readable only by explicit
  tool read, never auto-projected (ADR-0012:50, ADR-0013:101/110;
  `laputa/persona/types.go:107` confirms no Frozen Core projection).
  `MEMRULES.MD` did **not** ship: zero `.go` references repo-wide;
  ADR-0013:128 leaves it conditional ("If `MEMRULES.MD` survives a later
  implementation decision… Garden policy input"). Net: the partition's
  Markdown-authority half is implemented; the MEMRULES policy file remains
  a documented option, not code.

## Task 3 — third-party provider doc pins

| Provider | Pinned ref | Doc evidence |
| --- | --- | --- |
| mem0 | `mem0ai/mem0@94c3fe9` (2026-09-25); hosted docs at `docs.mem0.ai` unpinned | Polyglot monorepo: `mem0/` Python SDK, `mem0-ts/` TS SDK (`src/client` hosted `MemoryClient`, `src/oss` self-hosted `Memory`), `server/` FastAPI self-host — OSS and hosted surfaces are distinct, matching the issue's "separate OSS and hosted behavior" boundary. |
| memU developer contract | `NevaMind-AI/memU@2c050bc9`, `docs/developer.md` | Lifecycle `prepare → one external executor → commit` (`developer.md:9-15`). v1 constraints confirmed verbatim: input is "1–10 completed sessions" supplied by the application; selecting from an ongoing conversation is out of scope (`developer.md:3-5`); "Version 1.0 permits one active developer run at a time" at the fixed `~/.memu/developer` workspace (`developer.md:19-31`, restated `developer.md:192`, `developer.md:277`); strict canonical item model rejecting unknown fields (`developer.md:72`); no abort/discard command and no conflict resolution in v1 (`developer.md:269-279`). |
| TencentDB Agent Memory | `TencentCloud/TencentDB-Agent-Memory@bd88cc83` | Repo ships `memory-core` + `memory-hub` + `proxy` services (`README.md:33-45`); advertised integration model is base-URL model proxy ("One Proxy, unchanged protocol, zero-code integration", `README.md:55-76`) plus `MemoryProxy/v3-api-memoryproxy-doc.md` — relevant because the issue/design reject the transparent-proxy route in favor of explicit memory/asset APIs. |

## Task 4 — drift reconciliation

| # | Issue claim | Pinned evidence | Status |
| --- | --- | --- | --- |
| 1 | Diva pin `0fd005a1` is the BML reference baseline | `agent-diva` `main` HEAD = `0fd005a105d8…` (2026-08-21) | confirmed — still `main` HEAD |
| 2 | BML = typed SQLite + FTS5, direct memory CRUD | `bml/mod.rs:4-13`, `typed_store.rs:494-561` | confirmed |
| 3 | Ordinary memory mutation separate from persona governed-apply | tool→`memory_*`→`store.put` chain (`memory_add.rs:78`, `memory_home.rs:233/277/341`); persona `create_request`→`accept_request` file flow (`persona/service.rs:259`, `:346`); `put_governed` unregistered (`typed_store.rs:932`) | confirmed — and stronger than the issue states: the governed *memory* seam is now dead code, not just separate |
| 4 | MemoryHome uses `config-dir/memory/memory.sqlite3`; README `.laputa/` is stale | `memory_home.rs:107-111` vs `README.md:274-276`, `AGENTS.md`; legacy path = migration source only (`layout.rs:47-50`, test `memory_home.rs:1022-1038`) | confirmed — README/AGENTS stale at pin |
| 5 | Laputa modules are `laputa`, `mentle`, `garden` | three `go.mod`s (`github.com/dashimaki/{laputa,mentle,garden}`) | confirmed |
| 6 | Mentle README retains `mempalace-go` name | `mentle/README.md:1` | confirmed |
| 7 | Garden `fast.go` uses Mentle facade types | `garden/internal/recall/fast.go:12,16-17,38-39,96,121` | confirmed |
| 8 | `laputa/governance/governed.go`: `Mutate` writes state before audit append, ignores append error; `RollbackRef` is a state hash | File and symbols absent at `1e402835`; `laputa/governance/` is docs-only; only `HostArtifact.RollbackRef` remains (`garden/internal/evolution/bundle.go:87`); deletion enforced by `architectureguard/scanner.go:134-168` | **changed** — governed engine deleted by the ADR-0012/0013 clean break; the readiness defect is moot upstream but the *replacement* persona surface is unvetted for the same properties |
| 9 | Cognitive-partition ADR (0002) target MEMRULES/WORLD semantics; "not proof migration shipped" | ADR-0002 moved to `docs/archive/2026-08-14-laputa-clean-break/`, superseded by ADR-0012 (accepted 2026-08-14); WORLD.MD shipped tool-only (`persona/types.go:107`); MEMRULES.MD absent from code, conditional per ADR-0013:128 | **changed** — link stale (archived); partition semantics partially shipped: WORLD yes (tool-only), MEMRULES no |
| 10 | memU v1: completed sessions in; one active developer run per workspace | `docs/developer.md:3-5`, `:19`, `:192`, `:277` @ `2c050bc9` | confirmed |
| 11 | TencentDB integration = prefer explicit APIs, not transparent model proxy | Repo advertises proxy-first model (`README.md:55-76`) @ `bd88cc83`; explicit API surface exists under `MemoryProxy/v3-api-memoryproxy-doc.md` | confirmed as upstream posture — Vivy's direct-API requirement remains an adapter concern |
| 12 | mem0 as the pinned remote provider | `mem0ai/mem0@94c3fe9` (SDK 2.2.1 / TS 3.3.1 at pin) | pinned; OSS vs hosted split verified via repo layout (`mem0-ts/src/{client,oss}`) — capability detail deferred to MEM-2 |

## Residual notes for downstream stories

- MEM-0D's Laputa readiness verdict must target `laputa/persona` (Markdown
  authority + requests queue) or explicitly scope to an older sha — the
  governed engine the issue reviewed is gone.
- The `dashimaki/*` Go module namespace means laputa cannot be imported
  under `github.com/ProjectViVy/*` paths without a `replace` or upstream
  module-path change (*inferred* consequence; no Vivy import attempted).
- agent-diva `agent-diva-pro` divergence is real per `CLAUDE.md:43` but is
  not on the remote; the `main` pin stands.
