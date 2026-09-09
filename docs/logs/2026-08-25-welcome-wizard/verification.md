# Verification

Date: 2026-08-25

## Gate: `just ci` (repository root)

```
just ci
```

Result **EXIT=0**, all passed:

- Go: fmt-check / vet / test / headless-compile clean
- UI typecheck: `tsc --noEmit` clean (`src/**`)
- UI unit tests: **51 passed (11 test files)**, including the new
  `src/hooks/use-welcome.test.ts` (5 cases)
- `vite build` succeeded (only the existing chunk-size notice, not introduced here)

## E2E: `just ui-e2e`

```
just ui-e2e   # pnpm build + playwright test (bundled backend 127.0.0.1:8799, mock runtime)
```

Result **2 passed (15.7s)**:

- `welcome-wizard.spec.ts` — auto-open on first visit → skip → completion marker
  `'1'` → refresh suppression → “Run Wizard Again” on the Settings page → Model
  step prefilled with `mock / mock:hitl` → save `mock / mock` → Complete step →
  “Model Settings” deep-links to `/settings?tab=model` (tab selected, Provider
  value `mock`) → suppression after another refresh
- `runtime.spec.ts` — existing control-plane end-to-end regression passed
  (including the new wizard skip)

### e2e fix record (found and fixed in this iteration, not a leftover)

1. **Browser-locale context mismatch**: Playwright’s default context was `en-US`,
   so the app rendered English through `detectInitialLocale()`; all Chinese
   selectors in the existing spec (such as `Settings` and `Enter a message...`)
   failed. Fix: added `locale: 'zh-CN'` to `use` in `playwright.config.ts`, so e2e
   consistently simulates a Chinese user.
2. **Wizard-prefill assertion**: e2e backend `runtime.mock: true` makes the
   effective provider `mock` and default model `mock:hitl` (`defaultModelFor`
   derives it from the mock scenario), rather than configured `active: openai`.
   Corrected the assertion to `mock / mock:hitl`.
3. **Stale “Back to List” assertion**: after the `MasterDetail` refactor, the
   return button copy is `t('common.back')` (“Back”); the three `Back to List`
   assertions in `runtime.spec.ts` were stale. Corrected them to “Back”. (This
   historical broken link was independent of this feature but fixed here so the
   full e2e suite could turn green.)

## Live-path smoke: http://127.0.0.1:3015 (split Vite)

Each item was checked manually in the developer browser (IAB); all matched expectations:

1. Settings → General shows the “Welcome Wizard” rerun-entry card and “Run Wizard Again” button.
2. Clicking “Run Wizard Again” opens the wizard in Chinese, rendering the brand
   (Vivy / Project ViVY), subtitle, three-step indicator (Start / Model / Complete),
   three features (Smart Chat / Skill Extensions / Lifecycle & Evolution), and
   “Skip Wizard”.
3. Clicking “Next” renders the Model step, with Provider / default model
   prefilled from real backend settings (`mock / mock`), an empty Base URL with a
   placeholder, the notice “API keys are injected through runtime environment
   variables; do not enter or save them in the UI,” and a working “Open in Browser” button.
4. Clicking “Next” renders the Complete step with “Ready!” and three navigation
   cards (Start Chat / Model Settings / Skills Library).
5. Clicking “Model Settings” closes the wizard (writes the completion marker),
   changes the URL to `http://127.0.0.1:3015/settings?tab=model`, selects the
   “Model” tab, and leaves `mock` in the Provider input.

Note: the development backend (`just dev` pair) has no `OPENAI_API_KEY`, so the
smoke test stays on the mock provider. The wizard’s save path and inline error
notice for a real provider (`settings: provider ... unsupported; want openai,
anthropic or mock`) were already verified during development smoke testing.

## Skip note

- No embedded-UI (:8787) smoke test was run: the development/verification path
  follows AGENTS.md and uses split Vite (:3015); the embedded UI is covered by
  `just ci`’s `vite build`, and the e2e `go run ./cmd/vivy` (:8799, embedded UI)
  already ran the full wizard flow as a regression.
