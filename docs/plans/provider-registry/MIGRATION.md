# Provider Registry — Migration Inventory

**Status:** authored 2026-09-18. Documentation only; nothing here has been applied.

## 0. How to use this document

Line numbers were correct at `ff8a47d` (2026-09-18). **They are not a contract.**
Before touching any file, re-derive the list:

```powershell
git grep -n '"deepseek"' -- 'internal/**/*.go'
git grep -n 'DeepSeek\|ProviderDeepSeek' -- 'internal/**/*.go'
git grep -n 'bundle\|Bundle' -- 'internal/**/*.go' 'ui/src/**/*.ts*'
git grep -n 'fixtures' -- . ':!.worktrees'
```

Implement phases in order (`PROV-P1` → `PROV-P5`); each phase is one focused
commit with its own iteration log.

---

## 1. `internal/` — Go production code

### 1.1 Assembly and generated wiring

| Location | Today | Change | Risk |
|---|---|---|---|
| `internal/generated/assembly/zz_default.go:79` | `ProviderProfiles: []string{"deepseek","openai","anthropic"}` | adapter identities (three families) — **regenerate, never hand-edit** | generated file; a hand edit breaks the compiler evidence chain |
| `internal/modules/defaults/providers.go:19-38` | three vendor-named Profiles with `ModelIDs`/`SecretRefs` | three protocol-named adapter Profiles; `ModelIDs`/`SecretRefs` leave the Profile and come from data | the public Port projection is consumed by SDK evidence |
| `internal/modules/defaults/providers_test.go:10-31` | asserts 3 profiles, ids `deepseek/openai/anthropic`, families, secret refs | assert 3 adapter families and their states | |
| `internal/modules/defaults/catalog.go:50` | Port evidence record naming the three providers | evidence record naming the three adapters | |
| `sdk/generation/manifest.go:105` | `ProviderProfiles []string` | unchanged field, new values (adapter families) | wire/manifest semantics change, not shape |
| `sdk/internal/assembly/runtime_generate.go:116,298,408` | emits `ProviderProfiles` | unchanged mechanics | |
| `sdk/internal/assembly/evidence.go:77-80` | anchors: `internal/modules/defaults/providers.go#ProviderProfiles`, `providers_test.go#TestDefaultProviderProfilesMatchExistingRuntimeFamilies`, `sdk/generation/manifest.go#ProviderProfiles` | keep anchors valid; if a symbol is renamed, update the anchors in the same commit | **stale evidence anchors fail conformance** |
| `sdk/internal/removal_conformance_test.go:78` | expects symbol `{"vivy/provider-profiles","NewProviderProfiles","ProviderProfiles"}` | keep names or update the expectation | |
| `sdk/internal/conformance/reproduction_test.go:456` | cites `TestDefaultProviderProfilesMatchExistingRuntimeFamilies` | keep the test name or update the citation | |

Regeneration command (do not hand-edit the generated file):

```powershell
go run ./sdk/internal/cmd/generate-default --repo . --output internal/generated/assembly/zz_default.go
```

Also declared as `//go:generate` in `internal/generated/assembly/generate.go:3`.

### 1.2 Adapter layer (`internal/provider`)

| Location | Today | Change |
|---|---|---|
| `internal/provider/bundle.go` | `Bundle` struct with 17 fields; `LoadBundle(path)`; `ParseBundle([]byte)`; `validate()` | becomes the vendor/endpoint/model types; loading moves to the embedded FS; keep the strict-decode + joined-errors validation style |
| `internal/provider/bundle.go:38-54` | 10 fields with zero consumers | delete (see `DESIGN.md` §3.2) |
| `internal/provider/bundle.go:53` | `Backend string` | delete; `adapter` supersedes it |
| `internal/provider/catalog.go:20-37` | `Catalog` indexed by bundle name | index by vendor and by adapter family |
| `internal/provider/catalog.go:41-54` | `For(name)` switches on `Backend` | `Adapter(family)` over the sealed table |
| `internal/provider/catalog.go:59-76` | `ForProfile(profile)` looks the bundle up **by `profile.ID`** (`catalog.go:60`) and cross-checks `AdapterFamily` | look up by `AdapterFamily`; the `ErrAdapterFamilyMismatch` check becomes structural rather than a cross-source comparison |
| `internal/provider/catalog.go:82-88` | `ResolveModelInfo(ctx, providerName, modelID)` | key becomes the endpoint/model pair resolved from data |
| `internal/provider/profile.go:19-33` | `ProfileFromBundle` projects YAML → Profile | projects an endpoint → Profile |
| `internal/provider/openai.go:36-68` | `knownOpenAIModels`, including DeepSeek and OpenAI reasoning entries, with the dual-purpose `supportsThinking` flag (`openai.go:41-43`) | metadata moves into the data file; the flag splits into `supports_thinking` (model) and the `deepseek-thinking` capability (endpoint) |
| `internal/provider/claude.go:78-103` | `knownAnthropicModels` | metadata moves into the data file |
| `internal/provider/claude.go:17-24,60-65` | `claudeDefaultMaxTokens = 8192`, `claudeThinkingBudgetTokens = 4096`, `AutoCacheControl` gated on `SupportsPromptCaching` | keep the constants; the caching flag becomes an endpoint field |
| `internal/provider/resolving.go:43-45,98-102` | `NewResolvingChatModel(host, catalog, src)`; `catalog.ForProfile(profile)` | resolve endpoint → adapter family |
| `internal/provider/resolving.go:128-162` | `thinkingOptions`: `BackendEinoClaude` branch, and `if bundle.Name != "deepseek" { return nil }` (`:146`) | switch on the endpoint's `capabilities` + the model's `supports_thinking`; the hard-coded vendor name disappears |
| `internal/provider/resolving.go:91-92` | cache key `provider\x00model\x00baseURL\x00sha256(key)` | key on the endpoint identity instead of the vendor name |
| `internal/provider/ref.go:27-40` | `Ref` interface, `ModelSpec{ID,APIKey,BaseURL}` | unchanged |
| `internal/provider/discover.go` | `ModelListClient.List` decodes only `data[].id` | optionally also read `context_length` when present (out of phase-1 scope; note it) |

