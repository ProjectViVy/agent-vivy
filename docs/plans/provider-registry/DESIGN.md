# Provider Registry — Target Design

**Status:** authored 2026-09-18, part of the documentation-only batch. No code is
described here that has been written yet. Every phase that acts on this document
is `UNSCHEDULED` until a human schedules it.

**Audience:** the engineer implementing `PROV-P1`..`PROV-P5`.

---

## 1. Why this exists (the problem, with evidence)

The current provider layer stores the same fact in several places and guards none
of them. The table below is the inventory taken on 2026-09-18.

| Fact | Copy 1 | Copy 2 | Copy 3 | Guarded? |
|---|---|---|---|---|
| Provider set | `internal/app/app.go:254-272` (three hard-coded names) | `internal/modules/defaults/providers.go:19-38` | `internal/generated/assembly/zz_default.go:79` | profile↔manifest yes; **bundle↔profile no** |
| Adapter family | `fixtures/provider/*.yaml` `backend` | `internal/modules/defaults/providers.go` `AdapterFamily` | `internal/provider/catalog.go:68-75` | only lazily, at `Catalog.ForProfile` (`catalog.go:64-67`) |
| `env_key` | YAML `env_key` | `internal/config/config.go:608-610` | `Profile.SecretRefs` | **none** |
| `default_model` | YAML `default_model` | `internal/config/config.go:608-610` | `ui/src/components/settings/provider-catalog.ts` | **none** |
| model list | YAML `models` | `Profile.ModelIDs` | `ui/src/components/settings/provider-catalog.ts` | **none** |
| base URL | YAML `default_api_base` | `ui/src/components/settings/provider-catalog.ts` | — | **none** |

Two consequences worth stating plainly:

1. `internal/config/config.go:734` validates `env_key` against
   `^[A-Z][A-Z0-9_]*$`, which an empty string fails. So `config.Providers.*.env_key`
   is a **mandatory third copy**, not an optional override. No production code
   compares it with the bundle's `env_key` or with `Profile.SecretRefs`; the only
   equality assertions live in tests.
2. The adapter-family guard is lazy: a `backend` ↔ `AdapterFamily` drift surfaces
   on the first model call (`resolvingChatModel.inner` → `Catalog.ForProfile`),
   not at startup.

A third problem is structural rather than a duplication: the sealed unit is the
**vendor**. `deepseek` is a sealed Profile whose family is `openai-compatible`,
so DeepSeek's Anthropic-compatible endpoint cannot be expressed — reusing the
`anthropic` Profile would draw its key from `ANTHROPIC_API_KEY`
(`internal/modules/credential/module.go:80`, `internal/app/app.go:1237`) instead
of `DEEPSEEK_API_KEY`. The sealed object should be the **protocol adapter**; a
vendor is data.

---

## 2. Target model

Four layers, each with one owner and one lifetime.

| Layer | Answers | Count | Form | Sealed |
|---|---|---|---|---|
| **Adapter** | Which wire API do we speak? | **3** | Go code | yes (compile time) |
| **Vendor** | Whose endpoint, whose key? | 46 | data | no |
| **Endpoint** | Which adapter, which address, which models? | 1..N per vendor | data | no |
| **Model** | Which model id, how big, how expensive? | N per endpoint | data | no |

### 2.1 Adapters

| Family | Wire API | Backing component (pinned) | State |
|---|---|---|---|
| `openai-completions` | `POST {base}/chat/completions` | `eino-ext/components/model/openai v0.1.13` | `SUPPORTED` |
| `openai-responses` | `POST {base}/responses` | `eino-ext/components/model/agenticopenai v0.2.2` (`NewResponsesModel`) | `DEFERRED-INDEFINITE` |
| `anthropic-messages` | `POST {base}/messages` | `eino-ext/components/model/claude v0.1.25` | `SUPPORTED` |

The `openai-responses` deferral, its evidence, and the committed migration path
are in `EINO-CAPABILITY.md`. A deferred family is declared and shown as such; it
is never silently omitted and never backed by a custom substitute.

### 2.2 Invariants

1. **The sealed unit is the capability, not the vendor.** Engineering a new
   vendor is a data edit. Adding a fourth wire protocol requires code plus a
   sealed-manifest change.
2. **The running process never reads provider data from disk.** Data is embedded
   at build time (`//go:embed`); there is no directory path, no working-directory
   dependency, and no file a running instance can be pointed at.
3. **Data supplies data, never capability.** A vendor entry can name an adapter
   from the sealed three, an address, models, and metadata. It cannot introduce an
   adapter, a transport, or executable behaviour.

---

## 3. Data schema

One file carries every vendor. A separate JSON Schema file documents it and is
the review surface for edits.

