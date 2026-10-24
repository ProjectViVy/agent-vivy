# Acceptance

## Built-in vendor row updates in place

1. Start the split pair (`just run` + `cd ui && pnpm dev`) and open
   `http://127.0.0.1:3015/settings?tab=model`.
2. Select **DeepSeek** (`openai-completions`) and click **Add model**; enter
   `deepseek-smoke-x` and confirm.
3. Expected: the model appears in the list immediately and carries the current
   check mark; the top bar model switch also shows it. Reloading the page keeps
   it listed and current.
4. Expected on disk: `<VIVY_USER_HOME>/settings.yaml` contains a
   `catalog-deepseek-openai-completions` entry whose `models` include the new
   id — not a new visible provider row.

## Refresh updates the selected row and says so

1. With a custom provider whose base URL exposes `GET /models`, select its row
   and click the refresh icon in the model header.
2. Expected: the list replaces/extends with the upstream ids, the success note
   ("Synced N models from upstream") stays visible after the registry reload,
   and no second provider row appears for the same endpoint.
3. Adding a model id by hand and refreshing again keeps the manual id (union,
   upstream ids first).

## Manual ids persist and can be selected

1. Add a model id on any row (built-in or custom), reload the page, reselect the
   row.
2. Expected: the id is listed and can be selected (it is the current model after
   the add).
3. Adding an id that is already listed only switches the current model; it does
   not rewrite the provider document.

## Credentials

- The API key never appears in the page, the model list, or the refresh note.
  It is only written to the local settings document, and the key field remains
  write-only (empty after a successful save).

## Known limits

- A `custom-*` row created by an older refresh for the same endpoint as a
  catalog vendor stays visible; refresh and manual adds now target the catalog
  row (or the existing same-endpoint entry) instead of creating another one.