New files (phase `PROV-P1`/`PROV-P2`):

```
internal/provider/data/vendors.yaml
internal/provider/data/provider.schema.json
internal/provider/data/README.md
internal/provider/embed.go              # //go:embed data/*.yaml
```

`//go:embed` cannot reference parent directories, which is why the data lives
inside the package directory rather than at the repository root.

### 1.3 Configuration

| Location | Today | Change |
|---|---|---|
| `internal/config/config.go:214-226` | `Providers{Active, BundleDir, DeepSeek, OpenAI, Anthropic}` | `Providers{Active}` (vendor id) + optional endpoint override; `BundleDir` deleted |
| `internal/config/config.go:605-611` | default `Active: "deepseek"`, `BundleDir: "fixtures/provider"`, three `{env_key, default_model}` blocks | default `Active: "deepseek"` only |
| `internal/config/config.go:723-741` | validates `Active ∈ {deepseek,openai,anthropic}`, `BundleDir` non-empty, three env-key patterns, three non-empty default models | validates `Active` against the embedded vendor set |
| `config.example.yaml:36-37`, `config.yaml:12-23` | `bundle_dir` + three provider blocks | `providers.active` only. Note: the root `config.yaml` is gitignored local walkthrough config, so it exists only in checkouts that created one; it is absent in a fresh worktree and its absence is not an error. |

### 1.4 Application wiring

| Location | Today | Change |
|---|---|---|
| `internal/app/app.go:254-256` | `bundlePath` closure joining `BundleDir` + `<name>.yaml` | delete |
| `internal/app/app.go:257-272` | three `provider.LoadBundle` calls, then `provider.NewCatalog(...)` | `provider.LoadEmbedded()` + catalog construction |
| `internal/app/app.go:273-276` | `compiledProfiles` from `runtimeAssembly.ProviderProfiles` | unchanged mechanics; profile identities become adapters |
| `internal/app/app.go:277-283` | `credentialmodule.CompileScopes(..., cfg.Providers.DeepSeek.EnvKey, cfg.Providers.OpenAI.EnvKey, cfg.Providers.Anthropic.EnvKey)` | env-key set comes from the embedded vendor data |
| `internal/app/app.go:288-295` | `modelmodule.Compose(compiledProfiles, Capabilities{two families: SUPPORTED})` | three families, one `DEFERRED-INDEFINITE` |
| `internal/app/app.go:866-868` | `ConfigProvider`, `ProviderBundles` for the RPC deps | `ConfigProvider` = vendor id; `ProviderBundles` replaced by the catalog payload |
| `internal/app/app.go:1226-1248` | `applySettingsEnv` hard-codes a 3-way switch to pick the env var name (`:1233-1240`) | look the env var up from the embedded vendor data by the active endpoint |
| `internal/app/app.go:1271-1287` | settings overlay switch writing `cfg.Providers.<X>.DefaultModel` (`:1273-1286`) | overlay the resolved endpoint default; no per-vendor config block |
| `internal/app/app.go:1527-1536` | `defaultModelFor` 3-way switch on `"deepseek"`/`"anthropic"` | look up the vendor's default endpoint in data |
| `internal/app/model.go:45-60` | `newModelResolver` fallback `CompileScopes` with three config env keys | env keys from data |
| `internal/app/model.go:62-116` | `freezeFromEnv` iterates three hard-coded candidates (`:69-73`) | iterate the embedded vendors |
| `internal/app/model.go:135-170` | `currentLocked` switches on three provider names for the default model (`:152-159`) | resolve from the endpoint |
| `internal/app/model.go:16-17` | `VIVY_MODEL`, `VIVY_PROVIDER` env overrides | keep; `VIVY_PROVIDER` now names a vendor |
| `internal/app/settings/settings.go:52-54` | `ProviderDeepSeek/OpenAI/Anthropic` constants | vendor ids come from data; keep only the normalization aliases (see §3) |
| `internal/app/settings/settings.go:228-246` | `ProviderEntry{ID, DisplayName, Bundle, BaseURL, DefaultModel, Models, ApiKey}` | `Bundle` → the adapter family field (`bundle` YAML key kept for compatibility, or renamed with a migration read) |
| `internal/app/settings/settings.go:460-466` | `Settings.Validate` restricts `provider` to three names | validate against adapter families, applying the normalization map first |
| `internal/app/settings/settings.go:749-753` | registry entry `bundle` restricted to three names | validate against adapter families |
| `internal/app/settings/settings.go:765` | uniqueness key `bundle + "\x00" + base_url` | unchanged (adapter + base_url) |
| `internal/app/settings/settings.go:793-798` | `ActiveKey` resolves the registry key by `(provider, baseURL)` | unchanged mechanics |

### 1.5 Eval isolation

| Location | Today | Change |
|---|---|---|
| `internal/eval/isolator.go:74-80` | `bundleDir` + `filepath.Abs` + fallback `"fixtures/provider"` | delete; the child inherits the embedded data from the same binary |
| `internal/eval/isolator.go:87-93` | writes `providers.active` + `bundle_dir` + three blocks into the child config | writes `providers.active` only |

### 1.6 RPC

| Location | Today | Change |
|---|---|---|
| `internal/rpc/control.go:110-117` | `Deps.ConfigProvider`, `Deps.ProviderBundles []provider.Bundle` | vendor id + a catalog snapshot type |
| `internal/rpc/control.go:3731-3750` | settings view `DefaultModel`, `ConfigProvider` | unchanged shape |
| `internal/rpc/control.go:4424-4441` | `toProviderEntryResult` redacts the key to `api_key_set` | unchanged |
| `internal/rpc/control.go:4443-4493` | `providersResult` with `entries`, `bundles`, `profiles`; `bundles` carries the three YAML model lists and is **not consumed by the UI** | becomes the catalog payload (vendors + endpoints + adapters + profile states) and is consumed |
| `internal/rpc/control.go:4569-4619` | `settings/update` allowlist: config pair, or a registry entry for `(bundle, base_url)`, or the bundle's `models` when `base_url == ""` (`:4604-4615`) | same rules; sources become the embedded endpoint model lists |
| `internal/rpc/control.go:4780-4825` | `/models` refresh gate hard-codes `bundle ∈ {ProviderOpenAI, ProviderDeepSeek}` (`:4793`, `:4821`) | gate on `adapter == openai-completions` |
| `internal/rpc/control.go:4383-4400` | `providerProfileStatusResult` with `model_ids`, `state` | unchanged shape |

