# 2026-08-30 — Catalog-provider API Key entry (Settings → Model)

## Background

Users reported that when a catalog provider (such as DeepSeek) was selected under
"Settings → Model", the API Key field was **not editable**, even though the hint said
"Catalog providers can also have an API Key entered here; it is written to the local
user workspace."—the copy and behavior contradicted each other.

Root cause: the `catalogKeyHint` copy was changed to "Catalog providers can also be
entered here" in `faeb76e` (sandbox wording
normalization), but the `ModelSettingsCard.tsx` field's
`disabled={locked || !selectedEntry.custom}` and `commitPanelKey` early return on
`!selectedRegistry` were never opened up accordingly—the copy and implementation
drifted apart. The user chose the "fully open entry" direction.

## Changes

- `ui/src/components/settings/custom-providers.ts`
  - Added pure functions for catalog-provider key **overlay entries** (catalog overlay):
    `CATALOG_OVERLAY_PREFIX` / `catalogOverlayId(name)` /
    `isCatalogOverlayEntry(entry)` / `providerEntryByEndpoint(...)` (looks up registry
    entries by `(bundle, base_url)`, matching the backend `ActiveKey` resolution).
  - `allProviderEntries`: skips `catalog-*` overlay entries so a duplicate "Custom"
    row does not appear beside a catalog row; ordinary custom entries
    (including clones) remain visible.
  - `customApiKeySetFor`: changed from the merged-view custom flag to a registry
    entry matched by endpoint with `api_key_set`—catalog endpoints with a key
    override now also show "API Key configured".
- `ui/src/components/settings/ModelSettingsCard.tsx`
  - The API Key field enablement condition changed to `locked || bundle === 'mock'`:
    catalog providers (the openai/anthropic bundles) are editable, while the built-in
    offline Mock bundle remains disabled.
  - `commitPanelKey` now supports catalog providers: on blur it persists by endpoint.
    An existing registry entry (including an existing custom clone) is updated; otherwise
    a `catalog-<name>` overlay entry is created (an empty value with no endpoint entry
    **does not** create one from nothing). The custom-entry path keeps its prior semantics.
  - Added the `panelKeyDirty` dirty flag: an unmodified field does not submit on blur,
    preventing a click-in/click-out from accidentally clearing a configured key or
    creating an entry (also for custom providers).
  - Placeholders and hints: editable cases consistently use `apiKeyPlaceholder` /
    `apiKeyHint`; Mock uses `catalogKeyHint`.
  - Updated the top-level behavior comment.
- `ui/src/i18n/{zh,en}.ts` — `catalogKeyHint` is now Mock-specific
  ("The built-in Mock provider does not need an API Key.");
  editable catalog-provider cases use `apiKeyHint` (writes to ~/.vivy/settings.yaml,
  does not echo the value, and takes effect on the next message).

## Security boundary (D-010)

No new backend write surface: catalog and custom provider keys use the same
`settings/providers/upsert` path, persist write-only with 0600 permissions to
`~/.vivy/settings.yaml`, and are neither returned nor logged. At runtime, `ActiveKey`
resolves the authoritative value by `(bundle, base_url)`.

## Explicitly not done

- No backend changes: `FindProvider` / `ActiveKey` / `upsertProvider` already
  resolve by `(bundle, base_url)`, so catalog endpoints can directly hit registry entries.
- No real online provider-catalog synchronization (still `docs/TODO.md` §0.1
  `UI-PROV-RPC`).
- No key for Mock (it is an offline built-in bundle, and settings rejects `mock`).
- No UI for "delete overlay entry": blurring an empty value clears the
  key, and leaving the entry empty has no side effect (an empty `api_key` creates no
  override).
