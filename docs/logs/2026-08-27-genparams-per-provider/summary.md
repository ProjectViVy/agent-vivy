# Move the generation-parameter demo into 「Settings → General → Advanced Features」, edited independently per model

## Problem (user feedback, converged over three rounds)

1. The initial proposal, 「move the generation-parameter demo into the right side of a
   specific Provider configuration」, overloaded the provider panel, so the user stopped it.
2. Final direction: generation parameters are **not placed under the provider** and are not
   shown as a separate card in the Model tab; they are moved into the 「Advanced Features」
   section of the 「General」 tab and made model-specific—select a specific model from a
   dropdown, and generation parameters are editable **only for the selected model**.

## Changes

- `ui/src/components/settings/GenerationParamsCard.tsx` (new, Advanced Features card on
  the General tab):
  - card header 「Advanced Features」 (icon + description: a demo feature edited per model,
    with data stored only in the current browser);
  - inner section 「Generation parameters + Demo」 badge + description that they are saved
    per model;
  - **model dropdown** = selected-model quick list ∪ current runtime model (a model need
    not be in the quick list to be editable); defaults to the current runtime model, then
    the first available model;
  - shows and edits the temperature slider (0–2, 0.1 step, live numeric display) and Max
    Tokens only for the model selected in the dropdown; 「Save demo parameters」 + success
    feedback; an empty-state notice appears when no model is available.
- `ui/src/components/settings/SettingsView.tsx`:
  - removed the standalone 「Generation parameters」 card from the Model tab, leaving only
    one 「Vivy model configuration」 card;
  - mounts `GenerationParamsCard` on the General tab after the welcome-wizard card;
  - removed the obsolete `demoConfig` / `persistDemo('model')` branch and import; the Tools
    tab demo is now limited to tools (`loadTools` / `persistTools`).
- `ui/src/lib/demo-api.ts`: `getDemoGenParams(modelKey)` /
  `saveDemoGenParams(modelKey, params)`—stores `vivy.demo.gen-params` independently per
  model runtime triple key `provider/baseUrl/model` (unsaved/corrupt values fall back to
  0.7 / 4096; reads do not write the store).
- `ui/src/lib/types.ts`: added `DemoGenParams`.
- `ui/src/i18n/zh.ts` / `en.ts`: added `settings.advancedFeaturesTitle` /
  `advancedFeaturesDescription` / `modelSelect` / `genParamsModelHint` /
  `noModelsForGenParams`; `generationParamsDescription` now describes independent
  per-model saving.

## Not done (explicit boundaries)

- Generation parameters remain a demo surface (`vivy.demo.*`, not sent to a real Provider)
  and were not promoted to real configuration.
- No resizable/collapsible interaction was added (mentioned in the user's first message but
  not required by the final direction); the 「Advanced Features」 card is flat.
- `getConfig`/`updateConfig`/`getConfigStatus`/`getRuntimeConfig` were not removed (they
  remain as a runtime-configuration demo surface, with no UI consumer in this round).
- The root-tree compaction-section changes were not touched (uncommitted content in another
  parallel lane).
- Existing stale assertions at `ui/e2e/runtime.spec.ts:84` and
  `welcome-wizard.spec.ts:33` are an existing issue (`docs/TODO.md` §0.1
  `UI-E2E-STALE`) and were not changed.
