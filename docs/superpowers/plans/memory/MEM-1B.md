# MEM-1B Agent-facing memory_* tools + MEM-1A defect fixes Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development when that method is selected. Steps use checkbox syntax.

**Goal:** Expose the BML store to the agent itself as `std/tool@v1` providers (`memory_add`, `memory_get`, `memory_list`, `memory_search`, `memory_update`, `memory_remove`) so a run can read and write memory through the ordinary governed tool path; fix the six cheap MEM-1A final-review defects in the same pass.
**Architecture:** One more catalog record in `internal/modules/memory` — `vivy/memory-bml-tools` — with a `ProviderCollection` constructor `ToolProviders()` returning one `tool.Provider` per tool id. Same package, same `Active()` service registry; no new Port/Host/generator changes. Write-effect tools (`add`/`update`/`remove`) declare `Effect: "write"` which flips `domain.ToolSpec.Readonly=false` and routes them through the existing runtime approval gate — that IS the MEM-0D authority surface, nothing extra to build.
**Spec:** [design](../../specs/2026-09-26-memory-providers-design.md) REQ-MEM-3 (agent mutation path); upstream tool family reference `agent-diva-tools/src/memory_{add,get,list,search,update,remove,distill}.rs`. State/dependencies: [index](index.md).

## Global Constraints

- All MEM-1A Global Constraints still apply (one ctor per record, no-arg ctors + `Active()`, sealed-manifest equality, four-state outcomes, no silent failures, FTS5 escaping only).
- **Third record, same package.** `vivy/memory-bml-tools` must be its own `boundRecord` — its `ToolProviders()` collection returns `[]tool.Provider` and cannot be merged onto the sync or action records.
- **Used-marking.** Tool providers are marked used only via `Requires` `{PortRef: core/tool-host@v1, Provider: vivy/tool-host}` (the `vivy/protected-tools` / `vivy/mcp-host` precedent).
- **Tool ids are the PortRef ids** and must be snake_case tool names matching upstream Diva: `memory_add`, `memory_get`, `memory_list`, `memory_search`, `memory_update`, `memory_remove`. They must not collide with `tools.AssemblyControlledToolNames()` (protected ids) or any existing tool.
- **`memory_distill` is OUT of scope.** Upstream it only creates a pending governed Skill request; Vivy has no governed-request seam yet (MEM-3 territory). Same for the `actmem_*` family.
- **Effect discipline.** `memory_add`, `memory_update`, `memory_remove` → `Effect: tool.EffectWrite` (approval-gated). `memory_get`, `memory_list`, `memory_search` → `Effect: tool.EffectRead` (auto-execute).
- **Schemas.** Each `Definition.Schema` is a JSON Schema object matching the bml/Home call shapes (id/content/kind/trust/scope/limit/cursor/reason). Keep them minimal — required fields only.
- **Result shape.** `tool.Result{Text}` carries the same JSON envelope as the control actions (`MemoryCrudOutcome` for writes; entries array for reads) so agent and operator paths agree; errors map to the profile vocabulary (`bml_unavailable`, `memory_not_found`, `memory_revision_conflict`, `memory_invalid_request`), never leaked bml internals.
- **Caps.** Reuse `maxActionInput`/`maxActionOutput` byte caps from actions.go (or shared constants); reject oversized args up front.
- **Recipe:** append `vivy/memory-bml-tools` to `modules:` in `recipes/default.vivy.yml`; regenerate `internal/generated/assembly/zz_default.go` and the UI staged assembly if it changes.
- **Conformance digest:** any `internal/` change moves the internal source digest — re-pin the five internal-rooted `sourceSha256` rows in `sdk/internal/assembly/conformance_results.json` via `go run ./sdk/internal/cmd/source-hash internal ""` and do NOT run `go mod tidy` (it drops the generated-module requires restored in `2e65f162`).

## Task 1: fix MEM-1A final-review defects (cheap subset)

**Files:**
- Modify: `internal/modules/memory/provider.go`, `service.go`, `actions.go` as needed; `bml/` only if a fix genuinely belongs in the library.

**Fix list (from the whole-branch review):**
- [ ] 1. **Poison event wedge** (`provider.go` `ObserveRunWithReceipt`): a permanently-undecodable `run.completed` (malformed payload JSON / missing required field) currently returns `DeliveryFailed` forever — the host never abandons delivery, so one bad event wedges the cursor and all later ingest. Fix: deterministically-undecodable events return `DeliveryCompleted` (ack, no write) — the failure mode is permanent, retry cannot help.
- [ ] 2. **`vivy.memory.update` clears `evidence_refs`** (`actions.go`/`service.go`): `updateInput` lacks an evidence field and `Home.UpdateRecord` assigns nil. Fix: accept `evidence_refs` on the input and pass it through; when omitted, preserve the existing record's evidence (read-modify-write), don't blank it.
- [ ] 3. **`sanitizeIDPart` collision** (`provider.go`): `a/b` and `a\b` sanitize identically → cross-run dedupe collision. Fix: append a short hash suffix of the raw RunID to the sanitized id (e.g. `run-history-<san>-<sha8(raw)>`); keep ids deterministic.
- [ ] 4. **`Query` offset+limit overflow** (`provider.go`): host-controlled decimal cursor parsed into `int` can wrap. Fix: clamp/validate before arithmetic (reject cursor > maxInt-safety or cap offset at len(hits)).
- [ ] 5. **`marshalResult` reason vocabulary** (`actions.go`): output-cap failure reason must be snake_case like the rest (`memory_invalid_request` or a dedicated `output_too_large` — pick one consistent with the contract doc).
- [ ] 6. **`Open` idempotency** (`module.go`): second `Open` orphans the first `Home`. Fix: if `Active() != nil`, close-or-reuse deterministically (prefer: return the existing service).

