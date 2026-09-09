# Settings → Network Tools: adapt EINO native network support + real settings section (2026-08-27)

## Changes

Upgrade the `Settings → Network Tools` section from an 「Agent-Diva migration preview」
(fake data: bocha/brave/zhipu, which did not exist in the backend) to a **real settings
section**, exposing the [EINO native network tools](/docs/AGENT-VIVY-ARCHITECTURE-V0.md)
already present in the Vivy runtime (`network_search`: bing/google/duckduckgo/searxng/
wikipedia; `http_request`: read-only web fetching, with only the status surface implemented
this round). Backend first, frontend second; per the user's request, **lay the foundation
only, without end-to-end availability** (no live search verification, no browser-use
integration, and no key entry in the UI).

### Backend (phase one, first)

- `internal/config/config.go`: added a `NetworkSearch` configuration section to `Tools`
  (`tools.network_search.provider`, allowlist bing/google/duckduckgo/searxng/wikipedia,
  empty = automatic); passed through the `toolsDoc` mirror + `UnmarshalYAML`; `Validate()`
  rejects unknown providers; `Default()` uses an empty provider. Added documentation
  comments to `config.example.yaml` (environment-variable names only, no key values, D-010).
- `internal/app/settings/settings.go`: added `NetworkSearch.provider` to the settings
  document `Settings` (not a key, empty = inherit config); `Validate()` applies the allowlist;
  Save/Load round trip covered it. **Retained** the api_key overlay field and semantics
  (the tool-polish prototype removed it, but this iteration does not follow that change).
- `internal/runtime/network_search.go`: `NetworkSearchService.SetPreferredProvider`—when a
  request omits a provider, prefer the configured one; if the preferred provider is
  unavailable (missing key), automatically fall back to keyless duckduckgo/wikipedia
  without failing. Added `NetworkSearchProviderAvailability()`: returns providers in
  preference order plus environment-variable **presence** (`os.Getenv != ""`), never reading
  or returning key values, D-010.
- `internal/app/app.go`: `applySettingsOverlay` overlays the settings document's
  `network_search.provider` onto `cfg.Tools.NetworkSearch.Provider` (takes effect at the
  next startup, with no hot switching); calls `SetPreferredProvider(...)` after building
  `searchOps`; added `ConfigNetworkSearchProvider` to `ControlDeps` so RPC can echo the
  config default.
- `internal/rpc/control.go`: `settings/get` adds a `network_search` section
  `{provider, config_provider, providers[]}`; `settings/update` accepts and persists
  `network_search.provider` (retaining api_key semantics). Key values are never returned.

### Frontend (phase two, implemented after rebasing onto the main where this lane landed)

- `ui/src/lib/api.ts`: added the `network_search` view type + `NetworkSearchProviderInfo`
  to `Settings`; `SettingsUpdate` accepts `network_search.provider`.
- New `ui/src/components/settings/NetworkToolsCard.tsx`: real network-tools card using
  `useVivyStore` (`settings/get|update`, with demo-api disabled)—provider list +
  keyless/configured badges + environment-variable-name hints; preferred-provider selector
  (automatic + 5); saving merges the existing provider/base_url/api_key
  (`customApiKeyFor` echoes the overlay without clearing the model card's key);
  saving is disabled when `read_only`.
- `SettingsView.tsx`: promoted `Network Tools` to a first-class section (no 「Preview」
  badge, deep link `?tab=network` available, `SettingsTab` includes `'network'`);
  `diva-preview-data.*` removes `network` and updates tests; `DivaSettingsPreview.tsx`
  deletes the superseded `NetworkPreview` fake preview (including the
  `Globe2`/`NetworkProvider`/`network` branch).
- i18n: added top-level `networkTools` (identical zh/en structure, including provider names,
  environment-variable hints, and key-presence explanation); removed orphaned
  `diva.network.*` fake-preview entries.
- e2e: `ui/e2e/network-tools-setting.spec.ts`—deep link `?tab=network` → real roster
  (2 keyless 「Configured」 items, 3 key-required 「Needs configuration」 items) → select
  Wikipedia → save → refresh retains it → restore automatic.

## Scope notes (user request: lay the foundation, no end-to-end)

- **Not done**: live-call verification of real web search/fetching; browser automation
  (browser-use explicitly excluded); UI entry/storage of API Key (only environment-
  variable presence is shown, and values never enter the UI/logs/settings document, D-010).
- `http_request` (web fetching) appears only in the tool list outside the backend availability
  roster; no independent UI configuration surface was built for it this round—enable/disable
  and the domain allowlist are controlled by `config.yaml` `runtime.http_allowed_hosts`,
  recorded in `docs/TODO.md` §0.1 for follow-up.
- Network-tools configuration takes effect like the model provider: saved to
  `data/agent-home/settings.yaml`, **effective at next startup**, with no hot switching.
- Parallel-lane governance: the root tree was occupied by the Settings-Language/Compaction
  lane at the time; this feature was developed under `parallel-worktree-isolation` in the
  `../agent-vivy-network-tools` worktree + `feat/network-tools` branch and landed through
  merge/PR; frontend files were implemented after rebasing onto the main where that lane
  landed (`0791a3d`).

## Changed-file list

Backend: `internal/config/config.go`, `internal/app/settings/settings.go`,
`internal/runtime/network_search.go`, `internal/app/app.go`, `internal/rpc/control.go`,
`config.example.yaml`, and the corresponding `_test.go` files.
Frontend: `ui/src/lib/api.ts`, new `ui/src/components/settings/NetworkToolsCard.tsx`,
`SettingsView.tsx`, `DivaSettingsPreview.tsx`, `diva-preview-data.ts(.test.ts)`,
`ui/src/i18n/{zh,en}.ts`, and new `ui/e2e/network-tools-setting.spec.ts`.
