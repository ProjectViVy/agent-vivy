# 2026-08-28 · Provider direct configuration write + write-time environment variable synchronization + system-level user default workspace

## Goal and background

The user pointed out that “not supporting direct configuration writes and requiring handling through environment variables” is a problematic product direction;
they required (1) **completely changing the overall provider write logic**—adding/editing/deleting providers in the UI and configuring
base_url/model lists/API Key must no longer be localStorage preferences, but instead be actually written to a backend-persisted
document, with the environment variable values **synchronized after writing**; (2) providing a **system-level user workspace**
by default (diva-style: `~/.vivy` contains `workspace/` and other contents).

## Changes

### Go backend

- `internal/app/settings/settings.go` — Settings adds a `Providers []ProviderEntry`
  registry (id/display_name/bundle/base_url/default_model/models/api_key); validates each entry
  (rejects missing fields, invalid bundle, invalid URL, keys containing newlines, and duplicate (bundle,base_url); mock cannot be
  registered); `FindProvider` (matches by bundle+base_url), `ActiveKey` (registry entry key takes
  precedence, with the legacy `api_key` overlay as fallback), `UpsertProvider`, `IsZero`; Load normalizes an empty
  registry to nil.
- `internal/app/app.go` — extracts `applySettingsEnv` (base_url → `VIVY_API_BASE`,
  parsed key → the active bundle's `env_key`); startup overlay and **write-time synchronization share**
  it (via the `ControlDeps.ApplySettingsEnv` callback): settings/provider writes immediately update the current process's
  environment variables, and the same document replays them on the next startup.
- `internal/rpc/control.go` — new RPCs: `settings/providers` (lists the registry, returning only
  `api_key_set`), `settings/providers/upsert` (creates/updates by id, key write-only),
  `settings/providers/delete`; `settings/update` now uses **read-modify-write** (it no longer overwrites the entire document,
  and preserves the registry/network/execute sections), while active api_key is authoritatively resolved by the backend; the
  capability broadcast adds three entries; `settings/get`'s `api_key_set` reflects the resolved key.
- Tests: registry round-trip/validation/conflicts, ActiveKey precedence, UpsertProvider,
  RPC list/upsert/delete/rejection/read-only, update preserving the registry, and write-time env callback counts.

### TS frontend

- `ui/src/lib/api.ts` — RPC_METHODS adds three entries; `ProviderEntry`/`ProviderEntryInput`/
  `ProvidersView` types and `listProviders/upsertProvider/deleteProvider`.
- `ui/src/lib/store.ts` — `providers/providersPhase/providersError` state with
  `loadProviders/saveProvider/removeProvider`; `initialize()` loads the registry in parallel.
- `ui/src/components/settings/custom-providers.ts` — **remove localStorage**
  (`vivy.ui.customProviders` is no longer read or written); it is now a pure logic layer
  (validation/conflicts/merge/collapse/search/`customApiKeySetFor`), with the data source = wire `ProviderEntry[]` in the store.
- `ui/src/components/settings/ModelSettingsCard.tsx` — registry CRUD uses
  `saveProvider/removeProvider` RPCs; dialog/add-model/panel Key values all write to the backend; model selection/
  chip/quick switching `settings/update` **no longer carries api_key** (the backend resolves it from the registry);
  the “API Key configured” prompt uses `customApiKeySetFor`.
- `ui/src/components/chat/MaskAndModelSwitcher.tsx`, `NetworkToolsCard.tsx`,
  `GenerationParamsCard.tsx`, `saved-models.ts` — vendor labels/quick switching/network preferences
  adapted to registry parameters and key removal.
- `ui/src/i18n/{zh,en}.ts` — apiKeyHint changed to “write to runtime data and synchronize environment variables”;
  added `errors.saveFailed`.
- `ui/AGENTS.md` — secret/registry rules rewritten to the new semantics.
- Tests: custom-providers.test.ts rewritten as pure logic; saved-models.test.ts labels wired to
  the providers parameter; store.test.ts mock adds `listProviders`.

### System-level user default workspace (diva-style)

- `internal/config/config.go` — adds `userDataRoot()`: `VIVY_USER_HOME` →
  `os.UserHomeDir()/.vivy` → fallback to `data` (CI/development fallback); `Default()`'s
  sqlite/workspace_root/skills_root/data_dir defaults all live under that root;
  `DataDirectory()`'s postgres/fallback branches are synchronized as well.
- `config.example.yaml` — default path comments now point to the user home directory (explicit values still take precedence).

## API key semantics contract (single source of truth)

- `settings/providers/upsert`'s `api_key` is **write-only**: written to a 0600 runtime document,
  never returned and never logged; `settings/providers` returns only `api_key_set`.
- active key = the key from the registry entry matching (bundle,base_url), otherwise the legacy
  `settings.ApiKey` overlay; if neither exists, the environment is left unchanged (falling back to the runtime bundle's env_key).
- Effective timing: writing immediately synchronizes the process environment variables; already-constructed models still
  “take effect on next startup” (consistent with the existing overlay contract, with no runtime hot swap).

## Explicitly not done

- No runtime engine hot switching (writing synchronizes env; rebuilding models still requires a restart).
- No runtime parsing changes for independent per-gateway keys (`UI-MODEL-KEY-SCOPE`'s provider layer
  remains OPEN for retrieving keys by base_url; the registry and write path were moved to the backend to pave the way).
- No real online provider directory synchronization (`UI-PROV-RPC` remains OPEN).
- `saved-models` bookmarks continue to use localStorage (a UI preference, not provider configuration).
- This iteration was completed in the independent worktree `feat/provider-direct-write` (the root tree is being merged in parallel).

## Release notes

No standalone release: it ships with the daily build, and `just ci` already includes go test + ui build; no separate
`release.md` entry is written.
