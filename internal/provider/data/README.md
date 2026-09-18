# Provider data (`internal/provider/data/`)

This directory is the **single source of truth for provider configuration
data** in Vivy (decision D1). `vendors.yaml` is embedded into the binary at
build time (`internal/provider/embed.go`, `//go:embed data/*.yaml`) and
validated at startup. There is no runtime directory setting, no
working-directory dependency, and no file a running instance can be pointed
at: a running process never reads provider data from disk (D3).

`provider.schema.json` documents the document for editors and reviewers. The
authority is the Go validator in `internal/provider/vendor.go`; both must be
updated together.

## The shape

Four layers, only the first of which is code (D4):

| Layer | Answers | Form |
|---|---|---|
| **Adapter** | Which wire API do we speak? | Go code, sealed (`internal/provider/adapters.go`) |
| **Vendor** | Whose endpoint, whose key? | `vendors.yaml` |
| **Endpoint** | Which adapter, which address, which models? | `vendors.yaml`, 1..N per vendor |
| **Model** | Which model id, how big, how expensive? | `vendors.yaml`, N per endpoint |

Three adapters exist and no more: `openai-completions`, `openai-responses`
(**deferred, not implemented**), `anthropic-messages`. Data may name one of
these; it can never introduce an adapter, a transport, or executable
behaviour.

## Editing rules

- **One file.** No provider YAML lives anywhere else in the repository.
- **Strict.** An unknown key is a hard error. Validation collects every
  failure, so one run tells you everything that is wrong.
- **Secrets never live here.** `env_key` names an environment variable; it is
  never a value (D-010).
- **Raw model ids only.** Send the endpoint's own model id; never a
  `vendor/model` prefix. The validator rejects a model id that starts with the
  vendor's own name or the adapter's name. Prefixes are legitimate only when
  they are genuinely part of the id (for example an aggregator's
  `anthropic/claude-sonnet-4`).
- **`default_model` must be listed in that endpoint's `models`.**
- **`(adapter, base_url)` is the endpoint identity** and must be unique across
  the file. This is what makes a vendor speaking two protocols ordinary data:
  add a second endpoint entry with the other `adapter` and its own `base_url`.
- **Every vendor carries `provenance`** with `source`, `entry`, `derived_at`
  (D-025).

## How to add a vendor

Add one entry to the top-level array, keeping the file alphabetical:

```yaml
- name: example                    # ^[a-z][a-z0-9_-]*$, unique
  display_name: Example            # UI label
  env_key: EXAMPLE_API_KEY         # ^[A-Z][A-Z0-9_]*$
  endpoints:
    - adapter: openai-completions  # one of the sealed three
      base_url: https://api.example.com/v1
      default_model: example-chat  # must appear in models below
      models:
        - id: example-chat
  provenance:
    source: <where the values came from>
    entry: example
    derived_at: "YYYY-MM-DD"
    note: <what was changed or normalized>
```

## How to add an endpoint variant (a second protocol for one vendor)

Append a second entry under the same vendor's `endpoints` with the other
`adapter`, its own `base_url`, its own `default_model` and its own `models`.
The vendor's `env_key` is shared: that is the whole point of the vendor layer,
and it is why DeepSeek's Anthropic-compatible endpoint can reuse
`DEEPSEEK_API_KEY` instead of borrowing `ANTHROPIC_API_KEY`.

## Metadata: absent means unknown, never guessed

Model metadata (`context_window`, `input_per_mtok`, `output_per_mtok`,
`supports_images`, `supports_thinking`) is inline on each model entry and is
optional. **An absent field means Vivy does not know**, and every consumer
keeps its conservative default — `context_window: 0`/absent falls back to
`128000` (`internal/runtime/compaction_policy.go`), an unpriced model is
unpriced (never free), and an unknown `supports_thinking` sends no reasoning
parameters. Do not fill in a plausible-looking number.

Only the three first-party vendors (`deepseek`, `openai`, `anthropic`) carry
metadata, because those are the values the previous Go tables already
asserted. The other 42 vendors deliberately report unknown; declaring metadata
for them is a data review, not a derivation.

## What fails startup

`internal/provider/reconcile.go` compares this file against the sealed adapter
set in both directions (D15). Data naming an unsealed adapter, or a sealed
adapter with no endpoint anywhere, aborts startup. A data edit therefore
cannot silently change which protocols the build speaks.

## Provenance of the current content

`vendors.yaml` was derived once from the Diva provider catalog
(`ui/agent-diva-source/agent-diva-providers/src/providers.yaml`, 47 entries)
on 2026-09-18 and the repository then stood alone (D13). The derivation:

- dropped `custom` (a local-endpoint placeholder with an empty
  `default_model`) and `cherryin` (the source declares `models: []` and no
  `default_model`, so it cannot satisfy the vendor contract and has no
  selectable model; it remains reachable as a user-defined custom provider) —
  47 entries became **45 vendors**;
- dropped the 10 fields with no consumer plus `api_type` and the redundant
  `backend`, mapping `api_type: openai` → `openai-completions` and
  `api_type: anthropic` → `anthropic-messages`;
- stripped the vendor's own name prefix from 10 `default_model` values;
- filled 33 empty `default_model` values from the endpoint's first model,
  reproducing the pre-existing UI catalog's defaults;
- removed one stray leading colon from an upstream `base_url`;
- declared DeepSeek's Anthropic-compatible endpoint and OpenAI's deferred
  `openai-responses` endpoint;
- left every other model's metadata absent.

The current content is **45 vendors, 47 endpoints, 168 model entries**. The
provenance record on each vendor names its source entry.