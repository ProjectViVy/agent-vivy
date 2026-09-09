# 2026-08-30 model-list-sync — Settings → Model "Refresh model list"

## Summary

### What changed

The Settings → Model model list now has a "Refresh" feature: fetch model IDs from the
upstream OpenAI-compatible endpoint `GET {base_url}/models` and save them to the local
provider registry (`providers[].models` in `settings.yaml`).

Layered changes (worktree `agent-vivy-model-sync`, branch `feat/model-list-sync`):

**Backend**
- New `internal/provider/discover.go`: `ModelListClient.List` — 15s timeout ceiling,
  Bearer-key request, 4MiB response ceiling, trimming/deduplication/order preservation;
  errors never carry the key or URL (`*url.Error` strips the URL while retaining the
  cause chain, D-010).
- Add the `settings/providers/refresh` RPC in `internal/rpc/control.go`:
  - Refresh by registry `id`, or locate by `(bundle, base_url)`; when an OpenAI-compatible
    catalog provider has no registry row, **clone it into a new custom entry** for
    persistence (the frontend supplies `display_name`).
  - Resolve keys through `settings.ActiveKey` (registry-entry key or legacy overlay); on
    write-back, **change only `models` and preserve `api_key` verbatim** (avoiding the
    existing "whole-entry replacement clears the key" trap).
  - **Union strategy**: upstream IDs first (gateway order), then locally added IDs absent
    upstream; refresh never loses manual entries.
  - Do not persist failures: return a redacted error for HTTP/parse failures and leave
    the registry unchanged.
  - Reject native Anthropic entries/requests directly (its API has no `/models` protocol);
    reject read-only and Frozen (ENV-locked) cases consistently with other settings
    writes.
  - Register `settings.providers.refresh` in capabilities; inject
    `ControlDeps.ModelLists` (httptest in tests, default 15s client in production).
- Add API smoke and error-matrix tests for `settings/providers/refresh`
  (`control_test.go`), reusing the existing settings test environment
  (`newSettingsHandlerEnvWith` hook injection; old signature unchanged).

**Frontend**
- `ui/src/lib/api.ts`: register `RPC_METHODS` + `ProviderRefreshInput` +
  `refreshProviderModels`.
- `ui/src/lib/store.ts`: `refreshProvider` action (on success, `loadProviders` reads
  back the persisted result; on failure, write `providersError`).
- `ui/src/components/settings/ModelSettingsCard.tsx`: add a "Refresh" icon button next
  to "Add" in the model-list header (shown only for OpenAI-compatible entries; spinner +
  disabled while refreshing to prevent duplicate submissions); on success show "Synced N
  models from upstream"; on failure render `providersError`.
- i18n: `settingsModel.refreshModels` / `refreshing` / `refreshed` (zh + en).
- Add e2e `ui/e2e/model-refresh.spec.ts`: real browser + self-started backend + local
  `/models` service, covering empty list → refresh lists upstream models (checks Bearer
  key sent upstream) → union retains manual addition → reload persists the list without
  echoing the key.

**Existing defect found and fixed during e2e**: `toProviderEntryResult` returned
`append([]string(nil), ...)` for an empty model list → JSON `models: null`; frontend
registry validation requires `Array.isArray(models)`, so **registry entries with no models
were silently discarded and became invisible after creation** (triggered when a new custom
provider was created without a model list). Fix the wire format to always use `models: []`
(`internal/rpc/control.go` `toProviderEntryResult`) and add a regression assertion (empty
model upsert/list both serialize as `[]`, not `null`).

### Explicitly not done (outside scope)

- Do not turn the static catalog snapshot `provider-catalog.ts` into a runtime catalog
  (still TODO work requiring a separate iteration); refresh affects only refreshed/cloned
  registry entries and their display.
- Fetching model lists from native Anthropic endpoints (protocol unsupported); their
  lists remain static/manual.
- Automatically selecting a new model or rewriting `default_model` (refresh does not
  change the current selection).
- Migrating existing `vivy.ui.customProviders` localStorage (UI-PROV-REGISTRY, a
  separate item).
- Studio overlay, user plugins, and the tenant Journal (`data/vivy.db`, `data/demo/`,
  `data/workspaces/`) were not touched.

### Key decisions

- **Union rather than replacement**: retain models the user manually added, so refresh
  cannot silently lose local entries.
- **Clone catalog entries**: the catalog is a frontend static snapshot with no
  corresponding backend row; "save locally" requires a writable target, and cloning to
  a custom entry is the smallest path consistent with the existing "Manage providers"
  flow.
- **Write the key, never clear it**: unlike the old whole-entry replacement in upsert,
  refresh writes back only `models`, so it cannot clear a configured key; errors and
  responses never carry the key.