```yaml
# internal/provider/data/vendors.yaml
- name: deepseek                        # vendor id, ^[a-z][a-z0-9_-]*$
  display_name: DeepSeek               # UI label
  env_key: DEEPSEEK_API_KEY            # credential allowlist source + KeyMissingError text
  endpoints:                           # 1..N, unique by (adapter, base_url)
    - adapter: openai-completions      # one of the sealed three
      base_url: https://api.deepseek.com
      default_model: deepseek-flash
      models:
        - id: deepseek-flash
          context_window: 1000000
          input_per_mtok: 0.30
          output_per_mtok: 1.20
          supports_images: true
          supports_thinking: true
        - id: deepseek-chat            # metadata absent = unknown
      supports_prompt_caching: false
      capabilities: []                 # endpoint-level capability flags
    - adapter: anthropic-messages
      base_url: https://api.deepseek.com/anthropic
      default_model: deepseek-chat
      models:
        - id: deepseek-chat
      supports_prompt_caching: true
  provenance:
    source: agent-diva/agent-diva-providers/src/providers.yaml
    entry: deepseek
    derived_at: "2026-09-18"
    note: "Copied into the Vivy schema; gateway_prefix/detect_* dropped."
```

### 3.1 Fields and their consumers

Every field below has a named consumer. Anything without one is deleted.

| Field | Owner | Consumers |
|---|---|---|
| `name` | vendor | Profile projection id, `config.providers.active`, UI row identity |
| `display_name` | vendor | RPC `providersView`, UI labels |
| `env_key` | vendor | `credentialmodule.CompileScopes` allowlist, `KeyMissingError.EnvKey`, `applySettingsEnv` env target |
| `endpoints` | vendor | UI variant list, resolver endpoint lookup |
| `adapter` | endpoint | adapter lookup in the sealed table, `Profile.AdapterFamily` |
| `base_url` | endpoint | `ModelSpec.BaseURL`, `/models` discovery, endpoint identity |
| `default_model` | endpoint | `ModelSpec.ID` fallback, UI default, config default |
| `models` | endpoint | UI model list, `settings/update` allowlist, `ProviderEntry` seed |
| `models[].id` | model | outbound `model` field |
| `models[].context_window` | model | compaction policy, sidebar usage |
| `models[].input_per_mtok` / `output_per_mtok` | model | token cost statistics |
| `models[].supports_images` | model | image attachment gate |
| `models[].supports_thinking` | model | reasoning controls in the UI, thinking option gate |
| `capabilities` | endpoint | adapter request shaping (see §3.3) |
| `supports_prompt_caching` | endpoint | `einoclaude.CacheControl` on the Anthropic path |
| `provenance` | vendor | D-025 review record |

### 3.2 Deleted fields

These are declared in `internal/provider/bundle.go` today and have **zero Go
consumers** (verified by grep over the repository). They are not carried into the
new schema.

| Deleted | Reason |
|---|---|
| `keywords` | no Go or UI consumer; the UI searches `name`/`display_name` |
| `gateway_prefix` | no consumer; Vivy never inserts a gateway prefix |
| `skip_prefixes` | no consumer |
| `env_extras` | no consumer |
| `is_gateway` | no consumer; gateway semantics are unimplemented (22 of the 47 upstream entries set it) |
| `is_local` | no consumer |
| `detect_by_key_prefix` | no consumer |
| `detect_by_base_keyword` | no consumer |
| `strip_model_prefix` | no consumer; prefix stripping happens once, during data derivation |
| `model_overrides` | declared but never read |
| `api_type` | superseded by `adapter`, which is strictly more precise |
| `backend` | redundant with `adapter` (`eino-ext/openai` / `eino-ext/claude`) |

`provenance` is **kept** (required on every vendor) so product rule D-025
continues to hold unchanged; it moves from one record per bundle file to one
record per vendor entry.

### 3.3 Capabilities

An endpoint may declare named capability flags. The set is closed and validated
against the adapter: a flag is only legal on an adapter that implements it.

