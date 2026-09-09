# 2026-08-27 · Agent-Diva provider catalog and fold-logic port (frontend)

## Changes

Ported Agent-Diva's model-provider catalog and its interaction logic for "a batch of
infrequently used providers folded by default" to the Vivy frontend, in the 「Model」 tab
of the settings page and the top-bar model switcher.

### Added

- `ui/scripts/gen-provider-catalog.py` — deterministic generator for the frontend catalog
  from `ui/agent-diva-source/agent-diva-providers/src/providers.yaml` (rerunnable, with the
  data source included in the repository).
- `ui/src/components/settings/provider-catalog.ts` — generated provider catalog (47
  Agent-Diva providers + the local vivy mock, 48 entries total) and ported logic:
  - `FOLDED_PROVIDER_NAMES`: 20 folded ids exactly matching Diva's
    `hiddenProviderNames` in `ProvidersSettings.vue`;
  - `searchProviders` / `splitByFold`: search by displayName/name substring, case-insensitive;
    search bypasses folding (`more` always empty), while non-search state splits by the fold
    list;
  - `matchProviderEntry`: looks up catalog entries by `(provider, base_url)`.
- `ui/src/components/settings/provider-catalog.test.ts` — pure-logic tests for catalog
  completeness, fold splitting, search, and reverse lookup (13 cases).
- `ui/src/components/settings/ModelSettingsCard.tsx` — new settings-page 「Vivy model
  configuration」 card: left provider list (search box + common providers + 「More providers」
  folded row [MoreHorizontal icon + count + chevron] + automatic expansion when the current
  provider is folded), selected provider's static model list on the right (click fills the
  default model, current item checked), with the original three-input form below (Provider /
  default model / Base URL) and explicit save retained.
- `ui/src/i18n/{zh,en}.ts` — added the `settingsModel` entry block (search placeholder, more
  providers, current badge, no-model notice, custom-combination notice, no match).

### Modified

- `ui/src/components/settings/SettingsView.tsx` — moved the Model-tab form logic into
  `ModelSettingsCard`; deep-link `?tab=` behavior is unchanged.
- `ui/src/components/chat/MaskAndModelSwitcher.tsx` — removed the local
  `MODEL_CATALOG` and switched to the shared catalog: resolves the current provider by
  `(bundle, base_url)` (for example, provider=openai + a DeepSeek gateway → the top bar
  shows "DeepSeek" and lists its models); retained curated-model descriptions
  (balanced/lighter/…) as subtitles, with no subtitle for unlisted models.

## Key adaptation (why this is not a word-for-word port)

The Vivy backend's `settings.Validate` accepts only `provider ∈ {"", openai, anthropic,
mock}`, and product rules require sending the original model id to native endpoints without
automatically adding a gateway prefix. Therefore catalog entries are displayed as
Agent-Diva providers but mapped on selection to the Vivy-valid triple
`(bundle, baseUrl, defaultModel)`:

- `api_type: anthropic` → bundle `anthropic`; all others (diva `api_type: openai`) →
  bundle `openai`; local vivy → `mock`;
- strip the provider's own gateway prefix from `default_model`
  (`openrouter/anthropic/claude-sonnet-4` → `anthropic/claude-sonnet-4`;
  `dashscope/qwen-max` → `qwen-max`);
- do not port the `custom/default` placeholder models from the diva `custom` entry (no
  recommended models);
- fix a diva data bug: `aionly`'s `default_api_base` had the full-width-colon prefix
  `：https://…`, normalized to a valid URL during generation.

## Explicitly not done

- Online model-list refresh (diva's runtime `get_provider_models` fetch): vivy has no
  provider/model catalog RPC, so the catalog is a static snapshot → recorded in
  `docs/TODO.md` §0.1 `UI-PROV-RPC`.
- Per-provider API Key configuration and connection-test wizard: keys are managed only by
  the runtime environment (product rule; the UI does not hold keys).
- Custom-provider creation/deletion (diva's custom-provider CRUD).
- Playwright e2e for folding behavior: browser smoke coverage was used this round; no e2e
  cases were added.
- Existing hard-coded Chinese labels for the Model-tab three-input form
  ("Provider/default model/Base URL") were not migrated to i18n (see the existing §0.1
  `UI-SET-I18N` item; avoid mixing in another person's item).

## Release notes

No standalone release: shipped with the regular UI build; `just ci` already includes the UI
build, so no separate `release.md` was written.
