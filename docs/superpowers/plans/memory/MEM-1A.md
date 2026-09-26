# MEM-1A Vivy runtime wiring for BML Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development when that method is selected. Steps use checkbox syntax.

**Goal:** Wire the `bml/` library into the Vivy runtime as a T1 module: bounded recall via `std/context-source@v1`, committed-experience ingestion via `std/observer/run@v1` (receipt-aware), explicit memory CRUD via `std/control-action@v1`, and a status action; rewire the `vivy/memory` UI view from demo stubs to `module.action.invoke`.
**Architecture:** New T1 package `internal/modules/memory` holds two catalog records — `vivy/memory-bml` (control-action collection) and `vivy/memory-bml-sync` (one multi-interface provider for context-source + run-observer). The BML store is opened at composition in `internal/app/app.go` (the `storagemodule.Open` precedent) and shared with providers through a package-level `Active()` registry. No new Port, Host, Journal, or generator changes.
**Spec:** [design](../../specs/2026-09-26-memory-providers-design.md) REQ-MEM-2,3,4,5,9; contracts: `docs/architecture/VIVY-MEMORY-PROFILE.md`, `VIVY-MEMORY-HOST-CONTRACTS.md`; readiness: `docs/research/2026-09-26-memory-g0-readiness.md`. State/dependencies: [index](index.md).
**Library:** `bml/` (module `github.com/ProjectViVy/agent-vivy/bml`) — `Home` facade (`NewHome(configDir)`, `Warmup`, `ListRecords`, `GetRecord`, `AddRecord`, `UpdateRecord`, `RemoveRecord`, `ReadMemRules`, `WriteMemRules`, `Close`); `Store.Search/SearchVisible(SearchQuery)`.

## Global Constraints

