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

## Scope of this phase (`PROV-P4`)

Delivered:

- `settings/providers` carries the embedded catalog itself: `catalog:
  [{vendor, display_name, endpoints: [{adapter, base_url, default_model,
  models, executable, state}]}]`, built in `internal/rpc/control.go` from
  `provider.Capabilities()`. `executable` is the adapter's capability state
  (`modelhost.CapabilitySupported`), so the `DEFERRED-INDEFINITE`
  `openai-responses` endpoint is **visible and not selectable** instead of
  invisible. `Deps.ProviderVendors` widens from the three pre-migration vendors
  to all 45, which also retires `settings.LegacyVendorNames()`.
- The UI holds zero provider data. `ui/scripts/gen-provider-catalog.py` and the
  46-entry `PROVIDER_CATALOG` array are gone; `provider-catalog.ts` is a pure
  projection over the payload, `ModelSettingsCard.tsx` renders only loaded rows
  (with an explicit loading state and no invented fallback list),
  `custom-providers.ts` merges catalog rows with the registry and derives the
  refresh rule from the adapter (`openai-completions` + `http(s)`), and
  `saved-models.ts`/`GenerationParamsCard.tsx`/`MaskAndModelSwitcher.tsx` resolve
  labels and selection from the catalog.
- A vendor with two protocols is two rows. DeepSeek contributes
  `openai-completions@https://api.deepseek.com` and
  `anthropic-messages@https://api.deepseek.com/anthropic` under one display
  name, so the adapter prints inline on a multi-endpoint vendor and the row
  identity is `(vendor, adapter)`. The old fold ("More providers") was provider
  data with no wire representation and is gone.
- Selection writes the adapter: a row click plus a model click sends
  `{provider: <adapter>, base_url: <the endpoint's declared address>,
  default_model: <model>}` through `settings/update`, which the resolver reads
  back as the same `(adapter, base_url)` endpoint.
- `sdk/ui/src/module.ts` (the `@vivy/ui-sdk` face contract) names the real
  vocabulary: `FaceProviderAdapter`/`FaceLegacyProviderBundle`/
  `FaceProviderValue`, `FaceProviderEndpoint`/`FaceProviderCatalogEntry`, and
  `catalog` on `FaceProvidersView` and `FaceStoreState`. The package version
  stays `1.0.0`; the change is additive for module authors.
- The TUI reads the same payload (`sdk/tui/live/rpc.go` iterates catalog
  endpoints, keying `provider` on the adapter and skipping non-executable
  endpoints), so both faces are projections of one backend list.
- `ui/e2e/global-setup.ts` no longer writes the per-vendor config block that
  PROV-P3 removed — the e2e suite is outside `just ci`, so it was the one
  consumer still carrying the old shape, and strict config decoding rejected it.
- The `internal` source digest moved `0d24ebe4…` → `5e386f84…` in the five
  `internal`-rooted `sourceSha256` entries of
  `sdk/internal/assembly/conformance_results.json` (`MIGRATION.md` §8.7). The
  earlier value was written before the last `internal/` edit, which is the
  manual step the board tracks as `PROVIDER-PROFILE-DIGEST-PIN`.
- `ui/agent-diva-source/` (861 files, 17.4 MB, gitignored at `ui/.gitignore:31`)
  is gone from the launch checkout. It could never be deleted by the branch —
  worktrees do not share gitignored directories — so P4 removed it from the
  checkout itself, which is what makes the repository stand alone on disk.
  Because it was gitignored it is in no commit either, and the only remaining
  copy is the one the removal moved aside:
  `C:\Users\Administrator\AppData\Local\Temp\2\prov-p4-agent-diva-source`
  (861 files, 17.4 MB — the vendored upstream tree the data was re-derived
  from). It is left in place deliberately: deleting the last copy is a separate
  decision, not part of the branch. The re-derivation rules and the dropped
  upstream fields live in `internal/provider/data/README.md`, and each vendor's
  `provenance` block names its upstream entry, so the data stays traceable
  without the tree.

Not done (later phases, in order):

- `PROV-P5` runs the single full `just ci` and closes the board.

## Scope of this phase (`PROV-P5`)

Delivered:

