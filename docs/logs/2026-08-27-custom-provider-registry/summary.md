# 2026-08-27 · Custom provider registry (custom providers and models)

## Goal and background

The previous delivery's (「Selected models」 quick switch) quick list could only receive
entries from static-catalog model rows; manually entered custom combinations (private
gateways / local LLM endpoints outside the 48-provider static catalog) could not enter
the quick-switch flow, and custom-entry provider labels fell back to the original bundle
name (such as "openai"), making them hard to identify.

This iteration implements two decisions confirmed by the user:

1. **Scope = custom provider registry** (a Vivy adaptation of Agent-Diva custom-provider
   CRUD): the settings page can add/edit/delete custom providers (display name + runtime
   bundle + Base URL + default model + model list), include them in the left provider
   panel, and add a model to 「Selected models」 and select it immediately when clicked.
2. **Provider-label source = display name first**: the registry stores displayName (the
   single source of truth); an unregistered manually entered combination falls back to the
   baseUrl hostname (such as `my-gateway.example.com` or `localhost:11435`); without a
   baseUrl it falls back to the original provider bundle name.

## Changes

### Added

- `ui/src/components/settings/custom-providers.ts` — registry-persistence module
  (following the saved-models.ts / mask-catalog.ts patterns: module-level cache +
  `useSyncExternalStore` + custom-event / `storage`-event broadcast, not in the zustand
  store; SSR guard + per-entry validation):
  - localStorage key `vivy.ui.customProviders` (real feature, with `vivy.demo.*` disabled);
  - `CustomProvider = { id, displayName, bundle(openai|anthropic), baseUrl,
    defaultModel, models }`; ids are generated automatically (`custom-<uuid>` with a
    non-crypto fallback), and renaming/editing does not change the id; the quick list links
    by baseUrl and does not reference the id;
  - `getCustomProviders / addCustomProvider / updateCustomProvider /
    removeCustomProvider / useCustomProviders`; add/update checks `(bundle, baseUrl)` for
    conflicts with the catalog and existing custom entries, returning null on conflict
    (the form reports the error in place);
  - `parseCustomModels`: newline / comma / Chinese-comma-separated input, trimmed and
    deduplicated;
  - merged view (thin adapter layer; the static catalog in `provider-catalog.ts` remains
    clean): `MergedProviderEntry = ProviderCatalogEntry & { custom, registryId? }`,
    `allProviderEntries` (catalog first + custom entries after it), `searchMergedProviders`,
    `matchMergedProviderEntry` (catalog first, custom second, empty base_url keeps the
    bundle-name fallback), and `splitMergedByFold` (custom entries never go into more).
- `ui/src/components/settings/custom-providers.test.ts` — 16 pure-logic vitest cases:
  CRUD / conflicts / bad-data filtering / write-back / `parseCustomModels` / unique ids /
  merged view (ordering, custom flag, search, match priority, fold exclusion, and flat
  search state).

### Modified

- `ui/src/components/settings/saved-models.ts` — `savedModelVendorLabel` now uses:
  catalog/registry match → displayName; otherwise, with a Base URL → hostname
  (`new URL().host`, try/catch, falling back for an invalid URL or empty host); otherwise
  the original provider. Bookmark labels resolve the registry through baseUrl, so a rename
  takes effect globally without derived copies.
- `ui/src/components/settings/saved-models.test.ts` — updated and added label assertions:
  `vllm`'s `localhost:11434` in the catalog still matches the catalog (catalog first);
  an unregistered custom gateway falls back to its hostname (including port); an invalid
  URL / empty baseUrl falls back to the original provider; a registry displayName matches.
- `ui/src/components/settings/ModelSettingsCard.tsx` (the settings 「Model」 tab):
  - left-column data source changed to the merged view: non-search state = visible catalog
    → custom rows → 「More providers」 fold; search state = flat merged search (custom
    matches mixed in with a marker);
  - custom rows: muted 「Custom」 pill at the end; hover shows Edit (Pencil) / Delete (X)
    (`opacity-0 group-hover:opacity-100`, following the SessionDrawer convention);
    `ProviderRow` gained optional `actions` rendering—when actions exist, the row root
    becomes a `div.group > button(Select) + action` structure, avoiding invalid button
    nested inside button HTML;
  - permanent dashed row 「＋ Add custom provider」 at the bottom of the left column → new
    Dialog;
  - shared add/edit Dialog (ui/dialog): display name / runtime bundle (Select) / Base URL /
    default model / model list (textarea); field-level validation + in-place conflict
    errors; edit mode prefilled, with distinct 「Add/Edit」 titles; deletion only removes
    the registry entry, without affecting bookmarks or runtime configuration;
  - clicking a custom row = fill the form; clicking its model = fill the form +
    `addSavedModel` + immediately `saveSettings`, exactly the same path as catalog models;
  - registry management (add/edit/delete) and bookmark removal are unaffected by `locked`
    (local preferences); model-row / chip / switch-like operations continue to use
    `settingsPhase/locked`.
- `ui/src/components/chat/MaskAndModelSwitcher.tsx` — `displayProvider` reuses
  `savedModelVendorLabel` (removing the direct dependency on `matchProviderEntry`); the
  trigger and 「Current configuration」 block show an identifiable provider name for custom
  gateways.
- `ui/src/i18n/{zh,en}.ts` — added `settingsModel` entries:
  `customBadge / addCustomProvider / customDialogTitleNew / customDialogTitleEdit /
  customDialogHint / displayName / bundle / bundleOpenai / bundleAnthropic /
  baseUrl / defaultModel / modelsList / modelsListHint / save / cancel / editAria /
  removeAria / errors.{displayNameRequired,baseUrlRequired,baseUrlInvalid,duplicateBaseUrl}`
  (zh/en structure enforced by `i18n.test.ts`).

## Interaction contract (following the existing system)

- One intent, one action: registry CRUD only changes local `vivy.ui.customProviders` and
  never changes runtime configuration; runtime configuration changes only through model
  clicks / chips / the top-bar row / 「Save real settings」.
- Single source of truth: display name is stored only on registry entries; bookmarks and
  the top bar resolve it through baseUrl, with no derived copies.
- Busy / read-only: switch-like operations continue to use `settingsPhase/locked`; registry
  management and removal are unaffected by the lock.
- No keys: neither the registry nor the quick list stores keys (Vivy rule: secrets do not
  belong in the UI).

## Explicitly not done

- The A/C-plan 「Add to quick list」 button was not added: manually entered combinations
  enter the quick-switch flow by recording model ids in the registry's model list (the
  user confirmed scope B).
- Custom mock bundles are not provided (mock is a built-in offline bundle); custom bundles
  are limited to openai/anthropic.
- The generated catalog is not edited by hand (`provider-catalog.ts` data is generated by
  a script; the catalog's `custom`/`vllm` placeholder entries remain unchanged).
- Real backend provider-metadata RPC remains under `docs/TODO.md` §0.1 `UI-PROV-RPC` (this
  delivery is a UI-local structure and does not close that item).
- Browser smoke test: 8787 / 3015 were occupied by the user's Vivy Studio debug session,
  so self-testing was skipped and delegated to the user's Studio session (see
  `verification.md`).

## Release notes

No standalone release: shipped with the regular UI build; `just ci` already includes the
UI build, so no separate `release.md` was written.