### 1.7 Tests that will need updating

Non-exhaustive but complete enough to plan the work; re-derive with the greps in §0.

| File | Why it breaks |
|---|---|
| `internal/provider/provider_test.go` | `fixturesDir = "../../fixtures/provider"` (`:20`), `LoadBundle` assertions (`:22-56`), `NewCatalog(...).For(...)` (`:114-200`) |
| `internal/provider/deepseek_test.go` | loads the DeepSeek fixture; asserts the thinking request body (`:114-203`) and base-URL path (`:204-215`) |
| `internal/provider/claude_test.go:145` | `NewCatalog(newClaudeTestBundle(...))` |
| `internal/provider/resolving_thinking_test.go:33,105` | builds catalogs from bundles |
| `internal/provider/modelhost_routing_test.go:32-83` | `NewResolvingChatModel(nil, NewCatalog(bundle), …)` |
| `internal/provider/secret_audit_test.go` | bundle-shaped fixtures |
| `internal/provider/titler_test.go:115` | `NewCatalog()` |
| `internal/app/model_test.go:24-54,151-155` | loads three fixtures, `ProfileFromBundle` |
| `internal/app/default_generation_test.go:117-126` | asserts `ProviderProfiles[0].ID == "deepseek"` and manifest parity |
| `internal/app/{tokenstats_smoke,realsmoke,rpc_route,shutdown,facehost,action_gateway,mcp_live_reload,settings_overlay}_test.go` | construct `config.Providers{Active, BundleDir, DeepSeek, …}` |
| `internal/config/config_test.go:56,92,544` | `bundle_dir` fixture and default assertions |
| `internal/modules/defaults/providers_test.go` | profile identities |
| `internal/rpc/control_test.go:936-2156,2433,3124` | `ConfigProvider`, `ProviderBundles`, `provider_profiles` expectations |
| `internal/eval/isolator_test.go` | child config shape |
| `sdk/internal/conformance/reproduction_test.go` | `sourceSha256` for the `internal/` tree |

---

## 2. UI (`ui/`)

| Location | Today | Change |
|---|---|---|
| `ui/src/components/settings/provider-catalog.ts:1-14` | AUTO-GENERATED header; `ProviderRuntimeBundle = 'openai' \| 'anthropic' \| 'deepseek'` | module keeps only the types/projection helpers; `ProviderRuntimeBundle` becomes the three adapter families |
| `ui/src/components/settings/provider-catalog.ts:69-260` | the 47-entry `PROVIDER_CATALOG` data array | **delete**; the catalog arrives over RPC |
| `ui/src/components/settings/provider-catalog.ts:35-45` | `EXECUTABLE_STATES`, `isProviderExecutable` | keep; now gates on adapter families |
| `ui/src/components/settings/provider-catalog.ts:49-67` | `projectProviderEntry`, `providerSelection` | keep; `providerSelection` emits the adapter name as `provider` |
| `ui/src/components/settings/custom-providers.ts:26,35-40,47,55-59` | `CUSTOM_PROVIDERS_KEY`, `isProviderRegistryBundle` (`'openai'\|'anthropic'\|'deepseek'`), `RefreshableProviderBundle` (`'openai'\|'deepseek'`), `supportsModelRefresh` | adapter-family vocabulary; refreshable = `openai-completions` |
| `ui/src/components/settings/custom-providers.ts:129-247` | `toMerged`, `allProviderEntries`, `matchMergedProviderEntry`, `splitMergedByFold`, `catalogOverlayId` | keep the merge mechanics; the catalog argument becomes the RPC payload |
| `ui/src/components/settings/ModelSettingsCard.tsx:344-347` | `allProviderEntries(providers, settings?.provider_profiles)` | consumes the RPC catalog; needs a loading state |
| `ui/src/components/settings/ModelSettingsCard.tsx:562-582,762-775` | refresh action gated by `supportsModelRefresh(bundle, baseUrl)` | adapter-family gate |
| `ui/src/components/settings/ModelSettingsCard.tsx:381-391,473-553` | `applyModelNow`, `commitPanelKey`, `confirmAddModel` | unchanged semantics; `entry.bundle` → adapter family |
| `ui/src/components/settings/GenerationParamsCard.tsx:45` | filters by `isProviderExecutable(entry.provider, …)` | adapter vocabulary |
| `ui/src/lib/api.ts:13-14` | `RPC_METHODS` incl. `settings/providers*` | plus the catalog-bearing settings RPC (reuse `settings/providers` if possible) |
| `ui/src/lib/api.ts:115,327,349` | `provider_profiles`, `ProviderProfileStatus`, `bundles?` | `bundles` is declared but **never consumed**; becomes the catalog field and starts being consumed |
| `ui/src/lib/api.ts:375` | `refreshProviderModels` | unchanged |
| `ui/scripts/gen-provider-catalog.py` | generates the 47-entry TS array from a gitignored source | **delete** |
| `ui/agent-diva-source/` | a whole vendored Rust repository, ignored by `ui/.gitignore:31` and untracked | **delete** |
| `ui/src/components/settings/*.test.ts` | catalog/bundle vocabulary | update expectations |

---

## 3. `settings.yaml` migration

`Settings.Provider` and `Providers[].Bundle` change meaning from "bundle name" to
"adapter family". Three stored values must be normalized on read:

