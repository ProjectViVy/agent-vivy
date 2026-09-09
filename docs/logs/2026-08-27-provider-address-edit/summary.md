# Persistent model-address edit entry (right-header pencil + catalog clone as custom)

## Problem

The user reported 「still cannot see where to customize and edit the model address」: the
pencil edit button from the previous round was rendered only on the left-side
**custom-provider** row—if the user had not added any custom entry yet, no edit button
appeared anywhere on the page. The right header was precisely where the model address (Base
URL) text was shown, yet it had no edit entry at all, so a user looking at the address text
naturally could not find the entry point.

## Changes

- `ModelSettingsCard.tsx`:
  - in the selected provider's right-side header, **added a persistent pencil button beside
    the address text** (aria/title = 「Edit address and alias」).
    - If the selected entry is a custom provider → open the **edit** dialog (address/alias/
      runtime bundle/default model/model list/API Key).
    - If the selected entry is a catalog provider → open the **Add custom provider** dialog
      with the catalog entry's display name/runtime bundle/address/default model/model list
      **pre-filled**; saving after changing the address creates a new custom entry (the
      catalog entry itself cannot be edited in place; cloning it as custom is the only
      existing path for changing the address).
  - `CustomProviderDialog` adds a `preset` prop (prefill in add mode; `editing` takes
    priority), reset together with `customDialog` state when the dialog is closed/reused.
  - Saving a catalog clone after changing only the alias, without changing the address,
    hits the existing `(bundle, baseUrl)` duplicate check; the dialog shows
    「That Base URL already exists...」 (the existing conflict rule, not relaxed).
- `i18n/zh.ts` / `en.ts`: added `settingsModel.editAddressAria` (zh/en parity).

## Not done (explicit boundaries)

- Editing the address does not immediately apply it as runtime configuration: after saving,
  the user must click a model in that provider's list (or add a model) to actually select
  the new address—preserving the single-entry semantics of 「click model = select and save
  immediately」.
- Real online catalog synchronization remains `UI-PROV-RPC` (static-snapshot reload) and
  was not implemented in this round.
- Sharing one key across different gateways in the same runtime bundle keeps
  `UI-MODEL-KEY-SCOPE` OPEN.
