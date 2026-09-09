# 2026-08-27 · Complete channel-configuration port (agent-diva → vivy, UI only)

## Goal and background

The 「Channels」 section in settings was previously only a **fake-data preview** from
`DivaSettingsPreview` (three static `DIVA_CHANNELS` entries plus read-only summaries),
so it could not actually configure anything. This iteration ports the complete channel
configuration UI from the Agent-Diva settings page to the Vivy frontend: card view + list
view + inline edit form + add/edit wizard + platform brand icons + tutorial dialog. It
covers all 7 GUI platforms (telegram / discord / feishu / dingtalk / email / qq /
neuro-link) and upgrades 「Channels」 to a real settings section (removing the
「Preview」 badge).

**Scope limited to 「UI only」**: the Go backend is unchanged and no RPCs are added or
modified. The data layer therefore follows the repository's established patterns
(`custom-providers.ts` / `saved-models.ts`): localStorage `vivy.ui.channels` persistence
+ module cache + `useSyncExternalStore`; the storage shape matches Diva's `get_channels`
  wire format, `Record<channelName, {enabled, ...fields}>`, so the read/write layer can be
replaced directly when the backend is connected in the future.

## Changes

### Added (`ui/src/components/settings/`)

- `channel-schema.ts` — port of `channel-wizard-fields.ts`: complete credential-field
  schemas for 7 platforms (required / secret / group / options / defaults / placeholder /
  hint), `fieldDefaults`, `fieldsByGroup`, `splitIdList`/`joinIdList`,
  `coerceChannelFieldValue`, `normalizeChannelConfig`, `getRequiredFields`,
  `validateConfig`, and `isKnownChannel`.
- `channel-platforms.ts` — port of platform metadata (displayName / difficulty /
  requiresPublicIP / accessMethod / quickGuideSteps), `RETIRED_CHANNELS` +
  `isRetiredChannel` (retired: slack / whatsapp / nextcloud_talk / mattermost / matrix /
  irc, continuing the Diva decision from 2026-08-18).
- `channel-store.ts` — `vivy.ui.channels` local storage: `getChannels` /
  `saveChannel` / `toggleChannel` / `removeChannel` / `useChannels`; applies
  `normalizeDiscordConfig` on read (gateway_url / intents=37377 / boolean and list
  defaults, ported from Diva loadChannels); `channelStatusFor` /
  `getChannelStatuses` produce `{name, enabled, ready, missing_fields, notes}`, with
  readiness approximated by the presence of schema-required fields (a replacement for
  the backend's `getConfigStatus`).
- `channel-icons.tsx` — 5 brand icons (Telegram / Discord / Feishu / DingTalk /
  QQ) converted from SVG paths to React components + `PLATFORM_ICONS` /
  `PLATFORM_DISPLAY_NAMES` / `PLATFORM_DESCRIPTIONS` (email → lucide Mail, neuro-link →
  lucide Globe).
- `ChannelCard.tsx` / `ChannelCardView.tsx` — cards (platform icon / readiness badge /
  enabled state / missing-field summary / enable · edit · delete) + card grid and empty-state
  guidance.
- `ChannelEditorForm.tsx` — controlled inline form: text / password (visibility toggle) /
  number / select / textarea / string-list / boolean switch + hint text; basic fields are
  shown inline and advanced fields are placed in `<details>`; channels without a schema
  show 「No editable fields available」 and support JSON editing for unknown extra keys.
- `ChannelWizardModal.tsx` — multi-step wizard (select platform → configure credentials →
  done): edit mode preselects the platform and goes directly to the credentials step;
  completion merges the existing configuration (preserving `enabled`); the credentials
  step includes a quick-guide panel + 「View full configuration tutorial」.
- `ChannelTutorialModal.tsx` — tutorial dialog (platform overview: access method / public
  IP / difficulty stars + react-markdown-rendered built-in guide placeholder, equivalent
  to the fallback for missing Diva tutorial files).
- `ChannelsSettings.tsx` — main view: toolbar (refresh / card · list toggle / add channel),
  card view, list view (left channel list + right status card + inline editing + dirty-check
  save); deletion uses `confirm` + local removal (Diva originally used a
  deleteNotImplemented stub).
- `channel-schema.test.ts` / `channel-store.test.ts` — pure-logic vitest tests
  (platform coverage / defaults / coerce / list normalization / validateConfig /
  localStorage CRUD / Discord normalization / readiness state / SSR guard / bad-data
  filtering).

### Modified

- `SettingsView.tsx` — `channels` entered `SETTINGS_TAB_VALUES` as an explicit tab
  (label `settings.tabs.channels`, without a 「Preview」 badge), mounting
  `<ChannelsSettings />`; tab order: General / Model / Tools / Vivy Features / Language /
  Channels / preview sections.
- `diva-preview-data.ts` — removed the `channels` preview section and the fake
  `DivaChannelPreview` / `DIVA_CHANNELS` data.
- `DivaSettingsPreview.tsx` — removed `ChannelsPreview` and the `channels` case.
- `diva-preview-data.test.ts` — updated assertions (the preview sections no longer contain
  channels).
- `ui/src/i18n/zh.ts` / `en.ts` — added the `channels` dictionary domain (status / settings /
  wizard / card view / tutorial and other keys; the two files are key-for-key identical,
  with i18n parity tests enforcing this automatically).
- `docs/TODO.md` §0.1 — added `UI-CHANNELS-BE`: channel configuration is frontend-only;
  backend channel read/write and readiness reporting are not connected.

## Technical decisions

- **UI only / local persistence**: `vivy.ui.channels` (a real feature key, with
  `vivy.demo.*` disabled); the wire shape is Diva `get_channels`, and connecting the
  backend only requires replacing the read/write layer in `channel-store.ts`.
- **Readiness state = local schema validation**: all required fields present → ready,
  missing fields listed in missing_fields; this replaces the server-side
  `getConfigStatus` channel report, with `notes` always empty (the difference is noted
  in the documentation).
- **The wizard omits the 「Test connection」 step**: the Diva source `steps` array contains
  only platform/credentials/done; the 「Test」 step is currently unreachable dead code
  (`handleWizardTest` always returns unimplemented), so the reachable three-step flow was
  ported and can be extended when the backend has connection-testing capability.
- **Deletion = local implementation**: Diva originally used a `deleteNotImplemented` stub;
  complete channel configuration requires deletion to work, and local-storage deletion
  is honestly feasible.
- **Initially empty state**: no fake channels are seeded; the empty-state card guides the
  user to 「Add channel」.
- **The literal label "Channel" is retained** (the Diva source uses the alternate channel label): this
  matches Vivy's existing tab terminology.

## Explicitly not done (outside this round's scope)

- No backend RPCs (`get_channels` / `update_channel` equivalents) or server-side readiness
  report were added; no real connection test was implemented; external documentation was
  not connected to TutorialModal (built-in guide placeholder only).
- No DOM-rendering component tests were written (the project has not introduced jsdom /
  @testing-library; tests remain pure-logic vitest, consistent with the custom-providers
  and other patterns).