**NOT fixing in this slice (documented deferrals, note in iteration log):** WriteRules check-then-act atomicity (needs Home-level locking design), memory-bml-sync-only recipe asymmetry, AppendHistory content-equality, shared `ownerModule.Descriptor()` id — all latent or unreachable.

**Verification:** `go test ./internal/modules/memory -count=1` — new tests: poison-event ack (malformed payload → completed receipt, no record, cursor advances), update preserves evidence_refs when omitted + applies when given, colliding sanitized ids dedupe separately, absurd cursor rejected cleanly. `go vet`/`gofmt` clean.

## Task 2: memory_* std/tool providers

**Files:**
- Modify: `internal/modules/defaults/catalog.go` — third `boundRecord("vivy/memory-bml-tools", ...)` providing six `port("std/tool@v1", <id>)` entries; binding `ProviderConstructor="ToolProviders"`, `ProviderCollection=true`; `Requires` `core/tool-host@v1 → vivy/tool-host`.
- Create: `internal/modules/memory/tools.go` — `ToolProviders() []tool.Provider`; one provider per tool backed by `Active()`; `Definition()` returns `{ID, Description, Effect, Schema}`; `Invoke` decodes args (size-capped), calls `Service`, returns `tool.Result{Text: <envelope JSON>}`; unavailable service → `bml_unavailable` error result, not a panic/fake.
- Create: `internal/modules/memory/tools_test.go` — definition ids/effects/schemas; invoke happy paths against a temp bml store; approval-visible write effect; unavailable path.
- Modify: `recipes/default.vivy.yml` — add `vivy/memory-bml-tools` to `modules:`.
- Regenerate: `go generate ./internal/generated/assembly`; re-stage UI assembly if it diffs; re-pin `conformance_results.json` internal digest (see Global Constraints); update `internal/app/default_generation_test.go` / `sdk/internal/frontend/testdata` goldens if the sealed inventory changed.

**Behavior:**
- `memory_search {query, limit?, cursor?}` → entries JSON (read).
- `memory_list {kind?, scope?, limit?, cursor?}` → entries JSON (read).
- `memory_get {id}` → entry JSON (read).
- `memory_add {kind, content, trust?, provenance?, evidence_refs?}` → `MemoryCrudOutcome` (write).
- `memory_update {id, content?, base_revision, evidence_refs?, reason?}` → outcome (write).
- `memory_remove {id, base_revision, reason}` → outcome (write).

**Verification:** `go test ./internal/modules/memory -count=1`; `go build ./internal/generated/assembly ./internal/app`; manifest seal — `manifest.Tools` == generated provider set exactly; `go run ./sdk/internal/cmd/generate-default` output consistent; `go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1` green after re-pin (needs `node`+`pnpm` on PATH for the fixture/full-ui go-subprocess check).

## Task 3: gates + iteration log + index

- [ ] `gofmt -l` clean on all touched files; `go vet` clean.
- [ ] `go test ./internal/modules/memory ./internal/app ./internal/modules/defaults -count=1` green.
- [ ] `cd bml && go test ./...` green (untouched unless a fix required it).
- [ ] Controller-run `just ci` green end-to-end.
- [ ] Iteration log `docs/logs/2026-09-26-memory-1b-tools/{summary,verification,acceptance}.md` — record fixed defects, deferred nits, tool surface.
- [ ] `docs/superpowers/plans/memory/index.md` — MEM-1B row → Done.
- [ ] Ledger `.superpowers/sdd/MEM-1B/progress.md` complete (gitignored, session-local).

## Review Focus

- Grant/authority: write tools must surface `Effect: write` → `Readonly=false` so the approval gate fires; verify a write tool cannot bypass approval (check `bindGeneratedTools` path).
- No shadowing: the six ids must not collide with protected ids or existing generated/legacy tools (`bindGeneratedTools` errors on dupes — good, assert the seal).
- Poison-event ack must not silently drop *retryable* failures (store errors still `DeliveryFailed`).
- Evidence preservation semantics: omitted ≠ cleared.
- No Eino imports outside quarantine; no `data/` writes; authors mastwet only.