| Capability | Legal adapters | Effect |
|---|---|---|
| `deepseek-thinking` | `openai-completions` | adds `{"thinking":{"type":"enabled"}}` to reasoning requests (today's `resolving.go:156-158` behaviour) |

Rationale: DeepSeek requests reasoning through the Chat Completions API with two
fields — `thinking` **and** `reasoning_effort` — while OpenAI's reasoning models
accept `reasoning_effort` alone. A single `openai-completions` adapter cannot
express both, and a fourth adapter was rejected. One named flag is the smallest
closed vocabulary that keeps the three-adapter model intact. It is a **named
enum**, not free-form request injection: raw JSON is never accepted from data.

### 3.4 Validation (all rules fail closed, all errors joined)

Load-time rules, mirroring and extending `internal/provider/bundle.go:98-150`:

1. `name` matches `^[a-z][a-z0-9_-]*$`; vendor names are unique.
2. `env_key` matches `^[A-Z][A-Z0-9_]*$` (reuse `config.ValidEnvKey`).
3. `display_name` is non-empty after trimming.
4. `endpoints` is non-empty; `(adapter, base_url)` is unique across the file.
5. `adapter` is one of the sealed three.
6. `base_url` is an absolute `http`/`https` URL.
7. `default_model` is non-empty and appears in `models`.
8. Model ids are non-empty, contain no NUL/CR/LF, are unique within the endpoint,
   and carry no `<vendor>/` or `<family>/` prefix (the rule already enforced by
   `sdk/port/providerprofile/providerprofile.go:78-85`).
9. Every entry in `capabilities` is legal for the endpoint's adapter (§3.3).
10. Unknown keys are rejected (strict YAML decoding, as today).
11. Every vendor carries `provenance.source` / `entry` / `derived_at`.

Startup rules (the consistency gate, decision D15):

12. The adapter set referenced by the data equals the sealed compiled adapter set,
    compared **bidirectionally**. A data entry naming an undeclared adapter, or a
    sealed adapter absent from the data, aborts startup.
13. For each sealed `SUPPORTED` adapter, at least one endpoint exists. A
    `DEFERRED-INDEFINITE` adapter may legitimately have endpoints declared (so the
    UI can show them greyed out) or none.

---

## 4. Identity, selection, and data flow

### 4.1 Endpoint identity

An endpoint is identified by `(adapter, base_url)`. This is the same tuple the
current code already uses everywhere it matters:

- `settings.ProviderEntry` uniqueness key `e.Bundle + "\x00" + e.BaseURL`
  (`internal/app/settings/settings.go:765`)
- `Settings.FindProvider(bundle, baseURL)` (`settings.go:780`)
- `settings.ActiveKey(s, provider, baseURL)` (`settings.go:793`)
- the UI's `matchMergedProviderEntry(providers, provider, base_url, …)`

So the only change is vocabulary: the first element becomes an adapter name
instead of a vendor-derived bundle name.

### 4.2 Selection payload (unchanged shape)

```ts
{ provider: 'openai-completions', base_url: 'https://api.deepseek.com', default_model: 'deepseek-flash' }
```

`settings/update` already accepts exactly this triple
(`internal/rpc/control.go:4560-4623`); not one field is added or removed.
`base_url` remains the discriminator that selects an endpoint variant, which is
why multi-protocol vendors need no new wire concept.

### 4.3 Data flow

```
UI click on a model row
  -> providerSelection(entry, model) = {provider: <adapter>, base_url, default_model}
  -> rpc settings/update
        allowlist check (control.go:4582-4619): model must be
          (a) the config default pair with empty base_url, or
          (b) present in a registry entry for the same (adapter, base_url), or
          (c) present in the embedded endpoint's model list when base_url is empty
  -> settings.yaml
  -> ModelResolver.Current() -> Live()
  -> resolvingChatModel.inner()
        host.ResolveExecutable(vendor endpoint)      (capability state check)
        catalog.Adapter(adapterFamily)               (sealed adapter lookup)
        adapter.Model(ctx, ModelSpec{ID, APIKey, BaseURL})
  -> eino component -> HTTP -> vendor endpoint
```

The embedded data first participates at the `catalog.Adapter` step, which is the
same point where the bundle YAML participates today.

---

## 5. Default chain

| Level | Key | Default | Overridable by |
|---|---|---|---|
| Vendor | `config.providers.active` | `deepseek` | `settings.yaml` `provider` |
| Endpoint | `settings.yaml` `base_url` | empty = the vendor's first declared endpoint | user |
| Adapter | `settings.yaml` `provider` | `openai-completions` | user |
| Model | `settings.yaml` `default_model` | the endpoint's `default_model` | user |

Effective default today becomes: vendor `deepseek`, adapter
`openai-completions`, endpoint `https://api.deepseek.com`, model
`deepseek-flash` — i.e. OpenAI-standard streaming output, exactly the requested
default. Switching to another protocol is switching the endpoint variant of the
same vendor; no new setting is introduced.

### 5.1 `settings.yaml` migration

The value of `provider` changes meaning from "bundle name" to "adapter name", so
existing documents need a three-row mapping (decision D9):

| Stored value | Normalized to |
|---|---|
| `deepseek` | `openai-completions` |
| `openai` | `openai-completions` |
| `anthropic` | `anthropic-messages` |

Normalization happens on read and is persisted on the next save. The same mapping
applies to `providers[].bundle` in the user registry.

---

## 6. Model metadata and the unknown case

Metadata is inline on each model entry. Phase 1 fills only what the current Go
tables already know (`internal/provider/openai.go:48-68`,
`internal/provider/claude.go:88-103`); every other model stays empty.

**Unknown semantics (unchanged, now documented in the data instead of hidden in
code):** `context_window: 0` means unknown, and every consumer keeps its existing
conservative default.

| Consumer | Location | Unknown-model behaviour |
|---|---|---|
| Compaction policy | `internal/app/app.go:562`, `internal/runtime/compaction_service.go:104` | falls back to `fallbackContextWindowTokens = 128000` (`internal/runtime/compaction_policy.go:17`) |
| Thinking gate | `internal/provider/resolving.go:140,149` | no reasoning parameters sent |
| Token cost statistics | `internal/rpc/tokenstats.go` | unpriced |
| Sidebar usage | `internal/rpc/sidebar.go:109` | computed against 128000 |
| Image attachment gate | `internal/rpc/control.go:2886-2889` | allowed (deliberate D9 fail-open: unknown must not break custom gateways) |

A later phase may let a user declare metadata on a registry entry (decision D17);
that is explicitly out of phase-1 scope.

---

## 7. Availability projection

The existing machinery already supports the states this design needs; no new
concept is introduced.

| Condition | `ProfileState` | `CapabilityState` | UI |
|---|---|---|---|
| Adapter compiled and supported, not configured | `UNCONFIGURED` | `SUPPORTED` | selectable, prompts for a key |
| Adapter supported and a key resolves | `READY` | `SUPPORTED` | selectable, marked current |
| Adapter construction failed earlier | `UNAVAILABLE` | `SUPPORTED` | selectable, shows failure state |
| Adapter declared, no pinned implementation | `DEFERRED-INDEFINITE` | `DEFERRED-INDEFINITE` | rendered and disabled; `isProviderExecutable` excludes it (`ui/src/components/settings/provider-catalog.ts:35-45`) |

This is why `openai-responses` can ship in the data immediately: declaring it
yields a correct, visible, non-executable row.

---

## 8. Worked examples

### 8.1 DeepSeek (default; two protocols)

```yaml
- name: deepseek
  display_name: DeepSeek
  env_key: DEEPSEEK_API_KEY
  endpoints:
    - adapter: openai-completions        # default: OpenAI-standard stream
      base_url: https://api.deepseek.com
      default_model: deepseek-flash
      models: [deepseek-flash, deepseek-v4-pro, deepseek-v4-flash, deepseek-chat, deepseek-coder]
      capabilities: []
    - adapter: anthropic-messages        # DeepSeek's Anthropic-compatible endpoint
      base_url: https://api.deepseek.com/anthropic
      default_model: deepseek-chat
      models: [deepseek-chat]
      supports_prompt_caching: true
```

`deepseek-reasoner` moves under the `openai-completions` endpoint with
`capabilities: [deepseek-thinking]` and `supports_thinking: true`.

### 8.2 OpenAI (two protocols, one deferred)

```yaml
- name: openai
  display_name: OpenAI
  env_key: OPENAI_API_KEY
  endpoints:
    - adapter: openai-completions
      base_url: https://api.openai.com/v1
      default_model: gpt-4o
      models: [gpt-4o, gpt-4o-mini, gpt-4-turbo, gpt-4, o1-preview, o1-mini, gpt-5.1, gpt-5, gpt-5-mini, gpt-5-nano, gpt-5-pro, gpt-5-chat]
    - adapter: openai-responses          # declared, DEFERRED-INDEFINITE
      base_url: https://api.openai.com/v1
      default_model: gpt-5
      models: [gpt-5, gpt-5-mini, o3-mini]
```

The second endpoint exists so the capability is visible and honestly labelled; it
is not selectable until the deferral is lifted.

---

## 9. Rejected alternatives

| Alternative | Why rejected |
|---|---|
| Two adapters plus a model-level `dialect` dimension | Owner decision: three conventional protocol names are easier to reason about and match vendor documentation. It also forced a vendor→endpoint inheritance/override rule for `models`, which this design eliminates. |
| A fourth adapter `deepseek-reasoner` | The difference is one request field. A named endpoint capability is a smaller closed vocabulary than another adapter. |
| Runtime read of `vendors.yaml` from disk | Reintroduces working-directory coupling and lets an external file change a running instance's provider behaviour. |
| YAML → generated Go constants with a CI diff guard | Needs a generator, a checked-in artifact, and a guard, and buys nothing over `//go:embed`; the embedded file is already the exact reviewed artifact. |
| Keeping the PLG-P5 shape (per-vendor Profiles) | The sealed unit would remain the vendor, which is the defect being fixed: a vendor with two protocols cannot be expressed without inventing `deepseek-anthropic`. |
| One flat entry per `(vendor, protocol)` with no vendor layer | Duplicates `display_name` and `env_key` across the endpoints of every multi-protocol vendor, and loses the "configuration holds vendor information" shape the owner asked for. |
| Carrying all 17 upstream fields | Ten have no consumer; carrying them would import several hundred lines of inert data. |
