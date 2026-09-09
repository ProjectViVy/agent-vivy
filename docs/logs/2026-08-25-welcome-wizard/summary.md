# Welcome Wizard port summary

Date: 2026-08-25
Status: complete

## Outcome

Ported Agent-Diva’s welcome page/first-configuration wizard as Vivy’s first-use
onboarding (`WelcomeWizard`): it opens automatically the first time a user opens
Vivy, guides them through setting a default model, and navigates to Chat / Model
Settings / Skills Library after completion. The wizard uses the real
`settings/get` and `settings/update` RPCs and collects no API keys (D-010; keys
are injected only by the runtime environment).

## Delivered

- `ui/src/hooks/use-welcome.ts` — first-visit completion marker (localStorage
  `vivy.ui.welcome.completed`) + open-state module, reusing the module-state +
  `useSyncExternalStore` pattern from `use-theme.ts`; 5 unit tests in
  `use-welcome.test.ts`.
- `ui/src/components/layout/WelcomeWizard.tsx` — 3-step React wizard
  (Introduction → Model → Complete), mounted in `_layout.tsx`, with a
  `fixed inset-0 z-[200]` backdrop + floating glow; the Model step uses the real
  `settings/update` through `saveSettings`.
- `ui/src/routes/_layout.tsx` — automatically calls `openWelcome()` after
  initialization completes, when there is no error and the first visit is not
  complete; the wizard renders at the app root.
- `ui/src/routes/_layout.settings.tsx` — validates the `?tab=` parameter as a
  valid `SettingsTab`, allowing the wizard to deep-link to the Model section of Settings.
- `ui/src/components/settings/SettingsView.tsx` — added a “Welcome Wizard”
  rerun-entry card to the General section; `initialTab` supports a route-specified
  initial section.
- `ui/src/i18n/zh.ts` / `en.ts` — added `welcome.*` entries (same structure,
  enforced by `i18n.test.ts`).
- `ui/src/styles.css` — `.welcome-float` floating-glow animation respecting
  `prefers-reduced-motion`.
- `ui/e2e/welcome-wizard.spec.ts` — full e2e: auto-open on first visit → skip →
  completion marker → refresh suppression → rerun from Settings → prefill → save
  → complete → deep-link → suppression after another refresh.
- `ui/e2e/runtime.spec.ts` / `ui/playwright.config.ts` — adapted for the wizard
  (skip dialog, localStorage allowlist), and fixed the e2e browser context to
  `zh-CN` (see the e2e fix note in verification.md).

## Design decisions

- **Provider uses runtime model bundle names**: the wizard accepts one of
  `openai / anthropic / mock`; DeepSeek and other OpenAI-compatible services
  connect through the Base URL gateway, without inventing provider names (the
  backend accepts only these three bundle names). Prefill chain:
  `settings.provider || settings.config_provider || recommended value`.
- **Do not collect secrets**: the wizard explicitly says “API keys are injected
  through runtime environment variables”; the form has only provider /
  default_model / base_url.
- **Do not import `demo-api.ts`**: model saving uses the real RPC.
- **Write the marker on completion**: `completeWelcome()` writes the localStorage
  marker and closes; skip also writes the marker. Subsequent opens therefore do
  not auto-open, while the Settings-page “Run Wizard Again” action can reopen it anytime.
- **Rerunnable**: the General section of Settings provides a rerun entry, and
  every wizard opening starts again at step 1 and pre-fills from the current real settings.

## Explicitly not done

- No provider secrets are collected or displayed in the UI (following the
  product constraint and introducing no secret input beyond `env_key`).
- The backend RPC contract was not changed; `settings/get` /
  `settings/update` fields are unchanged.
- No wizard-copy differences beyond localization were implemented (zh is the
  canonical dictionary).
