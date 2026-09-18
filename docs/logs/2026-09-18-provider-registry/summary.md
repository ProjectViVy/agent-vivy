# Provider registry: one source of truth

Date: 2026-09-18. Branch: `feat/provider-registry`. Program plan:
`docs/plans/provider-registry/README.md` (phases `PROV-P1`..`PROV-P5`).

## Why

Vivy described a provider in four places: three YAML bundles under `fixtures/`
(loaded from disk at startup), a hard-coded model-metadata table in Go, a
47-entry generated TypeScript catalog in the UI, and a Diva registry the
repository did not own. They drifted, and the UI could not see a provider the
backend did not ship.

This program collapses that into one source: provider data embedded in the
binary at `internal/provider/data/vendors.yaml`, validated at startup against a
sealed set of exactly three protocol adapters, with the frontend holding no
provider data at all.

## Scope of this phase (`PROV-P1`)

Delivered:

- `internal/provider/data/vendors.yaml` — 45 vendors, 47 endpoints, 168 model
  entries derived once from the Diva registry, with per-model metadata for the
  first-party vendors and provenance stamped on every entry.
- `internal/provider/data/{provider.schema.json,README.md}` — the data contract
  and how to edit the catalog.
- `internal/provider/{adapters,vendor,embed,reconcile}.go` — the sealed adapter
  names, the strict parser and validation rules, `//go:embed` loading, and the
  startup consistency gate that fails closed when data and code disagree.
- The `Bundle` type, the disk loader, `fixtures/`,
  `schemas/providers.bundle.schema.json`, and `providers.bundle_dir` are gone;
  the Catalog now resolves vendors and endpoints from the embedded data.
- Failure-first tests: data validation (18 rejection cases), the gate in both
  directions, the embedded catalog's shape, the metadata anchors, the DeepSeek
  default chain, and the deferred `openai-responses` endpoint.
- `MIGRATION.md` §7 records the as-built inventory, including nine sites the
  plan's file list missed and four inventory corrections.

Not done (later phases, in order):

- `PROV-P2` re-seals the Profile set on adapter identities, folds the family
  bridge into an `Adapter{Family, State}` table, and fixes the OpenAI reasoning
  request shape.
- `PROV-P3` reduces `config.Providers` to `active` plus stateless overrides,
  moves credentials onto the embedded catalog, and adds the `settings.yaml`
  alias map.
- `PROV-P4` serves the catalog over RPC and deletes the UI's generated catalog.
- `PROV-P5` runs the single full `just ci`, refreshes the conformance digest,
  and closes the board.

Behaviour is intentionally unchanged in this phase: the same three vendors
resolve, the same `settings/providers` payload crosses the wire byte for byte,
and the same DeepSeek/Anthropic request shapes are sent. The only user-visible
difference is that provider metadata can no longer be pointed at a directory.

## Deviations worth knowing

- Vendor and credential identifiers may now start with a digit, because the
  upstream registry contains `302ai` and `302AI_API_KEY`. `config.ValidEnvKey`
  and the settings `auth_env` pattern were relaxed to the same rule.
- The catalog is 45 vendors, not the 46 the ledger predicted: `cherryin`
  declares no models and no default model and cannot form a valid endpoint. It
  remains reachable as a user-defined custom provider.
- `Catalog.EndpointForVendor` and the `Deps.ProviderVendors` rename landed in
  P1 rather than P3/P4, because the types they replaced no longer exist.