# Provider-edit entry polish: 「Manage provider」 dialog semantics + unified header-button layout

## Problems (user feedback)

1. Opening the dialog from the pencil (Edit) entry in the right header showed the title
   「Add custom provider」, which conflicted with the intent to edit; the user wanted this
   entry to correspond to 「Manage provider」.
2. The previous round placed the pencil button inside the address-text row (small icon h-3,
   p-1), inconsistent in size and position with the 「Sync from official」 / 「Add」 icons in
   the upper right (h-3.5, p-1), making the header-button layout messy.

## Changes

- `ModelSettingsCard.tsx`:
  - dialog title/copy now distinguish three semantics by source:
    - `editing` (custom entry) → 「Edit custom provider」;
    - `preset` (catalog entry, pencil entry, prefilled clone) → 「Manage provider」 + a
      dedicated explanation (it becomes a custom provider after saving, while the original
      catalog entry remains unchanged);
    - empty add (left-side 「＋ Add custom provider」) → 「Add custom provider」.
  - rebuilt the right-header layout: address text returns to a plain text row (no embedded
    button); the pencil (Edit), Refresh, and Add icon buttons now use **the same row, size
    (h-4 w-4), padding (p-1.5), and spacing**, positioned after the right-side runtime-bundle
    badge and visually aligned as one group.
- `i18n/zh.ts` / `en.ts`: added `settingsModel.customDialogTitleManage`
  (Manage provider) and `customDialogHintManage` (manage-semantics hint).

## Not done (explicit boundaries)

- Catalog entries still cannot be modified in place: saving the management dialog creates a
  new custom entry and leaves the original catalog entry unchanged (conflict validation and
  the existing `(bundle, baseUrl)` priority rule were not relaxed).
- Saving after changing only the alias, without changing the address, still hits duplicate-
  address validation (existing rule).
- Real online catalog synchronization `UI-PROV-RPC` and same-bundle, multi-gateway keys
  `UI-MODEL-KEY-SCOPE` remain OPEN.
