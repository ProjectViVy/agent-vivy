# Acceptance (2026-08-27, Settings → Network Tools)

How a person can confirm from the product perspective that this change works:

## Backend

1. Under `tools:` in `config.example.yaml`, a `network_search.provider` comment section
   explains that bing/google/searxng require environment variables and duckduckgo/wikipedia
   are keyless, without writing any key values.
2. After starting with `just run`, call the real RPC (or inspect the browser settings page):
   - `settings/get` returns `network_search: { provider, config_provider, providers:
     [bing, google, duckduckgo, searxng, wikipedia] }`; without environment variables,
     `duckduckgo` and `wikipedia` are `configured: true, keyless: true`, while
     bing/google/searxng are `configured: false` and include their `env_key` names.
   - After sending `network_search.provider: "wikipedia"` to `settings/update`,
     `data/agent-home/settings.yaml` contains `network_search: provider: wikipedia`,
     `settings/get` echoes `provider: "wikipedia"`; an unknown provider (such as `yandex`)
     is rejected (InvalidParams).
3. After restart (or by observing `Search` behavior directly): when a request omits a
   provider, `wikipedia` is preferred; if the preferred provider lacks a key, it
   automatically falls back to a keyless provider and the search does not fail.
4. When the settings document already has an api_key overlay, saving Network Tools settings
   **does not** clear api_key (the card merges existing fields + the custom-registry echo).

## Frontend

1. The deep link `http://127.0.0.1:3015/settings?tab=network` lands on the 「Network
   Tools」 section without a 「Preview」 badge; the card title and description are real copy
   that follows the global language (zh/en).
2. The card shows the real provider list: DuckDuckGo and Wikipedia are 「Configured / Keyless」;
   without environment variables, Bing, Google, and SearXNG are 「Needs configuration」 and
   show notices such as 「Environment variable BING_SEARCH_API_KEY required」 (variable name
   only).
3. Select Wikipedia → click Save → 「Saved; effective at next startup」 appears → after
   refresh, Wikipedia remains selected (persistent); restore 「Automatic」 and save to return
   to an empty preference.
4. `Settings → Network` (the old preview tab) no longer exists; Agent-Diva's bocha/brave/zhipu
   fake data is nowhere visible.

## Not verified / boundaries

- No live-call verification of real network search was performed (the user explicitly said
  「end-to-end availability is not required」); the provider request path is covered by the
  existing `network_search_test.go` httptest.
- read_only deployment (empty SettingsPath): the card is read-only and saving is disabled.
- Key values never enter the UI, logs, settings document, or RPC responses (D-010).
