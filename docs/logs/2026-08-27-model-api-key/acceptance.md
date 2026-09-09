# Acceptance guide — 2026-08-27 end-to-end model-key support

Confirm that the feature works from the user's perspective (development environment:
`just dev`, open `http://127.0.0.1:3015`; or run directly in a Vivy Studio session):

1. **The dialog accepts a key**: go to 「Settings → Model」 → 「＋ Add custom provider」.
   The dialog adds an 「API Key」 field (password box) that accepts input; saving creates
   the entry.
1'. **Directly visible in the main form**: Provider / default model / Base URL / API Key
   are arranged as a 2×2 grid; when a custom provider is selected, the API Key box echoes
   its registered key while a catalog entry is blank—the key can be seen/entered without
   opening the dialog, and clicking 「Save real settings」 submits it with the three-input
   combination (blank = clear the overlay).
2. **Entering it takes effect (next startup)**: click a model for the custom provider → it
   immediately becomes the runtime configuration; inspect `data/agent-home/settings.yaml`,
   which contains the literal `api_key: <your value>` and has permission 0600. After restarting
   the backend, requests use this key (no more "API key missing").
3. **Configured notice**: the settings page shows 「API Key configured (the value is not
   returned to the UI)」 below the save button.
4. **Value is not returned or logged**: click 「Settings」 at any time to inspect it
   (`settings/get` returns only `api_key_set=true`; the key's original text does not appear
   in the UI, network panel, or logs).
5. **Switching back to a catalog model automatically clears the overlay**: click a catalog
   model (such as DeepSeek · deepseek-chat) → `api_key` in settings.yaml becomes empty →
   after restart, it falls back to the runtime bundle's env_key environment-variable key.
6. **Edit/clear the key**: edit the custom provider, leave API Key blank, and save → the key
   is cleared the next time that combination is applied (with the same fallback above).
7. **Top-bar/chip switching carries the key**: click the custom entry in 「Selected models」
   (the top-bar dropdown or settings-page chip) → the key is likewise written and takes effect.
8. **Old-data compatibility**: custom providers saved before the field was introduced are
   not lost; the read side automatically supplies an empty key.
9. **Read-only deployment**: with `read_only`, switch-like operations are locked, but
   registry management (including the key field) and bookmark management remain available.

Security note (for awareness during acceptance): the key is stored in plaintext on this
machine at `data/agent-home/settings.yaml` (gitignored runtime data, distinct from committed
configuration and logs); neither the control plane nor logs contain the value.