| Stored | Normalized |
|---|---|
| `deepseek` | `openai-completions` |
| `openai` | `openai-completions` |
| `anthropic` | `anthropic-messages` |

Rules:

1. Normalization is applied wherever a stored value is validated or used
   (`Settings.Validate`, `validateProviderEntries`, `ModelResolver.currentLocked`,
   `control.go` allowlist, UI matching).
2. The normalized value is written back on the next save, so documents converge
   without a separate migration command.
3. An unknown value after normalization keeps today's behaviour: validation error
   on write, and an unusable-but-non-fatal selection on read.
4. Registry entries keep the `bundle` YAML key name for now (renaming the key is
   optional and would need its own read-compatibility path); only its vocabulary
   changes.

---

## 4. Deletions

| Delete | Evidence it is safe |
|---|---|
| `fixtures/README.md`, `fixtures/provider/{deepseek,openai,anthropic}.yaml` | `fixtures/` contains exactly these four files; the event/recovery fixtures it claims to hold live in `schemas/events/**` |
| `providers.bundle_dir` (config field, defaults, validation, eval child config, `docker/config.yaml` inheritance, `config.example.yaml`, root `config.yaml`) | replaced by embedded data; grep confirms the field's only readers are the ones listed in §1.3/§1.5 |
| `ui/scripts/gen-provider-catalog.py` | its only output is the TS data array being deleted |
| `ui/agent-diva-source/` | gitignored (`ui/.gitignore:31`) and untracked; the 47 rules are copied into the repository before deletion |
| `internal/provider/bundle.go`'s disk loader (`LoadBundle`) and the 10 dead fields + `backend` | zero consumers (verified by grep) |
| `ProfileFromBundle` as a YAML→Profile projection | replaced by endpoint→Profile |
| `Dockerfile:39` `COPY fixtures/provider /app/fixtures/provider` | no runtime file read remains; `WORKDIR /app` no longer matters for provider data |
| `schemas/providers.bundle.schema.json` | superseded by `internal/provider/data/provider.schema.json` |
| `schemas/README.md:18` reference to `../fixtures/provider/` | update |
| root `README.md:151` 「fixtures/ provider / event / recovery fixtures」 | update |
| `docs/dev/real-provider-smoke.md:12` 「Bundle | `deepseek` (`fixtures/provider/deepseek.yaml`)」 | update to the new data path |

---

## 5. Conformance artifact (`sourceSha256`)

`internal/sourcehash/tree.go:27-52` hashes **every regular file under
`internal/`**, excluding only `generated/assembly/zz_default.go`, and canonicalizes
CRLF to LF. Adding `internal/provider/data/**` therefore changes the `internal`
tree digest, and `sdk/internal/assembly/conformance_results.json` must be updated
in the same commit.

Current state of the manual step is tracked by `docs/TODO.md`
`PROVIDER-PROFILE-DIGEST-PIN`; the reproduction test computes the digest from the
live tree and fails with the wanted value, so the update is mechanical.

**Warning:** untracked files under `internal/` are inside the hash. The root tree
currently carries WF-1 lane WIP (`internal/workflow/`,
`internal/domain/workflow_test_support.go`). Any code phase must run on a clean
worktree, or those unrelated files get baked into the digest.

---

## 6. Rollback

| Phase | Rollback |
|---|---|
| `PROV-P1` (data + embed) | revert the commit; the `fixtures/` directory and the disk loader return together, since the same commit deletes them |
| `PROV-P2` (adapters + assembly) | regenerate `zz_default.go` from the reverted inputs; the generated file must never be reverted by hand alone |
| `PROV-P3` (config + credentials) | revert code; already-normalized `settings.yaml` values remain valid because the normalization map is additive — a reverted binary sees adapter names it does not recognize, so the revert must also keep the three aliases readable |
| `PROV-P4` (RPC + UI) | revert the backend and UI together; a split state leaves the UI without a catalog |
| `PROV-P5` (evidence) | no runtime effect |

Because `PROV-P3` changes the meaning of stored values, the safe landing order is
`P1 → P2 → P3 → P4 → P5` on one branch, landed as one merge. Splitting the merge
across a release boundary requires the alias read path to ship first.

---

## 7. As-built record (`PROV-P1`)

This section supersedes the corresponding rows of §1.2–§1.7 and §4 where they
disagree. It was written while landing the phase, from the actual diff.

### 7.1 Sites the plan inventory missed

Each of these broke the build or the tests during the phase and is now fixed.

| Site | Why the inventory missed it |
|---|---|
| `internal/studiocore/service.go:76-81` (`opt.Isolation.BundleDir` default pointing at `<worktree>/fixtures/provider`) | §1.3 covers `internal/eval` but not its Studio caller. |
| `internal/studiocore/service_test.go:161-167` + the local `fixtureBundle` helper (`:440`) | same gap on the test side. |
| `internal/app/app.go` eval runner construction | §1.3 lists the `Isolation` struct but not the composition root that fills it. |
| `internal/eval/runner_test.go:123-128` | §1.7 lists `isolator_test.go` only. |
| `internal/app/{tokenstats_smoke,realsmoke,rpc_route,shutdown,facehost}_test.go` | §1.7 names the group but the phase file list omitted it. `facehost_test.go` additionally copied all four fixture files into a temp `bundles/` directory per test; that whole helper is gone. |
| `sdk/internal/removal_conformance_test.go:34`, `sdk/internal/scx_release_test.go:71,175` | `sdk/` is inside the main Go module and consumes `eval.Isolation` directly. |
| `internal/codeface/launch_test.go:31,70` | sets `cfg.Providers.BundleDir` for the shared-settings tests. |
| `internal/config/config_test.go:544` | asserted the Dockerfile **must** contain `fixtures/provider`; the assertion is now inverted (the image must not copy fixtures). |
| `ui/e2e/global-setup.ts:16-24` | writes the Playwright child config; it emitted `bundle_dir`, which strict config parsing would now reject. |
| `docs/TODO.md` `DEEPSEEK-REASONING-CONTENT` relevant-paths cell | live backlog row pointing at `fixtures/provider/deepseek.yaml`. |

