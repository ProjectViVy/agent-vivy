# Acceptance — 2026-08-28 provider-direct-write

## User-visible: Settings → Model

1. **Backend-persisted registry**: add a custom provider (display name + bundle + Base URL +
   default model + model list + API Key) → the entry remains after refreshing the page (no longer localStorage);
   `data/agent-home/settings.yaml` (default `~/.vivy/settings.yaml`) shows a
   `providers:` section containing `api_key` (0600).
2. **Secret not returned**: `settings/providers` returns `api_key_set` for each entry; the UI/network panel/
   logs contain no raw secret; the `settings/update` payload no longer carries the key.
3. **Selecting a model synchronizes the environment**: click the provider model → within the process,
   `VIVY_API_BASE` from `os.Getenv` and the active bundle's `env_key` are synchronized to the new values (write-time
   `ApplySettingsEnv`); they remain effective after a restart (the startup overlay replays from the same document).
4. **Edit/delete**: edit the entry in the dialog (id unchanged); delete the entry → it disappears from the list; if it was
   active, the document clears the overlay, and restart falls back to the runtime bundle's env_key.
5. **Conflicts and validation**: duplicate registration of the same (bundle,base_url) is rejected by the backend; invalid URL /
   a key containing a newline produces a save error and is not written to disk.

## System-level user workspace

6. In the default scenario launched with a temporary `HOME`/`USERPROFILE` (or `VIVY_USER_HOME`) and no
   `config.yaml`, first startup → automatically creates `<home>\.vivy\workspace` and
   `<home>\.vivy\settings.yaml`; when `config.yaml` is provided, the path still follows the explicit value.

## How to verify

- Start `just run` + `cd ui; pnpm dev`, open `http://127.0.0.1:3015/settings?tab=model`,
  and perform items 1–6 above one by one.
- Backend unit tests contain all assertions (see verification.md); the user verifies browser smoke in a session with an available port.
