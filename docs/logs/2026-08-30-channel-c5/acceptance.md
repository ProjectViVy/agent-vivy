# CH-C5 — acceptance (how a person can tell it worked)

Date: 2026-08-30.

## Product view: the wishlist became the body's inventory

Open `http://127.0.0.1:3015` → Settings → Channels:

- **Default downloaded body**: an empty-state page—"This generation has no ears: the
  current binary has no channel plugins compiled in". There is no fantasy list of
  seven platforms and no "Add Email / Neuro-Link".
- **A generation with telegram**: cards show real status (enabled / needs configuration /
  pending restart), and startup-failure reasons are visible verbatim instead of
  pretending to be online.

## Human-verifiable points

1. **The gate is green**: `just ci` exits 0.
2. **What you see is what is compiled**: the Channels page list is exactly the set of
   channel plugins compiled into this generation. Want to configure a name absent from
   the body? The UI offers no entry point; calling RPC `channel/update` directly
   also gets rejected ("not compiled into this generation").
3. **The copy no longer misleads**: the allow_from hint is "empty = refuse startup"
   in both Chinese and English, rather than "empty means unrestricted".
4. **No key flows back**: the UI shows only an environment-variable name plus a set/
   unset badge for the token; the RPC response body has no token-value field (confirmed
   by reviewer grep across the full surface).
5. **Configuration changes echo back**: saving writes a `settings.yaml` overlay
   (open the file to see the channels override); the card shows a "pending restart"
   badge; after restarting, the ear follows the new configuration—to turn an
   ear off tonight, set `enabled: false` and restart.
6. **Narrow screens work**: cards and controls do not overflow at 375px wide.

## Real smoke record (2026-08-30)

Default-body empty state → pack telegram candidate + configure telegram (token env
deliberately unset) → card appears and `start failed: …TELEGRAM_BOT_TOKEN…` is
visible (fail-closed demonstration) → clear allow_from and save →
`settings.yaml` contains `allow_from: []` → restore the two-line allowlist and
save → overlay updates. No real Telegram network was touched (mock provider + no token).

## Explicitly not part of this slice's acceptance

- Hot-restarting an ear (configuration taking effect immediately) → a later slice decides
  whether to implement it.
- Real Telegram send/receive → CH-C4 has the capability; real-Bot smoke is pre-release
  human acceptance.
- DingTalk/Feishu/QQ/Discord appearing in the list → they appear only in generations
  where they are compiled in (C6/C7).

## Rollback

Revert this branch: returning the UI to the old read-only state is undesirable (the old
version was a localStorage fantasy); for a temporary fallback, the Settings page may
be read-only, but **do not** revert to the "empty means unrestricted" copy.