- One `just ci` on the final tree: exit 0 across `fmt-check`, `ui-ci`, `vet`,
  `test`, `headless-compile`, `plugin-ci`, with `sdk/internal` 506.8s and
  `sdk/internal/conformance` 195.1s inside it. That same run is the digest's
  proof, because the reproduction test recomputes the `internal` tree hash and
  byte-compares the checked-in artifact.
- The browser path re-exercised on the committed tree at
  `http://127.0.0.1:3015` (14/14), and the e2e specs this program changed
  (`model-refresh`, `welcome-wizard`) pass in the suite run.
- Board closeout: `PROVIDER-REGISTRY-REDESIGN` moved to `docs/COMPLETE.MD` §0.1
  with the program's history; the follow-ups it produced stay open in
  `docs/TODO.md` §0.1 (`PROVIDER-AGENTIC-MIGRATION`,
  `PROVIDER-DATA-CONFIG-EDIT`, `PROVIDER-PROFILE-DIGEST-PIN`,
  `UI-PROVIDER-WIZARD-STEP`, `CI-E2E-NOT-IN-GATE`).

What the program leaves behind: one embedded data file
(`internal/provider/data/vendors.yaml`, 45 vendors / 47 endpoints / 168 models)
validated at startup against exactly three sealed protocol adapters; a config
that names one fallback vendor; credentials derived from the data; and two faces
(the Web UI and the TUI) that render a payload instead of carrying a list. The
original four descriptions of "provider" — disk bundles, a Go metadata table, a
generated TypeScript array, and an unowned upstream registry — are gone, and
`ui/agent-diva-source/` no longer exists on disk.

The original shape was worse than "four copies of one fact": the directory the
runtime read at startup was `fixtures/provider/`, which `fixtures/README.md`
describes as test data and which `PRD §6.2` forbids in production paths, while
the shipped binary had no provider knowledge at all (no `//go:embed`, a
worktree-relative default, a Docker `COPY`, a pack-time copy from the packing
worktree). The diagnosis, with citations and the generalizable checks, is
`notes.md` in this directory — it supersedes the framing used elsewhere.

Not done by the program, and done since: the owner authorized landing on
2026-09-18, so the branch was fast-forwarded into `main` and pushed
(`MIGRATION.md` §8.9 records the procedure, the fact that `origin/main` was
eight commits behind and this push also publishes those, and the digest caveat
for a checkout that carries another lane's untracked files under `internal/`).
CI on the pushed tip passed `ui ci` and `backend ci`; the `full UI browser smoke`
job failed for a pre-existing timing reason in a job added by `082f0d9` on
2026-09-15 (its own 30s budget must cover a cold `go build`), so `main`'s CI has
been red since that job appeared — tracked as `CI-BROWSER-SMOKE-WEBSERVER`.

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
- `@vivy/ui-sdk` stays at `1.0.0` although its provider types changed. The
  version is a build pin (`sdk/ui/version.go`, `UI_BUILD_MANIFEST`), the change
  is additive for module authors, and no rule in `VIVY-FACE-PACK.md` versions the
  contract separately; bumping it would have meant touching the manifest test and
  the Go pin for no product reason.
- The UI keeps exactly one vendor-shaped table: the read-side alias map
  `{deepseek, openai, anthropic} → adapter` in `provider-catalog.ts`, mirroring
  the backend's normalization table so a pre-migration document resolves the same
  way on both sides. It is a compatibility vocabulary, not catalog data. What
  else still names a vendor under `ui/src` is demo material that never touches
  the settings or selection path — `lib/demo-api.ts` and
  `components/trajectory/trajectory-demo-data.ts`, both explicitly marked
  demo/local-mock by `ui/AGENTS.md` — plus illustrative placeholder copy in the
  wizard (a model id and an address shown as examples). The welcome wizard also
  keeps a DeepSeek console URL (a help link for obtaining an API key); its
  free-text model step is the last provider-shaped UX and is tracked on the board
  as `UI-PROVIDER-WIZARD-STEP`.
- Browser findings that are not this phase's defects: `just ui-e2e` is not part
  of `just ci` (`CI-E2E-NOT-IN-GATE`), and this workstation could not download
  Playwright's pinned Chromium revision, so the suite ran on the system Chrome
  through a scratch config that was never committed (`verification.md`).