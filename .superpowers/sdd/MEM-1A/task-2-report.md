# MEM-1A Task 2 report

## Status

DONE (revised after T2 review: rules.write CAS moved to content digest)

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
- `internal/modules/memory/service.go` — `Rules` returns `RulesView`
  (`content`, `source`, `revision`) where `revision` is the sha256 content
  digest of the current MEMRULES document (`bml.MemoryContentDigest`);
  `WriteRules` takes `baseRevision` and compares it against the digest of
  current content before writing — mismatch → `failed`/
  `memory_revision_conflict`. `StatusResponse` gains `rules_revision`
  carrying the same digest.
- `internal/modules/memory/actions_test.go` — new suite: closed inventory vs
  manifest IDs + effects, round trips for list/add/get/search/update/remove,
  stale-revision conflicts for update/remove/rules.write, `kind` enforcement,
  rules read/write CAS round trip, status payload, invalid-input table,
  oversized input, and unavailable outcome for all nine actions.

## Decisions

- **`rules.write` CAS base (revised per T2 review).** The plan input sketch
  says `{content}` but the constraint says `rules.write` "carries
  `base_revision` CAS" and a stale revision "returns conflict, never a
  forced write". Implemented as a **required** `base_revision` equal to the
  sha256 digest of the current MEMRULES content; `rules.read` and
  `vivy.memory.status` (`rules_revision`) expose that digest so the
  read→modify→write loop is closed. A content-digest CAS detects
  rules-vs-rules interleaved writes and external MEMRULES edits, and
  survives restarts — the earlier `StartupRevision()` proxy could not.
  Note `base_revision` is therefore a digest string for `rules.write` while
  remaining an integer record revision for `update`/`remove`.
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

- None remaining from the CAS revision question — the reviewer-selected
  content-digest base removes the process-local and lost-update weaknesses.
- Go toolchain was absent on this box; Go 1.26.4 installed to `~/tools/go`
  with `GOPROXY=https://goproxy.cn,direct` per the justfile mirror note.
  `just ci` not run (no `just` binary).