`internal/provider/secret_audit_test.go` was listed in §1.7 but needed no change:
it never references a bundle or a fixture path.

Historical records were deliberately not rewritten: `docs/logs/**`,
`docs/research/**`, `docs/COMPLETE.MD` and the older `docs/plans/**` describe what
shipped at their date.

### 7.2 Inventory corrections

| Item | Plan | As built | Evidence |
|---|---|---|---|
| Vendor count | 46 (47 − `custom`) | **45** (47 − `custom` − `cherryin`) | `cherryin` declares `models: []` and no `default_model` in both derived sources, so it cannot form a valid endpoint; it stays reachable as a user-defined custom provider. Ledger `D13` amended. |
| Endpoint count | not stated | 47 | `LoadEmbedded` assertion. |
| Model entries | not stated | 168 | `LoadEmbedded` assertion. |
| Identifier rules (§3.4 rules 1–2, `DESIGN.md`) | `^[a-z][a-z0-9_-]*$`, `^[A-Z][A-Z0-9_]*$` | `^[a-z0-9][a-z0-9_-]*$`, `^[A-Z0-9][A-Z0-9_]*$` | The upstream registry contains the vendor `302ai` and the credential name `302AI_API_KEY`. Rejecting them would have meant rewriting real identifiers. `internal/config`'s `ValidEnvKey` and `internal/app/settings`' `auth_env` pattern were relaxed to the identical rule, because the credential module validates a Profile's `SecretRefs` with `ValidEnvKey`. |
| `Catalog.EndpointForVendor` | scheduled in `PROV-P3` | introduced in P1 | `thinkingOptions` must resolve the endpoint (adapter + capabilities) rather than switch on a vendor name; an undeclared `base_url` (a user proxy) falls back to the vendor's default endpoint, preserving pre-P1 behaviour. |
| `Deps.ProviderBundles []provider.Bundle` | renamed by `PROV-P4` | renamed to `ProviderVendors []provider.Vendor` in P1 | The type it carried no longer exists. P1 fills it with exactly the compiled Generation's vendors, so the `settings/providers` payload is byte-identical; `PROV-P4` widens it to the whole embedded catalog. |
| `ProfileFromBundle(Bundle)` | deleted, replaced by an endpoint→Profile projection | `ProfileFromEndpoint(Vendor, Endpoint)` | The projection needs both layers: the endpoint supplies adapter/models/caching, the vendor supplies id/`env_key`. |
| RPC `bundles` array consumers | `PROV-P4` spec claims "consumed by nobody" | **consumed** by `sdk/tui/live/rpc.go:102-153` | `sdk` is in the same module; P1 keeps the payload shape, and `PROV-P4` must update the TUI in the same change. |

### 7.3 New files (the write point, ledger `D1`/`D2`)

| File | Contents |
|---|---|
| `internal/provider/adapters.go` | the three sealed adapter names, families, the `deepseek-thinking` capability and its legality per adapter |
| `internal/provider/vendor.go` | `Vendor` / `Endpoint` / `Model` / `Provenance`, the strict parser and all validation rules |
| `internal/provider/embed.go` | `//go:embed data/*.yaml`, `EmbeddedDataFiles`, `LoadEmbedded` |
| `internal/provider/reconcile.go` | the bidirectional embedded-data ↔ sealed-adapter gate (ledger `D15`) |
| `internal/provider/data/vendors.yaml` | the 45-vendor catalog |
| `internal/provider/data/provider.schema.json` | the data contract, beside the data |
| `internal/provider/data/README.md` | how to edit the catalog and what the gate enforces |
| `internal/provider/{vendor,reconcile,catalog}_test.go`, `testvendors_test.go` | failure-first data/gate tests and in-memory test vendors |

### 7.4 Conformance digest

`internal/sourcehash` hashes every file under `internal/`, so the embedded data
and the rewritten adapter files moved the `internal` digest
`911e594c…` → `087b41ac…`. The five `internal`-rooted entries in
`sdk/internal/assembly/conformance_results.json` were updated in the same
commit (§5). Compute it after the last edit to any file under `internal/`,
because a later `gofmt` or comment change moves it again — `PROV-P2`..`PROV-P5`
must refresh it once more each.

Generation rollback (the Assembly-level rollback described in
`docs/plans/plugin-platform/PLG-P9-release-conformance.md`) is unaffected: it
selects a prior Generation, and each Generation carries its own sealed adapter
set.

---

## 8. As-built record (`PROV-P2`)

This section supersedes §1.1–§1.2, §2 of `DESIGN.md` where the design's letter
and the landed code differ. It was written while landing the phase, from the
actual diff.

### 8.1 The adapter table is the sealed set

`internal/provider/adapters.go` now owns `Adapter{Family, State}` and
`Adapters()`, and every other projection is derived from it: `Capabilities()`
(the ModelHost map), `AdapterFamilies()`, `AdapterState`, `IsSealedAdapter`.
`internal/app/app.go` builds the ModelHost capability map from
`provider.Capabilities()` instead of a hand-written two-entry literal, so the
compiled capability set and the adapter table cannot drift.

`ErrAdapterUnknown` and `ErrAdapterDeferred` replace `ErrAdapterFamilyMismatch`
and `legacyAdapterFamily`: the mismatch the old error described is no longer
representable, because the endpoint's `adapter` **is** the family.

### 8.2 What "resolve by adapter" means in the call path

| Call | Role |
|---|---|
| `Catalog.Adapter(family) (Ref, error)` | the sealed-set lookup. It returns a **vendor-neutral** Ref: address, credential and model id must come from the `ModelSpec`. Unknown and deferred families fail closed with distinct errors. |
| `Catalog.RefForEndpoint(vendor string, ep Endpoint) (Ref, error)` | the construction path the runtime uses. It also carries the vendor's `env_key` (for `KeyMissingError`) and supplies the endpoint's `base_url`/`default_model` when the stored selection leaves them empty. |
| `Catalog.AdapterFamily(vendor, baseURL) string` | the projection the availability surface needs while the stored selection is still vendor-keyed. |

