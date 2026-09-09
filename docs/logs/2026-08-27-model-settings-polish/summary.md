# Model-settings page visual cleanup + themed generation-parameter card

## Problems (user feedback)

1. The 「Settings → Model」 page UI was confusing: the selected provider row combined a
   4px left border, accent background, and bold text as three simultaneous emphases; the
   「More providers」 and 「Add custom provider」 rows used `border-dashed + border-l-4
   border-l-transparent` to masquerade as list rows; three same-sized icon buttons in the
   right header (edit / sync from official catalog / add model) had different meanings but
   were placed side by side, and 「Sync from official catalog」 was only a static-snapshot
   reload fake operation (clicking it showed only an “reloaded” notice with no real result);
   the right header/API Key/model-list blocks were separated by consecutive `border-b`
   lines without clear grouping.
2. The 「Generation parameters」 card was disconnected from the theme: it contained a full
   amber `DemoNote` banner (repeating the description's notice that it was a “demo/not sent
   to the Provider”); labels were English Temperature / Max Tokens; the original numeric
   input had no range/value guidance; and the button and saved feedback were laid out
   haphazardly.

## Changes

- `ModelSettingsCard.tsx` (real model-configuration card):
  - removed the 4px left border from the selected provider row, converging on
    `bg-accent + text-accent-foreground + font-medium` (the accent language used by
    MaskAndModelSwitcher's current-configuration row); standardized hover to
    `hover:bg-accent/60`.
  - removed the `border-dashed / border-l-4` masquerade from 「More providers」 and 「Add
    custom provider」, changing them to ordinary ghost rows (muted text + hover accent)
    with the same height and padding as list rows.
  - standardized the left-column list container from `bg-muted/30` to `bg-card`, matching
    the right panel's surface language.
  - right column: removed the fake 「Sync from official catalog」 button
    (`handleRefresh`/`refreshNote` removed with it, and i18n `refreshModels`/`refreshedModels`
    removed from the zh/en dictionaries); moved the 「Add model」 button into the model-list
    section header (label uses the existing `settingsModel.modelsTitle` 「{{provider}} models」),
    leaving only provider name/address/runtime-bundle badge + edit pencil in the header.
  - promoted the 「API Key configured」 notice in the API Key section to small trailing
    text, reducing the stacked second paragraph; the read-only notice now uses
    `settings.readOnlyNotice` i18n and the project's amber convention (amber-600 /
    dark:amber-300, matching MaskAndModelSwitcher's readOnlyHint).
- `SettingsView.tsx` (settings page):
  - added the project's standard icon card header to both Model-tab cards
    (`h-9 w-9 rounded-lg bg-primary/10 text-primary`, `Cpu` for the model card,
    `SlidersHorizontal` for the parameter card), with titles/descriptions using existing
    `settings.*` i18n entries (the entries already existed but the page had hard-coded
    Chinese, creating a duplicate source of truth).
  - rebuilt the 「Generation parameters」 card: removed the full-card `DemoNote` banner and
    replaced it with a compact 「Demo」 badge beside the title; changed temperature to a
    themed slider (`Slider`, 0–2, 0.1 step, following the primary token) + live, fixed-width
    value display on the right; kept Max Tokens as a numeric input; added themed success
    feedback below the save button (primary checkmark + 「Saved locally」).
- `i18n/zh.ts` / `en.ts`: added `settings.demoBadge`; changed the zh value of
  `settings.temperature` to the English "Temperature" (previously English Temperature);
  updated `settingsModel.noModels` to point to the new button in the list header; removed
  obsolete `refreshModels` / `refreshedModels` entries. zh/en structure remains identical
  under the deep `i18n/index.test.ts` check.

## Not done (explicit boundaries)

- Settings-page tabs such as 「General / Personality / Tools / Vivy Features」 still use
  the existing hard-coded Chinese copy and were not all connected to `settings.*` entries
  (the page already mixed i18n and hard-coded copy; this round only closed the Model tab
  called out by the user).
- Real online catalog synchronization (UI-PROV-RPC in the documentation notes) remains
  OPEN; this change removes its fake UI placeholder, not the backend RPC plan.
- Generation parameters remain a demo surface (`vivy.demo.*`, not sent to a real Provider)—
  the demo identity is retained as a 「Demo」 badge + description and was not promoted to
  real configuration.
