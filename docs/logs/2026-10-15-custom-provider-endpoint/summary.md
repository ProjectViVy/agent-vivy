# Custom provider endpoint selection

## What failed

Selecting a provider registered in **Settings → Models** ("add custom provider")
made every run fail with the generic UI message:

```text
这次对话没有完成
The model run could not be completed. Please try again.
```

The backend log named the cause (`vivy.log.<date>`, WARN `run failed`):

```text
err="engine event error: [NodeRunError] provider \"WXYG\": no embedded vendor data
------------------------
node path: [node_1, ChatModel]"
```

`WXYG` was the registry entry's **display name**, not an embedded vendor.

## Root cause

`resolveStoredSelection` (`internal/app/provider_selection.go`) already
recognised a user-registered provider: an address no embedded vendor declares
plus a matching registry entry was labelled by that entry's display name "so
run/usage events do not blame the configured vendor". It then handed that label
to `resolveVendorSelection`, which asked the embedded catalog for a vendor of
that name. The catalog holds 45 reviewed vendors and never a registry alias, so
resolution reported not-ok and the run path reached
`Catalog.EndpointForVendor` → `provider "WXYG": no embedded vendor data`.

Two layers disagreed about what a custom provider is:

- the settings/registry layer treats it as an **endpoint** (adapter, address,
  model list, key) that the runtime can execute;
- the catalog layer only knows **vendors**, and a registry entry is not one.

The existing test pinned the defect as intended behaviour
(`resolveStoredSelection` returning `ok=false` for a registered provider), so no
gate could catch it.

## Fix

A user-registered provider is now resolved from its own registry entry, and the
resolved endpoint travels with the selection into the run path. The embedded
catalog is never asked about a name it cannot know.

| File | Change |
|---|---|
| `internal/app/provider_selection.go` | a registry-entry match returns `resolveCustomSelection` + `customEndpoint`: the selection carries the entry's adapter, address, model list and default model, and the alias only labels it |
| `internal/app/model.go` | `ResolvedModel` carries `Endpoint`; `Live()` forwards it |
| `internal/provider/resolving.go` | `LiveSpec` carries `Endpoint`; `inner()` builds the model from it for a registered endpoint and keeps the catalog/vendor path for embedded selections; `thinkingOptions` sends no reasoning field for a registered endpoint |
| `internal/provider/catalog.go` | new `Catalog.RefForSelection(name, endpoint)`: the adapter gate still applies, the credential is the key the caller already resolved, and `name` is never looked up |

Capabilities stay empty for a registered endpoint — the registry declares models,
not endpoint behaviour — so no `deepseek-thinking` or `reasoning_effort` is
invented for an upstream that never advertised it. The credential remains the
registry key resolved by the same `(adapter, base_url)` identity
(`settings.ActiveKey`), which is why `Ready` was already true and no
`KeyMissingError` appears.

## Explicitly not done

- **No model metadata for registered models.** `ResolveModelInfo` still reports
  zero (context window unknown) for a registry endpoint. The registry carries
  model ids only, so nothing is invented; callers keep their conservative
  defaults, as for any unknown model. Adding per-model metadata to the registry
  is a separate product decision.
- **No environment synchronization for registered providers.** `applySettingsEnv`
  skips them (there is no vendor `env_key` to write); the resolver reads the key
  from the document per call, which is the pre-existing behaviour.
- **No UI change.** The panel, the wire RPC and the `settings/model/select`
  permission check already accept a registry entry by `(adapter, base_url)`; only
  the runtime path was broken.
- **`openai-responses` remains `DEFERRED-INDEFINITE`.** A registered endpoint
  naming it now reaches the same sealed-adapter gate as any other selection and
  fails closed there.

## Scope

Kernel only (`internal/app`, `internal/provider`). No RPC, UI, storage,
migration or product-contract change. `internal/provider/data/vendors.yaml` is
untouched: a user registers a gateway, it does not become embedded data.
