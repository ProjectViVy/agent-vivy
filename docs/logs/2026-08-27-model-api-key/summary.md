# 2026-08-27 · End-to-end model-key support (API Key can be entered in the UI)

## Goal and background

The user reported that they "could not enter the model key" and requested a fix. Vivy's
existing product rule was that keys are injected only by the runtime environment
(config `env_key`, D-010; `api_key:` in config.yaml is a hard error), with the UI holding
no key fields and `settings/update` accepting only `(provider, default_model, base_url)`.
The user explicitly chose **Option A: real end-to-end effect**—the UI can accept a key,
the key enters the backend runtime data and is injected into the runtime bundle's env,
rather than being recorded only in the UI (a fake operation).

## Changes

### Go backend

- `internal/app/settings/settings.go` — added `ApiKey string
  \`yaml:"api_key"\`` to `Settings`; `Validate` rejects keys containing newlines (to
  prevent malformed YAML-injection entries); package documentation now states that keys
  are optionally stored in plaintext at `data/agent-home/settings.yaml` (0600,
  gitignored runtime data, not committed configuration), never written to logs and never
  returned to the control plane; an empty value means no overlay, and environment-variable
  keys continue to work normally.
- `internal/app/app.go` — at startup, `applySettingsOverlay` (symmetrically with the
  existing base_url overlay) injects a non-empty `api_key` into the current runtime
  bundle's `env_key` environment variable through `os.Setenv`; an empty value leaves the
  environment variable unchanged (the env-injection flow is unaffected); logs record only
  the `key_set` boolean and never the value.
- `internal/rpc/control.go` — `settings/get` and `settings/update` add the
  `api_key_set` boolean (the value itself never appears); `settings/update` accepts an
  optional `api_key` (full-document replacement semantics: missing/empty = clear the
  overlay).
- Tests:
  - `settings_test.go`: Save/Load round trip with a key; newline-key validation rejection.
  - `control_test.go`: set a key → `api_key_set=true` and the result JSON contains no key
    value; an update without api_key clears the overlay (`api_key_set=false`).
  - Added `internal/app/settings_overlay_test.go`: overlay injects the key into env_key;
    an empty key leaves the environment variable unchanged; missing settings document is
    a no-op.

### TS frontend

- `ui/src/lib/api.ts` — `Settings.api_key_set?: boolean`; added
  `SettingsUpdate = Pick<Settings,'provider'|'default_model'|'base_url'> &
  { api_key?: string }`; `updateSettings` normalizes `api_key ?? ''` (missing means clear).
- `ui/src/lib/store.ts` — changed the `saveSettings` signature to `api.SettingsUpdate`.
- `ui/src/components/settings/custom-providers.ts` — registry entries gain `apiKey`
  (local `vivy.ui.*` copy); the read side remains compatible with entries saved before the
  field was introduced (missing value filled with `''`); added `customApiKeyFor(bundle, baseUrl)`,
  which returns the key when the target triple matches a custom provider and
  returns `''` for catalog/unknown entries.
- `ui/src/components/settings/ModelSettingsCard.tsx`:
  - the custom-provider dialog adds an 「API Key」 field (password, `autoComplete=off`;
    blank = clear the configured key when applied; edit mode prefills the local copy),
    with a "stored only in local runtime data" notice;
  - the main form's three-input area expands to 2×2: a new 「API Key」 password box echoes
    the selected provider (custom entries show their registered key, catalog entries are
    blank) and is explicitly submitted with 「Save real settings」 (blank = clear the
    overlay)—the key is directly visible and enterable in the main form, not only in the
    dialog;
  - three application paths carry `api_key`: catalog/custom model rows (`applyModelNow`)
    and selected chips (`applySavedNow`) resolve through `customApiKeyFor`; the 「Save real
    settings」 form submits `form.api_key` (empty clears the overlay);
  - when the runtime configuration has a configured key, a 「API Key configured (the value
    is not returned to the UI)」 notice appears below the save button.
- `ui/src/components/chat/MaskAndModelSwitcher.tsx` — the top-bar quick-switch
  `selectModel` likewise carries the key resolved by `customApiKeyFor`.
- Copy: `i18n/{zh,en}.ts` (`settingsModel.apiKey / apiKeyPlaceholder /
  apiKeyHint / apiKeyConfigured`; `customDialogHint`, `settings.modelConfigDescription`,
  `welcome.introBody / secretNote` updated to the new policy); `SettingsView.tsx` model-card
  CardDescription synchronized; the key-rules line in `ui/AGENTS.md` rewritten to the new
  product policy.

## Key semantics contract (consistent with the backend, one authority)

- `settings/update` is full-document replacement: every submission carries the complete
  key state, which may be empty.
- UI resolution: if the target triple matches a custom provider and its `apiKey` is
  non-empty → submit that value; otherwise `''` (clear the overlay and fall back to the
  runtime bundle's env_key environment-variable key).
- Activation timing: like the base_url overlay, it takes effect on the next startup (no
  runtime hot switching).
- Security boundary: the value is stored in `data/agent-home/settings.yaml` (0600,
  gitignored runtime data); `settings/get` returns only `api_key_set`, and neither control-
  plane responses nor logs contain the value.

## Explicitly not done

- Independent key per gateway: with runtime-bundle-level env injection, different base_urls
  within one bundle cannot hold separate keys → recorded in `docs/TODO.md` §0.1
  (`UI-MODEL-KEY-SCOPE`).
- Encrypted key storage/second confirmation: plaintext storage at 0600 remains (the existing
  settings.yaml shape in the same directory).
- Runtime hot switching: it still takes effect at the next startup (consistent with base_url
  semantics).
- Key fields in the demo/local mock preview areas: still placeholders (`vivy.demo.*` keys
  disabled).
- Browser smoke test: 8787 / 3015 were occupied by the user's Vivy Studio debug session,
  so self-testing was skipped and delegated to the user's Studio session (see
  `verification.md`).

## Release notes

No standalone release: shipped with the regular build; `just ci` already includes the UI
build and full Go tests, so no separate `release.md` was written.