`Catalog.ForProfile` is gone, so `providerprofile` is no longer imported by
`catalog.go`. The runtime gate and the availability marks in `resolving.go` are
keyed by `endpoint.Adapter`; the RPC status closure and `ModelResolver.Current`
use `AdapterFamily` to reach the same key.

`ProfileFromEndpoint` is deleted. `provider.AdapterProfiles()` computes one
Profile per sealed adapter, unioning the model ids and Secret references of every
embedded endpoint that speaks it, and `defaults.ProviderProfiles()` delegates to
it while keeping its name, signature and `defaultProviderOptionsSchema` — the
compiled Generation owns the option surface, so it replaces the minimal schema
the projection carries. The evidence anchors in
`sdk/internal/assembly/evidence.go:77-80` and the test name cited by
`sdk/internal/conformance/reproduction_test.go:456` therefore stay valid.

### 8.3 Two contract rules had to widen

| Rule | Was | Now | Why |
|---|---|---|---|
| `providerprofile.secretRefPattern` (`sdk/port/providerprofile/providerprofile.go:25`) | `^[A-Z][A-Z0-9_]*$` | `^[A-Z0-9][A-Z0-9_]*$` | The `openai-completions` union contains `302AI_API_KEY`. This is the same leading-digit case `config.ValidEnvKey` hit in P1; the two rules must agree, because the credential allowlist validates `SecretRefs` with `ValidEnvKey`. |
| `providerprofile.Validate` native-prefix check | derived `openai/` from the family `openai-compatible` | derives `openai-completions/` | The family is now the adapter id, so the forbidden prefixes are the adapter names. No embedded model id collides with them; P1's family name would have rejected any `openai/…` id in the union. |

### 8.4 Thinking rule, and the behaviour change it enables

`resolving.go` now holds `decideThinking(adapter, deepSeekCapability,
supportsThinking, mode) thinkingShape` — a pure function returning *what* the
request carries; `thinkingShape.options()` renders it as Eino options. The rule
is unchanged for Anthropic and DeepSeek.

The real change: an OpenAI-compatible endpoint that is **not** DeepSeek now sends
`reasoning_effort: high` for a model whose metadata declares thinking support,
on `auto` and `on`. It sent nothing before, because the only per-model flag was
DeepSeek's and the resolver returned early for every other vendor. The data
therefore declares `supports_thinking: true` for `o1-preview`, `o1-mini`,
`gpt-5.1`, `gpt-5`, `gpt-5-mini`, `gpt-5-nano`, `gpt-5-pro` and `o3-mini`
(the deferred endpoint). `gpt-5-chat` and `gpt-image-1` stay false:
`reasoning_effort` is not a parameter of either.

Because families are now deterministic, the reachable
`MarkUnavailable` case is no longer a family mismatch but a credential that
disappears between the readiness projection and construction; the conformance
subtest drives exactly that case and asserts the profile reports `UNAVAILABLE`.

### 8.5 Assembly, and one piece of transitional scaffolding

`internal/modules/defaults/catalog.go` Port identities are the three adapter ids,
and `internal/generated/assembly/zz_default.go` was regenerated — one line
changed, the sealed `ProviderProfiles` list. Generating twice produced
byte-identical output. `sdk/internal/testdata/default-generation.expected.json`
was updated in the same commit, as was
`internal/app/default_generation_test.go`'s assertion that the first compiled
Profile is the default chain's adapter.

`internal/app/app.go` gained `transitionalVendorNames`
(`deepseek`, `openai`, `anthropic`). The compiled Profiles are keyed by adapter
now, so this list is what keeps the pre-baked Settings/TUI `providers` payload
byte-identical for this phase. `PROV-P3` removes the per-vendor config blocks and
`PROV-P4` serves the whole embedded catalog here; the list dies with them.
`PROV-P3` did delete it as a hard-coded list — the payload then came from
`settings.LegacyVendorNames()`, derived from the normalization table and keeping
the same three vendors in the same order (§8.6). `PROV-P4` deleted that helper
too: the payload is the embedded catalog (§8.8).

### 8.6 Configuration, credentials, and selection (PROV-P3 as built)

Delivered in `refactor(provider): derive configuration and credentials from
provider data`:

| Item | As built |
|---|---|
| `config.Providers` | one field, `active`. `Provider`, the three blocks and their validation are deleted. `Default()` still yields `deepseek`. `validate` checks the shape; the app's startup gate checks membership in the loaded data (see below). |
| Removed-key hint | `Load` turns the strict-decoding error into one sentence naming the removed block: `providers.deepseek was removed in PROV-P3 — provider metadata is embedded in the binary, so keep providers.active only`. |
| `Catalog.EndpointForVendor` | three arguments now: `(vendor, adapter, baseURL)`. An empty adapter means "the vendor's default protocol"; a declared address wins; an address no endpoint declares keeps the vendor's endpoint for the requested adapter. |
| `Catalog.AdapterFamily` | deleted. The adapter comes from the endpoint (`Endpoint.Adapter`) or the stored selection. `Catalog.VendorForEndpoint(adapter, baseURL)` is the reverse lookup the address→vendor rule needs. |
| `ResolveModelInfo` | searches every endpoint of the vendor before falling back to the default endpoint, through the new pure `modelInfoFor(vendor, endpoint, modelID)`, so a deferred adapter resolves metadata without constructing anything. |
| `provider.Vendor.EndpointForAdapter` | new: the first declared endpoint for one protocol. |
| `provider.VendorEnvKeys` | new: the sorted, de-duplicated credential allowlist derived from the vendor data. |
| `provider.LiveSpec` / `app.ResolvedModel` | gained `Adapter`. `Provider` remains the vendor, so `Service.SetModel`, `ModelInfo`, `ModelMeta` and the sidebar keep their meaning. |
| `freezeFromEnv` | iterates the embedded vendors; the configured vendor wins when several keys are set; `VIVY_PROVIDER` names a vendor, or an adapter's first vendor. |
| `applySettingsEnv` | takes the catalog and writes the key into the resolved **vendor's** variable. |
| Deleted | `defaultModelFor`, `providerConfigBaseline`, the three-way switch in `applySettingsEnv`, `applySettingsOverlayAt`'s per-vendor `default_model` overlay, and `app.transitionalVendorNames`. |
| `app.settings.LegacyVendorNames` | the pre-baked Settings/TUI vendor list, now derived from the normalization table, so the `settings/providers` payload stays byte-identical until `PROV-P4` widens it. |
| `internal/app/settings/provider_migration.go` | the one normalization table plus `NormalizeProviderSelection`, `NormalizeAdapter`, `ValidProviderValue`, `ProviderValueError`, `LegacyVendorNames`, consumed by `Settings.Validate`, `validateProviderEntries`, `FindProvider`/`ActiveKey`, the registry uniqueness key, `IsOpenAICompatibleSelection`, the resolver and the RPC allowlist. |
| `internal/eval/isolator.go` | its candidate config names `providers.active` only. |

