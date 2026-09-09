# UI-DIVA-PREVIEW-I18N — Fully internationalize DivaSettingsPreview + SettingsView

## Scope

- `ui/src/components/settings/DivaSettingsPreview.tsx`: removed hardcoded copy
  throughout the component and switched it to `useTranslation()`. GeneralPreview
  (preview notice / chat display / cache and runtime status / About Vivy / the
  "Compaction configuration has graduated" migration note) and SelfEvolutionPreview (the Self-evolution
  section's description, frequency options, confirmation-strategy toast, and five
  action rows) all use `t()`; action names use the dynamic key
  `t(\`divaPreview.actions.${action.id}\`)`, and the confirmation-strategy notice
  uses the interpolation `t('divaPreview.confirmUpdated', { label })`. Proper
  names such as MIT / projectViVY remain literal.
- `ui/src/components/settings/diva-preview-data.ts`: `DIVA_EVOLUTION_ACTIONS`
  now retains only IDs, with display copy moved into i18n
  (`divaPreview.actions.<id>`), avoiding the dual track of Chinese labels in the
  data file and a second translation in the component. `diva-preview-data.test.ts`
  has no label assertions and needed no changes.
- `ui/src/components/settings/SettingsView.tsx`: absorbed the same class of debt
  in this file: removed the `DIVA_TAB_LABELS` constant, changed every tab trigger
  to `t('settings.tabs.*')` (the DIVA additional section uses dynamic
  `t(\`settings.tabs.${…}\`)` plus the `t('divaPreview.previewBadge')` preview
  badge), and changed the execution-timeout, application-information, tools,
  lifecycle, Run Inspector, read-only notice, error notice, and save button to
  `t()`. Existing keys are reused where possible (title/subtitle/appInfo/tools/
  lifecycle/runInspector/readOnlyNotice/saving), with new
  `settings.executeTimeout*` and `settings.saveGeneral` keys.
- `ui/src/i18n/zh.ts` / `en.ts`: added a top-level `divaPreview` section (about 50
  keys plus nested `actions`), with equivalent zh/en keys; changed the value of
  `tabs.network` from "Network" to "Network tools" to match the existing visible
  copy (`network-tools-setting.spec.ts` asserts that the tab is named "Network
  tools");
  added the `settings.executeTimeout*` and `settings.saveGeneral` keys.

## zh copy policy

The zh copy for new `settings.*` keys was transferred character-for-character
from the original hardcoded text (execution-timeout card, error notice, and save
button). The preview area, which had previously leaked Chinese into the English
interface, received new English copy. The zh copy remains unchanged, so there is
zero user-visible Chinese regression.

## Not done

- `routeTree.gen.ts` is generated; its churn is not included in this commit
  (standing policy).
- Preview-area mock-data behavior is unchanged (the DIVA migration preview is
  still display-only and does not persist configuration); whether the preview
  area should graduate into a real section is outside this item (other TODO
  rows).
- i18n coverage for other components such as `Welcome`, `ChannelsSettings`, and
  `SandboxSettingsCard` is outside this item (`language-setting.spec.ts` already
  covers the main language-switch path).
