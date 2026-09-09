# Multi-branch merge into main integration record

## Background

The user requested that the remaining development branches be merged into main. The root
worktree had previously been on `feat/settings-genparams-provider`
(3c2181e) and `feat/trajectory-panel` (87980e4); this closeout covered four branches:
`feat/network-tools` (including reconciliation with `feat/tool-polish`), `feat/execute-timeout`,
and `feat/channels-ui`.

The top of main after the merges (all local merges, not pushed to origin):

```
8944a53 merge: feat/channels-ui
186b321 merge: feat/execute-timeout
49a2ad3 merge: feat/network-tools (including tool-polish reconciliation 90f60a3)
3c2181e ui(settings): move generation-parameter demo into General-Advanced features
87980e4 ui(dashboard): control-center trajectory panel
```

## Conflicts and decisions

### Deduplication of same-topic implementations (`network_search`, most important)

Both `tool-polish` (base 6a391b0) and `network-tools` (base 0791a3d) implemented
the preferred provider, validation, and settings overlay for
`tools.network_search.provider`: the same name and responsibility, with similar
implementations. `merge feat/tool-polish` was first reconciled inside the
`feat/network-tools` worktree (12 conflicting files resolved one by one), and the result
was then merged into main as a whole:

- Retained tool-polish's unique deliverables: 1-based line numbers for read_file
  (patch anchors), moving echo_info out of the default enabled set (removing it from
  `Tools.Enabled` in `config.Default()`), filesystem polish, and the no-key fallback note.
- Retained network-tools' unique deliverables: a real 「Network Tools」 tab in settings
  (NetworkToolsCard), the network_search section and availability roster for
  settings/get|update (key presence only), the `api_key_set` overlay, and
  `ui/e2e/network-tools-setting.spec.ts`.
- Deduplicated by removing tool-polish's inline 「Network Search」 card from the 「Tools」
  tab (the canonical UI is NetworkToolsCard in the network tab); one copy each of the
  provider configuration, validation, and roster was retained.
- Fixed semantics: `MaskAndModelSwitcher` now passes through the `network_search`
  preference when switching models (settings/update replaces the entire document, so
  failing to pass it through would clear the network search settings).
- Test union: both preferred-provider tests, `PrefersConfiguredProvider` and
  `HonorsPreferredProvider`, were retained; availability tests use the union of both
  sides' coverage; the `staticReadOps` in `filesystem_test.go` was supplemented with a
  `ListDir` stub.

### Merging execute-timeout with existing capabilities

- The `ControlDeps`/`settingsResult`/`settings/update` payloads now carry all three
  field groups: api_key, network_search, and execute_max_timeout_seconds (each branch
  previously carried one group).
- `applySettingsOverlay` applies api_key → network_search → execute in sequence.
- The real 「Execution timeout ceiling」 form remains under Settings → General (including
  the `settings_overlay_test.go` add/add merge: all five tests for api_key/empty key/no
  document/network_search/execute coexist).
- `WelcomeWizard`/`MaskAndModelSwitcher` pass through network_search and execute when
  saving, preventing them from clearing each other.
- The Model tab retains main's ModelSettingsCard system (the form-based model page from
  the execute-timeout branch was replaced by main's provider-catalog system).

### Upgrading channels-ui to a real section

- Settings gained a real 「Channels」 tab (9 new components including ChannelsSettings,
  the `vivy.ui.channels` channel-store, and schema validation); the migration preview
  `ChannelsPreview` and `DIVA_CHANNELS` were removed.
- `DIVA_PREVIEW_SECTIONS` was narrowed to `['general','self-evolution','sandbox']`
  (language/compaction/network/channels are now real or have been merged elsewhere).
- The 「Language」 tab label now uses the i18n entry (`settings.tabs.language`), with the
  corresponding correction to `ui/e2e/language-setting.spec.ts` (asserting the 'Language'
  label after refresh with the English UI).

## Not done (explicit boundaries)

- origin was not pushed (explicit user authorization is required).
- Existing `UI-E2E-STALE` was not fixed (stale assertions at runtime.spec.ts:84 and
  welcome-wizard.spec.ts:33, recorded in `docs/TODO.md` §0.1).
- Backend channel read/write, the `http_request` configuration surface, and similar
  items remain §0.1 OPEN items (UI-CHANNELS-BE, UI-NETWORK-HTTP, etc.); this round only
  landed the frontend form.
- Branch reconciliation occurred only inside the network-tools branch (90f60a3); the
  remaining branches were merged as-is.
- The generation-parameter demo retained the final direction of 「General → Advanced
  Features, edited from a per-model dropdown」 and was not reverted.