Two decisions worth recording:

1. **A stored value is not rewritten on write.** `MIGRATION.md` §3 rule 2 says the
   normalized value is written back on the next save. P3 stores what the client
   sent instead, because for an **undeclared** gateway address the vendor name is
   the only thing that identifies the credential owner: rewriting `provider:
   openai` to `openai-completions` would move that selection onto
   `config.providers.active`'s vendor. Both spellings stay readable, the new UI
   (P4) writes adapter values, and documents therefore converge as they are
   edited rather than by a rewrite no user asked for.
2. **The membership check for `providers.active` lives in the app, not in
   `config.Validate`.** `internal/config` must not import `internal/provider`
   (it is a leaf config package; the Eino quarantine keeps the registry out of
   it), so `Validate` checks the name's shape and `app.Compose` rejects a name the
   loaded data does not declare, immediately after `LoadEmbedded`.

### 8.7 Conformance digest

The `internal` digest moved `087b41ac…` → `978e0d42…` (P2) → `13f4ee33…` →
`a0811ff1…` (P3) → `35ff06a2…` → `0d24ebe4…` → `5e386f84…` (P4) and the five
`internal`-rooted entries in `sdk/internal/assembly/conformance_results.json`
were refreshed in the same commit each time. `PROV-P5` owns the final value.
The last move matters as a process note: `0d24ebe4…` was written before the
final `internal/app/app.go` edit, so it was already stale when the phase's gates
ran — exactly the manual step `PROVIDER-PROFILE-DIGEST-PIN` (TODO §0.1) tracks.
The canonical value is always `HashSourceTree(<repo>/internal, "")`, which is
what `sdk/internal/conformance/reproduction_test.go` computes; recompute it after
the last edit under `internal/`, not before.

### 8.8 The catalog on the wire, and a zero-data UI (`PROV-P4` as built)

**The payload.** `settings/providers` returns `catalog`, and `bundles` is gone
from the wire:

```jsonc
"catalog": [
  { "vendor": "deepseek", "display_name": "DeepSeek",
    "endpoints": [
      { "adapter": "openai-completions", "base_url": "https://api.deepseek.com",
        "default_model": "deepseek-flash", "models": ["deepseek-flash", "…"],
        "executable": true, "state": "SUPPORTED" } ] } ]
```

`internal/rpc/control.go` builds it from `Deps.ProviderVendors` through
`providerCatalogResult`, which reads `provider.Capabilities()` for `state` and
sets `executable` to `state == modelhost.CapabilitySupported` — the same
predicate `ModelHost.ResolveExecutable` applies, so "visible but not selectable"
cannot drift from "cannot be constructed". The payload crosses the boundary with
no credential field at all: no `env_key`, no `api_key`, no value.

**The frontend's bundled catalog is deleted, not duplicated.** `app.Compose` now
hands the *embedded* catalog to the control plane
(`executableVendors := catalog.Vendors()`), so `settings.LegacyVendorNames` — the
transitional three-vendor list that kept the P3 payload byte-identical — is
deleted along with its test, and the vendors a client can offer are the data
itself. The TUI (`sdk/tui/live/rpc.go`) reads `catalog` too: one option per
executable endpoint, `provider` = the adapter and `base_url` = the declared
address, which is exactly what `settings/model/select` stores; a deferred
endpoint is skipped rather than offered and then rejected.

**Data fidelity, checked against the source before the generator dies.** The
embedded `internal/provider/data/vendors.yaml` was compared with the upstream
Agent-Diva registry it was ported from (`ui/agent-diva-source/…/providers.yaml`,
47 entries, read-only; the checkout is gitignored and absent from this lane):

| Check | Result |
|---|---|
| Vendors | 45 = 47 − `custom` (not a vendor) − `cherryin` (no models, no default model). Both drops are the ones `MIGRATION.md` §7.2 and `internal/provider/vendor_test.go` already record; the plan's Task 4 text (46) is the pre-P1 count. |
| Display names | identical for all 45. |
| Model ids | no upstream id is dropped. Every addition is the vendor's own `default_model` (gateway prefix stripped) that upstream's `models` list omitted, plus `o3-mini`/`gpt-5`/`gpt-5-mini` on the hand-authored deferred Responses endpoint of `openai`, plus `vllm`'s default model (upstream lists none). |
| Addresses | every vendor's upstream `default_api_base` is one of its endpoints. Two addresses were added or repaired by the port: `aionly` (upstream's value carries a UTF-8 BOM, the embedded one does not) and `deepseek`'s second endpoint `https://api.deepseek.com/anthropic` (DeepSeek speaks both protocols by design). |

The generator script `ui/scripts/gen-provider-catalog.py` is deleted and the
`AUTO-GENERATED` header is gone with the array it described.