- **One `ProviderConstructor` per module record.** The generated assembly emits the same ctor call into every Port family the module provides. A `ProviderCollection` ctor returning `[]controlaction.Provider` cannot also satisfy `[]contextsource.Provider`, so the sync plane and the action plane MUST be two separate catalog records (`vivy/memory-bml` + `vivy/memory-bml-sync`). Do not try to merge them.
- **Two catalog records, one package.** Both `boundRecord`s point at `agent-vivy/internal/modules/memory`. The sync record's `NewProvider()` (non-collection) returns a single `*Provider` that implements `contextsource.Provider`, `observer.RunProvider`, and `observer.ReceiptRunProvider` — declare both port IDs as `vivy.memory.bml` so the one `ID()` satisfies both manifest lists.
- **No-arg constructors.** Generated code calls `ProviderConstructor()`. Stateful access flows through `internal/modules/memory.Open(ctx, cfg)` called once in app composition (after `storagemodule.Open`), which stores the service in a package-level `atomic.Pointer`; every provider resolves `Active()` at call time and returns an explicit unavailable/failed outcome when nil — never a fake success.
- **Sealed-manifest equality.** `buildRunSubscriptions` requires provider IDs == `manifest.RunObservers` and one `RunObserverPolicy` per provider (auto-derived: event types `run.completed/failed/cancelled`, allowed payload fields `outcome,view,summary,cause_category,message,reason,tenant_id,workspace_id,session_id`). `buildGeneratedContextHost` requires provider IDs == `manifest.ContextSources`. Provider `ID()` values must match the declared PortRef IDs exactly.
- **Used-provider marking.** `std/context-source` and `std/observer/run` providers are marked used only when the module `Requires` the consuming host port with the host provider named: add `Requires` `{PortRef: core/context-host@v1, Provider: vivy/context-host}` and `{PortRef: core/observer-host@v1, Provider: vivy/observer-host}` on the sync record (mirroring the `vivy/context-source` Requires precedent in `catalog.go`). Control actions are auto-marked used; no Requires needed for `vivy/memory-bml`.
- **Error contract.** All outcomes follow the profile's four-state vocabulary (`MemoryCrudOutcome{Status, Revision, Outcome}`); unsupported/missing-store paths return `unsupported`/`unavailable` explicitly — no silent no-ops, no emulated content. FTS5 text must go through `bml`'s escaping, never hand-built MATCH strings.
- **No `std/status-source` provider.** The generated `RuntimeAssembly` has no status-provider field (only `LanguageServerStatuses`); there is no runtime consumption path for module status sources. Expose status via the `vivy.memory.status` control action instead. Do NOT declare `std/status-source` in Provides — it would seal a manifest entry nothing constructs.
- **No `std/tool@v1` memory tools in this slice.** Agent-requested recall/correction tools are a follow-up; MEM-1A is recall + ingest + CRUD + UI.
- **Grants:** none requested. `rpc.client` is the only ceiling and actions run under the existing `module.action.invoke` client grant. No `grantApprovals` entries.
- **Recipe:** append `vivy/memory-bml` and `vivy/memory-bml-sync` to `modules:` in `recipes/default.vivy.yml`. They are not Ordered ports — no `order:` entry needed.
- **Regen:** `go generate ./internal/generated/assembly` after catalog edits (or the repo's generator entrypoint).
- **Air gap:** never read/write `data/` from agent sessions; the running app owns its data dir.
- `gofmt`/`go vet`/`go test` green in `internal/modules/memory`, `internal/modules/defaults`, `internal/app`; `just ci` green for the final gate.

## Task 1: memory module skeleton + composition wiring

**Files:**
- Create: `internal/modules/memory/module.go` — `NewModule()` owner `Construct` for both records; `Open(ctx, cfg)` composition entry: `bml.NewHome(cfg.DataDirectory())` → `Warmup` → store in `active atomic.Pointer[Service]`; `Active() *Service`; `Close()`.
- Create: `internal/modules/memory/service.go` — `Service` wrapping `*bml.Home`: `List/Get/Search/Add/Update/Remove/Rules/Status` methods returning contract DTOs; maps bml errors (`bml_unavailable`, `memory_revision_conflict`, `memory_not_found`, `memory_invalid_request`) to outcome statuses without leaking Content into error strings.
- Modify: `internal/modules/defaults/catalog.go` — two `boundRecord`s + Binding/`Requires` wiring in the switch blocks.
- Modify: `recipes/default.vivy.yml` — add both modules to `modules:`.
- Modify: `internal/app/app.go` — open the memory service after storage open (only when `vivy/memory-bml` is in the compiled manifest), close on shutdown.
- Modify: root `go.mod` — `require github.com/ProjectViVy/agent-vivy/bml v0.0.0` + `replace github.com/ProjectViVy/agent-vivy/bml => ./bml`.
- Regen: `internal/generated/assembly/zz_default.go` via `go generate`.
- Test: `internal/modules/memory/module_test.go` — Open on `t.TempDir()`, Active/Close lifecycle, unavailable-outcome when closed.

- [ ] Implement T1. Verify: `go build ./internal/modules/memory ./internal/app`; `go test ./internal/modules/memory`.

## Task 2: control-action providers

**Files:**
- Create: `internal/modules/memory/actions.go` — `ActionProviders() []controlaction.Provider`; one provider per action, `Definition()` + `Invoke(ctx, host, input)` resolving `Active()`; reuse the masks caps (`maxActionInput = 32<<10`, `maxActionOutput = 256<<10`).
- Actions: `vivy.memory.list` {limit?}, `vivy.memory.search` {query, limit?}, `vivy.memory.get` {id}, `vivy.memory.add` {kind, content, evidence?}, `vivy.memory.update` {id, content, base_revision}, `vivy.memory.remove` {id, base_revision, reason}, `vivy.memory.rules.read`, `vivy.memory.rules.write` {content}, `vivy.memory.status` {}.
- Each action validates input size/shape, calls Service, returns the 4-state outcome JSON; `update`/`remove`/`rules.write` carry `base_revision` CAS.
- Test: `internal/modules/memory/actions_test.go` — invoke each action against a temp-dir store; conflict paths return `conflict`/`failed`, not errors-as-success.

- [ ] Implement T2. Verify: `go test ./internal/modules/memory`; manifest `Actions` list == declared PortRef IDs == provider `Definition().ID`s.

## Task 3: sync providers (context-source recall + run-observer ingest)

**Files:**
- Create: `internal/modules/memory/provider.go` — `NewProvider() *Provider`; `ID() = "vivy.memory.bml"` for both ports; `Query(ctx, contextsource.Request)` → `Service.Search` → `Candidate` mapping per profile §record-envelope (SourceID, ContentID=record id, Version=record revision, Resource, UpdatedAt, Confidence, Metadata namespaced `vivy.memory-bml.`); cursor/limit honoring.
- `ObserveRunWithReceipt(ctx, RunEvent)` — on `run.completed`: extract allowed payload fields (`summary`, `session_id`, `workspace_id`, `tenant_id`) → append a `history`-kind record with evidence `run:<run_id>`; **idempotent on EventID** — dedupe via the store's apply-journal mechanism (or equivalent `bml` API; if none exists, record the event id in record metadata and check-before-write). Return `DeliveryReceipt{EventID, State}` truthfully (`completed` only after the write commits; `failed` on error; same receipt for a duplicate delivery). `ObserveRun` delegates to the receipt path.
- Test: `provider_test.go` — recall returns Candidates matching declared fields; duplicate `ObserveRunWithReceipt` for the same EventID writes once and returns identical receipt; failed store → `failed` receipt.

- [ ] Implement T3. Verify: `go test ./internal/modules/memory`; `go build ./internal/app` (assembly wiring compiles through generated code).

## Task 4: UI rewire to real backend

**Files:**
- Modify: `plugins/vivy-memory/ui/vivy-memory/src/view.tsx` — replace `getDemoMemories()` with `host.rpc` calls: `module.action.invoke` `{module_id: "vivy/memory-bml", action_id: "vivy.memory.list"|"vivy.memory.search", input: {...}}` — follow `plugins/vivy-masks-ui/ui/vivy-masks/src/mask-client.ts` (`MaskClient.fromRPC(host.rpc)`) precedent; keep MasterDetail layout, search box drives `vivy.memory.search`.
- Keep `ui/src/lib/demo-api.ts` untouched (other demo views still use it); do not delete the demo path — the memory view just stops calling it.
- Test: `plugins/vivy-memory/ui/vivy-memory/src/view.test.tsx` — mock rpc `module.action.invoke` like `MaskPage.test.tsx`.

- [ ] Implement T4. Verify: `cd plugins/vivy-memory/ui/vivy-memory && pnpm test` (or repo UI test command); `cd ui && pnpm -s typecheck` if the plugin compiles into the host build.

## Task 5: gates + iteration log

- [ ] `gofmt -l internal/modules/memory` clean; `go vet ./internal/modules/memory ./internal/modules/defaults ./internal/app`; `go test ./internal/modules/...`.
- [ ] `just ci` (module/conformance pressure suites included).
- [ ] Write `docs/logs/2026-09-26-memory-1a-wiring/{summary,verification,acceptance}.md` per repo log convention.
- [ ] Update `docs/superpowers/plans/memory/index.md` MEM-1A row → Done.

## Review Focus

- **Manifest sealing:** generated `ContextSources`/`RunObservers`/`Actions` must match declared PortRef IDs exactly; a mismatch fails at `Start`/validation, not silently.
- **Observer idempotency:** a redelivered EventID must not double-write a history record.
- **Truthful degradation:** `Active()==nil` or store error → explicit `failed`/`unavailable` outcome or `failed` receipt — never fabricated candidates/records.
- **CAS discipline:** `update`/`remove` must pass `base_revision`; a stale revision returns `conflict`, never a forced overwrite.
- **No Eino leakage:** memory package imports only `bml` + `sdk/port/*` + `internal/moduleport` types — no `cloudwego/eino*` imports (AGENTS.md quarantine; Eino capability check: none needed — recall is local FTS5, ingestion is event-driven writes; no LLM components involved).
