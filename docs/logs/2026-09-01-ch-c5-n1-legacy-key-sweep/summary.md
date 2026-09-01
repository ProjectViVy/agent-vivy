# CH-C5-N1: delete-only sweep of the legacy `vivy.ui.channels` key

## What changed

- `ui/src/components/settings/channel-store.ts`: every `refreshChannels()`
  now removes the legacy `vivy.ui.channels` localStorage key. The key has
  been ignored since the server became the single source of truth, but old
  browsers kept the stale frontend copy — which may contain token-ish
  legacy fields (`{ ..., telegram: { token: ... } }` in the historical
  shape). Delete-only, no migration; `removeItem` is idempotent and cheap,
  so the sweep rides every pull instead of a once-per-load flag (a flag
  also made tests order-dependent). No `window` / disabled storage →
  silently skipped.

## Tests

- `channel-store.test.ts`: the legacy-key test now plants the historical
  copy, asserts it is **gone** after `refreshChannels()`, and asserts no
  `setItem` ever ran (delete-only, no write-back, no migration).

## Not done

- Any generic localStorage audit beyond this one key (no other keys are
  channel-related).
