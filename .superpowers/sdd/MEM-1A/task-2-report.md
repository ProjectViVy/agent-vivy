# MEM-1A Task 2 report

## Status

DONE_WITH_CONCERNS

## Changed files

- `internal/modules/memory/actions.go` — replaced the stub `Invoke` with a
  per-action dispatch table (`invoke func(ctx, *Service, json.RawMessage)`).
  All nine `vivy.memory.*` providers resolve `Active()` at call time; a closed
  registry returns `failed`/`bml_unavailable`, oversized input returns
  `failed`/`memory_invalid_request`, and every logical failure stays inside
  the four-state outcome JSON — no Go errors for domain failures. Strict
  decoding (`DisallowUnknownFields` + trailing-JSON check, empty input → `{}`),
  per-action closed input schemas, 32 KiB input / 256 KiB output caps (masks
  precedent); an oversized marshaled result collapses to an explicit failed
  outcome, never a truncated payload.
- `internal/modules/memory/service.go` — `Rules` now returns `RulesView`
  (`content`, `source`, `revision`) so callers can obtain the CAS base;
  `WriteRules` takes `baseRevision` and compares it against
  `home.StartupRevision()` before writing — mismatch → `failed`/
  `memory_revision_conflict`.
- `internal/modules/memory/actions_test.go` — new suite: closed inventory vs
  manifest IDs + effects, round trips for list/add/get/search/update/remove,
  stale-revision conflicts for update/remove/rules.write, `kind` enforcement,
  rules read/write CAS round trip, status payload, invalid-input table,
  oversized input, and unavailable outcome for all nine actions.

## Decisions

- **`rules.write` CAS base.** The plan input sketch says `{content}` but the
  constraint says `rules.write` "carries `base_revision` CAS" and a stale
  revision "returns conflict, never a forced write". BML exposes no MEMRULES
  document revision; the only revision on this surface is
  `Home.StartupRevision()` (the authority revision `vivy.memory.status`
  advertises). Implemented as a **required** `base_revision` compared against
  `StartupRevision()`; `rules.read` returns that revision so the
  read→modify→write loop is closed. Limitation: rules writes don't bump the
  revision (it tracks the record projection), so two rules writes against the
  same base both succeed — the CAS guards the memory state the caller read,
  not rules-vs-rules lost updates.
- **`add.kind` is required and must be `long_term`** (bml's only permitted
  kind); other kinds → `failed`/`memory_kind_forbidden`, absent →
  `memory_invalid_request`. `evidence` decodes as `[]bml.EvidenceRef`.
- **`update`/`remove`/`rules.write` treat absent `base_revision` as
  `memory_invalid_request`** (pointer decode distinguishes missing from 0);
  a mismatched revision is `memory_revision_conflict` from the store.
- **`status`/`rules.read` payloads** are `{status:"listed", ...}` objects so
  the `required:["status"]` result contract holds; degraded/closed service
  reports `failed`/`bml_unavailable` like every other action.
- Wire field names follow the plan sketch (`id`, `query`, `kind`, `evidence`,
  `reason`, `base_revision`), mapped onto the bml `record_id`/`evidence_refs`
  request types at the action boundary.

## Commands and results

- `go test ./internal/modules/memory -count=1` — pass (13 test fns incl.
  module_test.go).
- `go build ./internal/modules/memory ./internal/app` — pass (with the
  documented `ui/dist/.keep` placeholder for `go:embed all:dist`).
- `go vet ./internal/modules/memory ./internal/modules/defaults
  ./internal/app` — clean.
- `gofmt -l internal/modules/memory` — clean.
- `go test -tags vivy_headless ./internal/app -count=1` — pass (generated
  assembly inventory assertions green).
- `go test ./internal/modules/defaults ./internal/modules/masks -count=1` —
  pass.
- Manifest `Actions` == declared PortRef IDs == provider `Definition().ID`s:
  all three lists are built from the same `Action*` constants; catalog
  unchanged, `zz_default.go` not regenerated.

## Concerns

- The `rules.write` CAS semantics above are an interpretation of two
  partially contradictory plan lines; if the intent was a MEMRULES-document
  revision (content-hash) rather than the authority revision, the service
  needs a small revision scheme added. Flagging for reviewer veto.
- `StartupRevision()` is process-local (restarts at 0/1 on reopen), so a
  base_revision captured before a restart will conflict — safe (stale fails
  closed) but conservative.
- Go toolchain was absent on this box; Go 1.26.4 installed to `~/tools/go`
  with `GOPROXY=https://goproxy.cn,direct` per the justfile mirror note.
  `just ci` not run (no `just` binary).
