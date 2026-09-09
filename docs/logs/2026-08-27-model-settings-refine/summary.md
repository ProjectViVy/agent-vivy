# 2026-08-27 · Model-settings interaction refactor (three review suggestions implemented)

## Background

After reviewing the 「Model」 settings page in Vivy Studio, the user made three suggestions;
this iteration implements them:

1. Add a **persistent small edit button** on the right side of custom-provider rows so the
   address (Base URL) and alias (display name) can be edited.
2. Add a **「Sync from official」 refresh button** and an **「Add」 button** (manually add a
   model) to the header of the concrete model list on the right.
3. **Remove the three-input form below** (Provider / default model / Base URL / API Key +
   Save real settings), and **move API Key entry above the model list**.

## Changes

- `ui/src/components/settings/ModelSettingsCard.tsx` — refactored:
  - removed the bottom 2×2 input form and 「Save real settings」 button: selection/application
    now entirely follows "select provider → click model/add model → select and save
    immediately" (the form was an old explicit-submission boundary whose semantics
    duplicated quick switching, so it was removed per the user's suggestion); also removed
    form-related state (`form` / `applyProviderEntry` / `selectProvider`), with selection
    changed to the `selectedName` cursor (default follows the current runtime provider);
  - provider rows: the custom-row **Edit button (Pencil) is persistently visible** (clicking
    opens the add/edit dialog, where display name = alias, Base URL = address, runtime
    bundle, default model, model list, and API Key can be edited); Delete (X) remains shown
    on hover;
  - added two icon buttons to the right model-list header (beside the original bundle label):
    - `RefreshCw` 「Sync from official」—reloads the merged view and shows the feedback
      「Model list reloaded (static catalog snapshot)」. Honest boundary: online sync remains
      in `docs/TODO.md` §0.1 `UI-PROV-RPC` (the interface does not exist; the button is
      currently "reload + feedback" and does not pretend to fetch new data);
    - `Plus` 「Add」—expands an inline input at the top of the list (Enter/✓ add, Esc/✕
      cancel): for custom providers, the model id is also persisted into the registry's
      `models`, then follows the same semantics as clicking a model (select immediately +
      add to quick list + store the key); catalog providers do not write to the registry,
      but the bookmark persists the combination;
  - moved API Key from the bottom form to above the model list: it is editable when a custom
    provider is selected (written back to the registry on blur and applied with model
    clicks); it is disabled for catalog providers with the notice
    「Catalog-provider keys are injected by the runtime environment」;
  - moved the `read_only` notice to the card top; the 「API Key configured」 notice appears
    below the key row only when the selected provider is the current runtime configuration;
    the error section remains at the bottom of the card.
- `ui/src/i18n/{zh,en}.ts` — added `refreshModels / refreshedModels / addModel /
  addModelPlaceholder / addModelConfirm / catalogKeyHint`; updated `noModels` and
  `customProviderHint` copy (no longer pointing to the deleted form).

## Interaction contract

- One intent, one action remains: click model/add model/chip/top-bar row = select and take
  effect; edit/delete registry = local-only; the key is submitted with application.
- After removing the main form, the sole entry point for any model id is 「Add」; the sole
  entry point for any custom combination is the 「Add custom provider」 dialog—both genuinely
  persist and take effect, with no fake operation.
- 「Sync from official」 does not fake network behavior: it only reloads the static snapshot
  and states that feedback clearly; the online-sync task remains in `UI-PROV-RPC`.

## Explicitly not done

- Real online provider-catalog synchronization (the Go side has no provider/model catalog
  RPC)—`docs/TODO.md` §0.1 `UI-PROV-RPC` remains OPEN.
- Editing catalog-provider rows (official entries cannot be changed; editing is limited to
  custom providers).
- Browser smoke test: 8787 / 3015 were still occupied by the user's Vivy Studio session,
  so self-testing was skipped and delegated to that session (see `verification.md`).

## Release notes

No standalone release: shipped with the regular UI build; `just ci` already includes the UI
build, so no separate `release.md` was written.
