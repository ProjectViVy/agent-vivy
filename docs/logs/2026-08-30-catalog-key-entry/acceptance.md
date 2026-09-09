# Acceptance guide — 2026-08-30 catalog-provider API Key entry

Development environment: `just dev` → `http://127.0.0.1:3015` → Settings → Model
(a hard refresh of the Studio session also works).

1. **The catalog-provider field is editable**: click any catalog provider on the left
   (for example, DeepSeek); the API Key password field on the right is **editable**.
   Its placeholder is "sk-… (empty = clear the configured key when applied)", and the
   hint below is "Written to the local user workspace (~/.vivy/settings.yaml); the
   value is not returned to the UI or written to logs. It takes effect on the next
   message."
2. **Blur persists without echoing**: enter `sk-...` and blur the field (click
   elsewhere); the hint changes to "API Key configured (the value is not returned to
   the UI)". After refreshing, the field is still empty (write-only; the value is
   never returned), but the "configured" hint remains; the custom-provider list does
   **not** show a duplicate "DeepSeek" row.
3. **Clear the key**: clear the field and blur it; the endpoint key override is removed
   and the "configured" hint disappears.
4. **Guard against accidental changes**: click into the field and back out without
   entering any characters; the key is neither cleared nor used to create an entry.
5. **Changing models takes effect immediately**: click any model under that catalog
   provider; the next message uses that provider with the key just entered (the backend
   resolves it by endpoint).
6. **Custom-provider behavior is unchanged**: when a custom provider is selected, its
   key can still be edited or cleared and the "configured" hint behaves as before.
7. **Mock remains disabled**: when the built-in Mock is selected, the field is disabled
   and shows "The built-in Mock provider does not need an API Key.".
