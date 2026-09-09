# CH-C5 — Inspect + Backend-Connected Settings Page (= UI-CHANNELS-BE)

## 1. Identity

| | |
|---|---|
| ID | CH-C5 |
| Stage | E Visibility |
| Person-days | 2 |
| Milestone | M-CH2 |
| Dependency | CH-C4 (requires the real compiled-in name) |
| Successor | Continue to F if C6 is not complete |
| Branch | `feat/channel-c5` |
| Contract | § settings-page rules; `docs/TODO.md` UI-CHANNELS-BE |
| Frontend skill | `.agents/skills/oil-frontend/SKILL.md` |

**Claim this PLAN; do not open a separate UI-CHANNELS-BE lane.**

## 2. Goal

At `http://127.0.0.1:3015`, residents use Settings → Channels and see only the names **compiled in to this generation's body**. The empty allow_from copy is "Empty means startup is denied", not "Leave blank for no restriction". email / neuro-link disappear from the addable list until their corresponding plugins exist. Data is written to the backend envelope, not kept as `vivy.ui.channels` as a ledger.

## 3. Current State

- `ui/src/components/settings/ChannelsSettings.tsx` + `channel-schema.ts` + `channel-store.ts`: seven platforms, localStorage.
- `allow_from` hint: "Leave blank for no restriction".
- The platform list includes email / neuro-link.
- No inspect RPC lists compiled-in versus enabled entries.

## 4. Target Structure

```text
RPC
  channel/inspect     compiled-in[], enabled[], advertised capabilities
  channel/get         envelope (no secret values)
  channel/update      write config.yaml channels: envelope; unknown names fail

UI
  list = intersection of inspect.compiled-in
  no compiled-in → empty state "This generation has no ears", not a seven-platform repository
  allow_from copy is fail-closed
  token shows only the env_key name
```

## 5. File Inventory

**Modify** `internal/rpc`, `internal/config`, the inspect surface in `internal/channelhost`, `ui/src/components/settings/channel-*`, the `ui/src/lib` API, `channel-schema.test.ts` / `channel-store.test.ts`, and i18n.

**Do Not Touch** the SDK usage of the five plugins; do not send secret values back to the frontend.

## 6. Steps

1. inspect: derive the list from the `Register()` SeamChannel names + Host Discover.
2. RPC: allow get/update to accept only compiled-in entries.
3. UI: change the data layer in api.ts; remove the localStorage ledger path (it is acceptable to ignore the old key during migration; record a TODO if needed).
4. Correct the allow_from copy (zh + en).
5. Addable list: remove email/neuro-link; a telegram that is not compiled in must not be "added" either.
6. Update vitest.
7. `just ci`.
8. **Browser:** run `just dev`, open 3015, and walk through both the empty body and a candidate packed with telegram. Test desktop + narrow viewport.
9. Log to `docs/logs/YYYY-MM-DD-channel-c5/`; record browser steps in verification.

## 7. Acceptance

- Default just run body: the channel page is empty or contains only the unrelated hello channel; there is no email/neuro-link add option.
- Packed telegram candidate: telegram appears; allow_from can be written; after saving an empty list, Start fails observably.
- Secret fields do not return values.
- `just ci` is green.

## 8. Prohibitions

- Configuration must not invent names absent from the body.
- Do not continue using localStorage as the source of truth.
- Do not show "Add NeuroLink" when neurolink is not compiled in.
- Do not only take screenshots without clicking through.

## 9. Risks and Rollback

- Old localStorage remnants: ignoring them is safer than incorrectly migrating secrets.
- inspect and UI platform IDs must match the plugin `Name()` (telegram is not Telegram Bot).
- Rollback: the UI may temporarily be read-only; do not revert to the "Leave blank for no restriction" copy.

## 10. Handoff

This phase closes half of the user-visible gate. For domestic leaves, see [CH-C6.md](CH-C6.md) / C7a / C7b.

> **DONE 2026-08-30** — Branch `feat/channel-c5`. `channel/inspect|get|update` RPC; settings overlay `channels` (pointer fields, preserves opaque settings, effective after restart); UI=complete compiled-in set + empty state + fail-closed copy + token shows only the env name. New C6/C7 ears appear **automatically** on this page (inspect-driven), with no UI changes required. Filing: `docs/logs/2026-08-30-channel-c5/`.
