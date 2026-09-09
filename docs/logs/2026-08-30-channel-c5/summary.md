# CH-C5 — inspect + Settings page wired to the backend (summary)

Date: 2026-08-30. Branch `feat/channel-c5` (cut from `feat/channel-c4`
5374d6f; sequential slices reused the same worktree).
PLAN: `docs/plans/channel-epic/CH-C5.md` (claimed UI-CHANNELS-BE, no separate
lane). Contract: `VIVY-CHANNEL-PACK.md` §11/§13.

## What changed

Settings → Channels changed from a "seven-platform localStorage fantasy" to the ears
actually compiled into this generation's body. The resident sees the
`Register()` SeamChannel partition, not a wishlist.

1. **Host inspect surface** (`internal/channelhost`): `StartAll` records
   a note for each ear (unconfigured / disabled / empty allow_from refused start /
   startup-failure reason / normal); `Inspect()` emits deterministic
   `ChannelStatus{Name, Capabilities, Configured, Enabled, Started, TokenEnv, TokenEnvSet, Note}`
   output—`TokenEnvSet` reports only a bool (`LookupEnv` non-empty), and the
   value never leaves the kernel.
2. **settings overlay through channels** (decision: `channel/update` writes
   `settings.yaml`, not config.yaml—following the `MCPServers` precedent):
   pointer fields in `Settings.Channels []ChannelOverlay`
   (`Enabled *bool` / `AllowFrom *[]string` / `TokenEnv *string`, unset ≠
   explicitly empty); `applySettingsOverlay` merges by name into
   `cfg.Channels`, **preserving the opaque settings node from config.yaml**
   (tests pin that `parse_mode`, etc. survive enabled/allow_from overrides).
   Ghost names are warned and dropped, never blocking startup. The ear takes effect
   after process restart (the contract §11 "turn off an ear tonight" semantics); no hot
   restart.
3. **RPC**: `channel/inspect` (compiled-in set + capabilities + status + notes),
   `channel/get` (single-ear envelope, no key value), `channel/update`
   (writes overlay; an uncompiled name → `InvalidParams "channel %q is not compiled into this generation"`;
   `*` is rejected by both settings and config validation; Frozen/read-only
   deployments use the existing Conflict gate). Three capabilities were added to
   initialize. Unknown-name copy exactly matches `partitionChannels` startup
   failure.
4. **UI rewrite**: list = full inspect set (only compiled entries are visible; empty
   body → "This generation has no ears" empty state with no Add button); Add wizard
   options = compiled but unconfigured names; email / neuro-link fully removed from
   schema/platforms/icons/i18n; `allow_from` copy is now fail-closed (localized text:
   "one sender per line; empty = refuse startup", i18n-keyed, zh/en structurally
   aligned); token shows only the environment-variable name + "unset" badge + D-010
   explanation (no key-value field);
   `pendingRestart` compares documented state with process state
   (enabled/configured/token_env). localStorage `vivy.ui.channels` is no longer
   read (no migration, no key migration).
5. **Tests**: Go—overlay merge/retention/ghost names, settings round-trip (explicit
   empty allow_from ≠ unset), and all rpc inspect/get/update gates; UI—fully mocked
   store api (server truth, legacy ignored, token_env never invented), five-platform
   schema assertions, and i18n structural alignment (12 dead keys removed, bidirectional
   grep with no dangling references).

## Browser real-path smoke (`http://127.0.0.1:3015`, desktop 1280 + narrow viewport 375)

- **Default body** (`just run`): Channels page empty state "This generation has no
  ears" + guidance copy; no email/neuro-link and no Add entry.
  Screenshot archived.
- **pack telegram candidate** (`pack --with telegram` + scratch config + mock
  provider, with no real Telegram network touched): telegram card appears (enabled /
  needs configuration badges), with `start failed: …TELEGRAM_BOT_TOKEN…` reason
  visible verbatim (token unset → full Secret fail-closed chain: settings decode →
  envelope name pinning → env resolution); editor shows "one sender per line; empty =
  refuse startup", token_env name + unset badge; save `allow_from: []` → `settings.yaml`
  persists `allow_from: []` (empty list = refused-start semantics persisted);
  save the two-line allowlist again → overlay updates. No horizontal overflow in the
  narrow viewport.
- Cleanup after smoke: candidate EXE / Vite / scratch all removed; root tree untouched
  (including clearing the root-tree Vite residue occupying 3015).

## Explicitly not done

- No hot restart for ears (config changes take effect after process restart; inspect +
  pendingRestart badge conveys this).
- `allow_from` changes are not part of the inspect surface → pendingRestart cannot
  detect a pure allow_from edit (a wire field can be added later; recorded).
- Old localStorage keys are not cleaned up or migrated (ignoring is safer than
  mis-migrating keys; recorded).
- On inspect failure, the card view falls back to empty state + a header error bar
  (reviewer note #11, an improvement opportunity).
- The toggle persists documented-state allow_from into the overlay (effective value is
  unchanged; reviewer note #12).
- The session list does not filter `sess_ch_*` (honest visibility; no filter in
  this slice).
