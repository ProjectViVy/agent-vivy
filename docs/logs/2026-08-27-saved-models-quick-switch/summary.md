# 2026-08-27 · 「Selected models」 quick switch (Agent-Diva savedModels port) + model interaction cleanup

## Changes

Ported Agent-Diva's 「Selected models」 quick switch to the Vivy frontend: the quick list is
stored only in local localStorage and consumed by the settings page and top bar from the
same source; model selection follows 「one intent, one action」—clicking selects and saves,
removal changes only the local list, and manual form edits still require explicit submission.

The three decisions were implemented using the recommended options:

1. **Clicking a model in the settings page = select and save immediately** (the diva-style
   one-step flow), rather than merely filling the form.
2. **The top-bar dropdown shows only selected models**; remove the "all catalog models for
   the current provider" section—catalog browsing belongs to the settings page.
3. **Add a 「Selected models」 management area to the settings page** (corresponding to
   diva's "selected model / provider list two-state" behavior).

### Added

- `ui/src/components/settings/saved-models.ts` — quick-list persistence module (following
  the same pattern as `mask-catalog.ts`: module-level cache + `useSyncExternalStore` +
  custom-event / `storage`-event broadcast, not in the zustand store):
  - `SavedModelEntry = { provider, baseUrl, model }`: runtime triple, no key and no
    redundant displayName—display names are resolved at render time through
    `savedModelVendorLabel` / `matchProviderEntry` (catalog match → provider displayName,
    otherwise fall back to the original provider), a single source of truth;
  - localStorage key `vivy.ui.savedModels` (real feature, with `vivy.demo.*` disabled);
  - `getSavedModels` (SSR guard + try/catch + per-field validation filtering bad data),
    `addSavedModel` (deduplicate by triple, append at the end, no limit, consistent with
    diva), `removeSavedModel` (filter by triple), and `useSavedModels()` hook.
- `ui/src/components/settings/saved-models.test.ts` — 16 pure-logic vitest cases:
  triple deduplication and append order, remove, bad JSON / missing-field filtering, label
  resolution (catalog match / custom-gateway fallback), localStorage write-back, custom
  events and cross-tab `storage` broadcast, SSR guard.

### Modified

- `ui/src/components/settings/ModelSettingsCard.tsx` (settings-page 「Model」 tab):
  - added a 「Selected models」 area above the two-column grid: flat chips (`provider ·
    model`); clicking a chip = immediate `saveSettings(triple)` (same semantics as the
    top-bar quick switch), inline X = `removeSavedModel` (local preferences can be removed
    at any time, unaffected by the lock); empty-state notice "Click a model in the provider
    list below to add it";
  - upgraded provider-model row clicks: fill the form + `addSavedModel` (idempotent) +
    immediate `saveSettings`; busy state continues to use `settingsPhase/locked`, errors
    continue to use the existing `settingsError` section;
  - model-row trailing state marker: `Check` = current runtime configuration; non-current
    but selected → muted `Bookmark` icon (title = added to quick list);
  - retained manual edits to the three inputs + 「Save real settings」 button (the explicit
    submission boundary for custom combinations is unchanged).
- `ui/src/components/chat/MaskAndModelSwitcher.tsx` (top bar):
  - changed the `ModelMenu` data source to `useSavedModels()`; removed
    `MODEL_DESCRIPTION_KEYS` and the "current provider's available catalog models" section
    (browsing belongs to the settings page);
  - menu structure: current-configuration block → divider → 「Selected models」 rows
    (filter current triple to prevent duplication; title = model id, subtitle = provider
    name; click = immediate switch through `saveSettings`) → empty-state copy → divider →
    management entry `navigate({ to: '/settings', search: { tab: 'model' } })`;
  - inline removal: X appears on hover (`opacity-0 group-hover:opacity-100`, the existing
    SessionDrawer styling convention), and `onSelect`/`stopPropagation` keep the menu open
    without triggering row switching;
  - **did not port** diva's "removal clears runtime configuration" side effect: removing a
    bookmark does not change live config.
- `ui/src/i18n/{zh,en}.ts` — added entries (structure synchronization enforced by
  `i18n.test.ts`): `settingsModel.savedTitle/savedEmpty/added/removeSaved`,
  `maskSwitcher.savedModels/noSavedModels/removeSavedAria`; removed obsolete
  `maskSwitcher.optionalModels` and `maskSwitcher.models.*` entries that became unused when
  the catalog section was removed.

## Interaction contract (oil-frontend alignment)

- **One intent, one action**: clicking a model row / chip / top-bar row = select and take
  effect; removal = change only the local list; manual edit = explicit save.
- **Busy scope**: switching is locked during a request (using `settingsPhase/locked`);
  removal is unaffected by the lock and does not block.
- **One source for the same data**: the quick list is stored only in the `saved-models`
  module, and the settings page and top bar consume the same `useSavedModels()` source.
- **Empty / read-only state**: both the top bar and settings page have empty-state copy;
  `read_only` deployments disable switching but allow local-list management (the remove
  button is unaffected by read-only).

## Explicitly not done

- Top-bar browsing of "all catalog models for the current provider" (removed by decision ②;
  browsing belongs to the settings page).
- diva's removal-clears-runtime-configuration side effect (bookmarks are decoupled from live
  config).
- List-size limit and sort editing (diva semantics: no limit, append order).
- Browser smoke test: 8787 / 3015 were occupied by the user's Vivy Studio debug session,
  so self-testing was skipped and delegated to the user's Studio session (see
  `verification.md`).
- New Playwright e2e (coverage uses pure-logic vitest + user Studio verification this round).

## Release notes

No standalone release: shipped with the regular UI build; `just ci` already includes the UI
build, so no separate `release.md` was written.
