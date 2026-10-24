# Settings → Models: refresh and manual model ids update the row in place

## Problem

In Settings → Models, refreshing an upstream model list or manually adding a
model id left the visible model list (and the current-model check) on the old
content. For a built-in vendor row such as DeepSeek, changing the model could
not be made visible at all: refresh wrote into a hidden `catalog-*` overlay (or
created a visible `custom-*` clone) while the catalog row kept showing the
embedded list, and a manual id was only persisted as a selection
(`settings/update`), never into the registry that renders the list.

## What changed

One endpoint identity — `(normalized adapter, base_url)` — now has one
effective model set: the registry overlay ∪ the embedded catalog, projected by
the backend. The UI only persists ids and renders the projection.

- `internal/rpc/control.go`
  - `providerCatalogResult(vendors, entries)` indexes registry entries by
    `endpointKey(bundle, base_url)` and serves `unionModels(overlay.Models,
    endpoint.ModelIDs())` for each catalog endpoint. Overlay ids come first
    (upstream discovery order, then local additions), embedded-only ids follow,
    duplicates collapse. `ThinkingModels` stays derived from embedded
    declarations. The UI and the TUI (`sdk/tui/live/rpc.go`) therefore read the
    same catalog truth.
  - `refreshProviderModels` no longer clones a catalog endpoint into a visible
    `custom-<nonce>` row when no registry entry exists. It resolves the endpoint
    through `provider.NewCatalog(...).VendorForEndpoint(...)` and persists the
    stable `catalog-<vendor>-<adapter>` id — the same id the API-key panel writes
    (`catalogOverlayId` in `ui/src/components/settings/custom-providers.ts`), so
    refresh and key entry share one hidden overlay. Non-catalog addresses keep
    the existing `custom-<nonce>` clone.
- `ui/src/components/settings/ModelSettingsCard.tsx`
  - `confirmAddModel` writes the id into the endpoint's registry entry before
    selecting it: existing same-endpoint entry (including a legacy clone) when
    present, otherwise `catalogOverlayId(vendor, adapter)`. It appends to the
    registry entry's models (or the projected row's models when the overlay is
    new), omits `api_key` (backend patch semantics keep the stored key), keeps
    the input open and skips selection on failure, and only clears the input
    after the selection is saved. Ids already in the list skip the registry
    write entirely, so browsing a listed model does not create an empty overlay.
  - The panel-key/refresh-note reset effect now keys only on the selected row,
    so the note produced by a refresh survives the `loadProviders()` that the
    refresh itself triggers. Since the providers change no longer clears the
    field, every successful `commitPanelKey` write clears the draft explicitly.
- `ui/src/components/settings/custom-providers.ts` — comment records the shared
  overlay-id contract with the backend refresh.

## Scope

Provider-registry projection and Settings UI only. No Eino capability is
involved: no model loop, orchestration, RPC surface, provider adapter, or
storage schema changed. No new RPC method and no wire field was added.

## Explicitly not done

- Pre-existing same-endpoint `custom-*` clones (created by older refreshes)
  remain visible. Hiding or merging them was offered and explicitly not chosen.
- `providerRowKey` (vendor + adapter) is unchanged; the catalog and a legacy
  clone still render as separate rows.
- No migration of existing `settings.yaml` rows; the new union projection reads
  them as-is.
