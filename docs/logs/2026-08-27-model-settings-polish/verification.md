# Verification record — model-settings page visual cleanup + themed generation-parameter card

## Automation

- `cd ui; pnpm typecheck` → passed (`tsc --noEmit` produced no output)
- `cd ui; pnpm test` → all 15 files and 105 cases passed, including
  `src/i18n/index.test.ts` (deep zh/en dictionary structure check) and
  `src/components/settings/*` (provider-catalog / custom-providers / saved-models).
- Repository-root `just ci` (fmt-check + vet + test + headless-compile + ui-ci:
  install --frozen-lockfile + typecheck + test + build) → passed (exit 0, including a
  normal `vite build`).
- Another parallel lane in the session removed the 「Personality」 demo tab from the same
  shared root tree (types.ts / demo-api.ts / e2e/runtime.spec.ts and the Personality section
  of SettingsView). Revalidation of the merged tree: `pnpm typecheck` / `pnpm test` (105 ✓) /
  `pnpm build` all passed; live smoke confirmed the 「Model」 tab works, the Personality tab
  was removed with the other lane, there were no console errors, and no horizontal overflow.
  That lane's files remain unstaged and belong to its commit.

## Live smoke test (split pair: Vite :3015 + control plane :8787, Playwright headless)

Page: `http://127.0.0.1:3015/settings?tab=model`, 1440×1000.

- No console errors and no horizontal overflow (`scrollWidth <= clientWidth`).
- Render assertions:
  - 「Vivy model configuration」 and 「Generation parameters」 card headers render normally;
  - 1 "Demo" badge and 1 English "Temperature" label;
  - 0 「Sync from official catalog」 buttons (required: removed).
- Interaction assertions:
  - click OpenRouter in the left column → right-panel title changes from Mock → OpenRouter
    (selection linkage works);
  - click the 「Add model」 button in the model-section header → an 「Enter model id and
    press Enter to apply」 input appears (Esc collapses it);
  - keyboard-operate the temperature slider 0.7 → 0.8 (ArrowRight) → 2.0 (End), with the
    right-side value text synchronized (0.7 → 0.8 → 2.0);
  - click 「Save demo parameters」 → 「Saved locally」 feedback appears.

Existing backend contracts and data flow were unchanged: this change only affects UI
presentation and i18n copy, does not touch settings RPC or the logic in
`custom-providers.ts` / `saved-models.ts` / `provider-catalog.ts`, and therefore adds no new
tests; existing settings unit tests were reused for regression.

## Not verified

- Actual contrast under dark/pink/Hatsune Miku themes was not screenshot-reviewed (no image
  input model); all controls use semantic tokens (bg-accent / bg-primary / muted), so they
  should adapt with the theme in theory, and the UI smoke test ran in the default theme.
- Two-column layout behavior at narrow viewports (< md) was not recorded separately (the
  existing `md:grid-cols-…` breakpoint was unchanged).