**The Face contract had to widen too.** `sdk/ui/src/module.ts` is the published
`@vivy/ui-sdk` contract, and `ui/src/lib/ui-sdk-face-compat.test.ts` asserts it
*exactly* in both directions, so the UI could not change without it. The
provider types now name the real vocabulary: `FaceProviderAdapter` (the three
sealed ids), `FaceLegacyProviderBundle`, `FaceProviderValue` for
`FaceProviderEntry.bundle` / `FaceProviderEntryInput.bundle` /
`FaceProviderRefreshInput.bundle` (whose pre-P3 `"openai" | "deepseek"`
whitelist is gone, because the refresh rule is now "the endpoint's adapter is
`openai-completions`"), plus `FaceProviderEndpoint` /
`FaceProviderCatalogEntry` and `catalog` on `FaceProvidersView` and
`FaceStoreState`. The package version stays `1.0.0`: it is a build pin
(`sdk/ui/version.go`, `UI_BUILD_MANIFEST`), the change is additive for module
authors, and no rule in `VIVY-FACE-PACK.md` versions the contract separately.

Two implementation constraints are worth recording because they are not obvious
from the type definitions. `ui/node_modules/@vivy/ui-sdk` is a hard-linked copy
that pnpm materializes from `sdk/ui`, so an edit to `module.ts` is invisible to
`tsc` until `pnpm install --frozen-lockfile` runs in `ui/` (`just ui-core` does
that first, which is why CI sees it). And `FaceStoreState` must stay *mutually*
assignable with the implementation's own store state, because zustand's
`subscribe` is a property with call signatures rather than a method, so its
listener parameter is checked contravariantly: `catalog` and
`FaceProviderCatalogEntry.endpoints` are therefore mutable arrays, and
`FaceProviderEndpoint.state` is the exact `ProviderAdapterState` union rather
than `string`.

---

## 9. Migration sequence summary

```text
PROV-P1  data + embed + strict validation + startup consistency gate; delete fixtures/ and bundle_dir
PROV-P2  adapter table + sealed manifest + catalog-by-family + thinking capabilities; regenerate zz_default.go
PROV-P3  config shrink + credential/env-key source + settings.yaml aliases + data-derived selection   [done]
PROV-P4  catalog RPC + UI zero-data + loading state; delete the generator script and ui/agent-diva-source   [done]
PROV-P5  evidence, sourceSha256, TODO rows, iteration log, just ci   [done]
```

### 8.9 Landing (executed 2026-09-18)

The owner authorized the landing; the branch was fast-forwarded into `main` and
pushed. The branch landed as `77be9c5`, eleven commits ahead of `main`'s
then-tip `ff8a47d` — `9c6f8c7` (plan), `3025405` (P1), `885382e` (P2), `ac9da30`
(P3), `8c62886` (P4), `7a833f0` (P5 closeout), `c1da466`/`dd759dc`/`c22584e`
(the diagnosis, inventory and reflection in `notes.md`), `4755902` (the
`AGENTS.md` rule), `77be9c5` (this landed record) — and `main` had no commit the
branch lacked. The commit carrying this note is a docs-only follow-up pushed on
top of the landing. The `just ci` run recorded in
`docs/logs/2026-09-18-provider-registry/verification.md` was made on the code
tree at `8c62886`; the commits after it touch `AGENTS.md` and `docs/` only
(`git diff --stat 8c62886..HEAD` lists no code path), so no re-run was required.

Procedure actually used:

```text
git checkout main                          # root checkout, at ff8a47d
git merge --ff-only feat/provider-registry
git push origin main
```

Two things the operator should know:

- `origin/main` was **eight commits behind** local `main` before this push
  (`fe60b18` → `ff8a47d`): the DeepSeek-default commit and seven docs/scheduling
  commits that had never been pushed. Pushing `main` publishes them too — that is
  inherent to publishing the branch, not a side effect of this landing.
- CI on the pushed tip (run `35361729156`): `ui ci` passed (1m35s) and
  `backend ci` passed (19m46s, including the Go tests, the conformance suite and
  the `internal` digest on the pushed tree). `full UI browser smoke` failed with
  `Error: Timed out waiting 30000ms from config.webServer.`, and the `just ci`
  aggregator job failed in 4s only because it is
  `test "$BROWSER_RESULT" = "success"`. **That failure is not from this branch**:
  the job was added by `082f0d9` (PR #30, 2026-09-15) and `main`'s CI has been
  red since — the same signature is on the previous `main` tip `fe60b18`
  (run `34944037203`), while a PR-branch run on the same day passed
  (`eb8fee3`). Cause: Playwright's own `webServer` starts the **default**
  generation with `go run ./cmd/vivy` on `127.0.0.1:8799` under a 30s timeout,
  and the spec also drives the packed full UI the job starts on `:3015`
  (`VIVY_FULL_UI_URL`); 30s has to cover a cold `go build` of this repository on
  a Windows runner, which it usually cannot. Tracked with the fix direction as
  `CI-BROWSER-SMOKE-WEBSERVER` in `docs/TODO.md` §0.1.
- The root checkout carries another lane's untracked files under `internal/`
  (`internal/workflow/`, `internal/domain/workflow_test_support.go`). They are
  outside this branch, but because `internal/sourcehash` hashes every file under
  `internal/`, a `just ci` run *in that checkout* computes a different digest
  than the committed `5e386f84…`. The committed value is correct for the tree
  this branch landed; the root checkout's value is tracked by
  `PROVIDER-PROFILE-DIGEST-PIN` in `docs/TODO.md`.

Notes unchanged from the proposal:

- `ui/agent-diva-source/` was removed from the launch checkout during P4. It is
  gitignored, so it cannot travel with the branch; the removal is already done on
  the machine, and a fresh clone simply will not have it.
- The lane's `data/` and `.env` are per-checkout scratch (`.env` only held
  `VIVY_DEFAULT_LOCALE=zh` for the e2e run); neither is committed.
- If a revert is ever needed, `MIGRATION.md` §6 pairs the P4 backend and UI in
  one revert; a split state leaves the UI without a catalog.
- The worktree was left in place for review; removing it
  (`git worktree remove .worktrees/provider-sot`) is a separate housekeeping step.
