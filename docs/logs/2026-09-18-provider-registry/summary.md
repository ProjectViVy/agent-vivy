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

## Scope of this phase (`PROV-P2`)

Delivered:

- `internal/provider/adapters.go` now carries `Adapter{Family, State}` and
  `Adapters()`; `Capabilities()`, `AdapterFamilies()`, `AdapterState` and
  `IsSealedAdapter` are all derived from that one table, and `app.go` builds the
  ModelHost capability map from it.
- The sealed unit is the adapter everywhere: `Catalog.Adapter(family)` is the
  sealed lookup (vendor-neutral), `Catalog.RefForEndpoint(vendor, endpoint)` is
  the vendor-bound construction path, `ForProfile`/`ProfileFromEndpoint`/
  `ErrAdapterFamilyMismatch`/`legacyAdapterFamily` are gone, and the runtime gate
  and availability marks are keyed by the endpoint's adapter.
- `provider.AdapterProfiles()` projects the sealed adapters onto the compiled
  Profile set, unioning each family's model ids and Secret references over every
  embedded endpoint that speaks it; `defaults.ProviderProfiles()` delegates and
  keeps the name and signature the SDK evidence anchors cite. The deferred
  `openai-responses` family is a visible, non-executable Profile.
- Thinking is decided by a pure function over adapter, endpoint capability, model
  metadata and the run preference. **Behaviour fix:** OpenAI-compatible reasoning
  models now receive `reasoning_effort: high` on `auto`/`on`; they received
  nothing before.
- `internal/generated/assembly/zz_default.go` regenerated (byte-reproducible),
  `sdk/internal/testdata/default-generation.expected.json` updated, conformance
  digest refreshed, and `MIGRATION.md` §8 records the as-built detail.

Not done (later phases, in order):

- `PROV-P3` reduces `config.Providers` to `active` plus stateless overrides,
  moves credentials onto the embedded catalog, and adds the `settings.yaml`
  alias map. `app.transitionalVendorNames` and `defaultModelFor` are its
  removal targets.
- `PROV-P4` serves the catalog over RPC and deletes the UI's generated catalog,
  which is when the pre-baked Settings/TUI vendor list becomes the whole
  embedded catalog.
- `PROV-P5` runs the single full `just ci` and closes the board.

## Scope of this phase (`PROV-P3`)

Delivered:

- `config.Providers` is one field: `active`, naming the vendor whose declared
  endpoint the runtime falls back to. The three per-vendor blocks, their
  `env_key`/`default_model` validation and the `Provider` struct are gone, so
  `config.example.yaml` is nine lines shorter. A document that still carries
  `deepseek:`/`openai:`/`anthropic:` fails strict decoding with an error that
  names the removed key and says why.
- The stored selection names a **sealed adapter**, and the vendor, endpoint
  variant, address and default model all come from the embedded data:
  `resolveStoredSelection` prefers an explicit address an embedded endpoint
  declares, then the vendor a pre-migration value named, then
  `config.providers.active`. `Catalog.EndpointForVendor` gained the `adapter`
  parameter; `Catalog.AdapterFamily` is gone and `Catalog.VendorForEndpoint`
  replaces it for the address→vendor direction.
- `internal/app/settings/provider_migration.go` holds the one normalization
  table and its four consumers: `Settings.Validate`, `validateProviderEntries`,
  `FindProvider`/`ActiveKey`, and the RPC allowlist. The registry uniqueness key
  normalizes too, so one endpoint identity cannot be stored under two spellings.
- Credentials are data-derived at both ends: the model resolver's allowlist is
  `provider.VendorEnvKeys(catalog.Vendors())`, the composition root no longer
  passes three config env keys into `credentialmodule.CompileScopes`, and
  `applySettingsEnv` writes a key into the **resolved vendor's** variable. A
  third-party vendor therefore works from its own environment variable with no
  per-vendor configuration; `MINIMAX_API_KEY` plus a MiniMax endpoint is Ready
  where it was impossible before.
- `freezeFromEnv` iterates the embedded vendors, `VIVY_PROVIDER` names a vendor
  (or an adapter), the configured vendor wins when several keys are set, and
  `defaultModelFor`/`providerConfigBaseline`/the three-way switch in
  `applySettingsEnv` are deleted, together with `app.transitionalVendorNames`
  (the pre-baked catalog list is now `settings.LegacyVendorNames()`).
- `ResolvedModel` and `LiveSpec` carry `Adapter` next to `Provider`, so the
  ModelHost, the availability projection and the thinking rules key on the
  sealed protocol while the vendor keeps ownership of the credential and the
  display name.

Not done (later phases, in order):

- `PROV-P4` serves the whole embedded catalog over RPC (`Deps.ProviderVendors`
  widens from the three pre-migration vendors), makes the UI zero-data, and
  deletes `ui/scripts/gen-provider-catalog.py` and `ui/agent-diva-source/`.
- `PROV-P5` runs the single full `just ci` and closes the board.

## Deviations worth knowing

- Vendor and credential identifiers may now start with a digit, because the
  upstream registry contains `302ai` and `302AI_API_KEY`. `config.ValidEnvKey`
  and the settings `auth_env` pattern were relaxed to the same rule.
- The catalog is 45 vendors, not the 46 the ledger predicted: `cherryin`
  declares no models and no default model and cannot form a valid endpoint. It
  remains reachable as a user-defined custom provider.
- `Catalog.EndpointForVendor` and the `Deps.ProviderVendors` rename landed in
  P1 rather than P3/P4, because the types they replaced no longer exist.
- `providerprofile.secretRefPattern` was widened in P2 for the same leading-digit
  reason, so the Profile validator and `config.ValidEnvKey` agree.
- P2 keeps the `settings/provider` payload byte-identical through
  `transitionalVendorNames`; the wire vocabulary changes in P3, not here.
- P3 stores a selected value as the client sent it. A pre-migration client
  writing `provider: openai` keeps that spelling, because the vendor name is the
  only thing that identifies the credential owner of an *undeclared* gateway
  address; rewriting it to the adapter would silently move the selection onto the
  configured vendor. The new UI (P4) writes adapter values, so documents converge
  as they are edited, and both spellings stay readable forever.
- P3 keeps `Deps.ProviderVendors` at the three pre-migration vendors, now derived
  from `settings.LegacyVendorNames()` instead of a hard-coded app list. Widening
  it to the whole catalog is P4's `settings/providers` payload change
  (`MIGRATION.md` §8.6).
- `config.Validate` cannot check that `providers.active` exists in the embedded
  data: `internal/config` must not import `internal/provider` (the Eino import
  quarantine keeps that package out of a leaf config type). `Validate` checks the
  shape and the app's startup gate checks membership, one line after the data is
  loaded.