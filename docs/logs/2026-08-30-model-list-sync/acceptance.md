# acceptance.md — 2026-08-30 model-list-sync

User perspective: how to confirm this change is usable, correct, and has no side effects.

## Verification path (manual)

1. Start Vivy (`just dev` or the normal entry point) and open **Settings → Model**.
2. Select or add an **OpenAI-compatible** provider on the left (a custom entry with
   Base URL and API Key already configured is best).
3. A **refresh icon button** is visible beside the model-list title in the right pane
   (to the left of the new "+" button); hovering shows "Refresh model list".
4. Click Refresh:
   - The button enters a refreshing state (spinning icon, disabled to prevent duplicate
     submissions).
   - When the upstream is reachable and returns `{data:[{id},…]}`, the list shows the
     upstream model IDs and the text "Synced N models from upstream" appears below the
     title.
   - Models manually added with "Add" (not present upstream) remain at the end of the
     list after refresh.
   - After refreshing, **switch to another provider and back**, or **refresh the page**;
     the model list remains (persisted in the `settings.yaml` registry).
5. Key correctness: a configured API Key is not cleared by refresh (the "API Key
   configured" hint remains); the plaintext key never appears anywhere in the UI (the
   backend resolves it through the registry and returns only `api_key_set`).
6. Failure path: when the upstream is unreachable or returns 401, the list is unchanged,
   the card shows a redacted error at the bottom (without the URL or key), and no empty
   shell entry is created.
7. Native Anthropic providers (such as Claude): **do not show** a Refresh button (the
   protocol does not support it), and list behavior remains as before.

## Acceptance criteria

- In step 4, the model list syncs from upstream and persists, without losing manual
  entries.
- In step 5, the key is never returned or cleared.
- Existing "Add model", selected-model chips, and edit/delete custom-provider behavior
  do not regress.
- `just ci` is all green (see the verification.md record).

## Known boundaries (by design)

- Refreshing a static catalog provider (preconfigured entries such as OpenAI/Anthropic)
  clones it into a "Custom" entry before persisting (the catalog entry stays unchanged
  and shows a "Custom" badge), matching the "Manage providers" clone semantics.
- Refresh supports only OpenAI-compatible endpoints; native Anthropic endpoints have no
  Refresh entry point.
