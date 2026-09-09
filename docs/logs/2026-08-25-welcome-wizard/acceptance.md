# Acceptance

Date: 2026-08-25

## User perspective: how to tell the port succeeded

1. **Automatic first-use onboarding**: in a fresh browser (with no
   `vivy.ui.welcome.completed` marker), open `http://127.0.0.1:3015` (or the
   product URL) and wait for initialization. The “Welcome to Vivy” wizard opens
   automatically; its introduction explains that Vivy is a local AI agent and
   shows three steps (Start / Model / Complete).
2. **Skip without being bothered again**: click “Skip Wizard”; the wizard closes
   and does not reappear after refresh. `vivy.ui.welcome.completed` is `'1'` in
   browser localStorage.
3. **Rerun at any time**: go to Settings → General, find the “Welcome Wizard”
   card, and click “Run Wizard Again” to open it. The wizard starts at step 1 each
   time and pre-fills the current settings.
4. **Configure the default model**: in the Model step, enter a Provider (one of
   `openai` / `anthropic` / `mock`; DeepSeek and others connect through Base URL),
   the default model, and Base URL. The UI clearly states that API keys are
   injected by the runtime environment and are not entered or saved in the UI.
   Click “Next” to save; if backend validation fails, an inline notice appears in
   the wizard and it stays on the current step.
5. **Completion navigation**: after a successful save, the “Ready!” completion
   page offers three direct cards: Start Chat (Chat page), Model Settings
   (Settings page with the “Model” section selected and `?tab=model` in the URL),
   and Skills Library (Skills page).
6. **Persistent state**: after completion, refresh and the wizard does not open;
   the Model section in Settings shows the just-saved Provider / default model values.

## Acceptance path

- `just ci` is all green (51 unit tests, including 5 `use-welcome` cases).
- `just ui-e2e` has 2 passing tests (`welcome-wizard.spec.ts` covers the full
  flow in steps 1–6; `runtime.spec.ts` is the regression suite).
- The developer browser manually completes steps 1–5 at
  `http://127.0.0.1:3015` (see verification.md).

Note: if the developer browser has already clicked “Model Settings” / “Skip,”
the first-visit marker has been written. To verify automatic opening again,
clear `vivy.ui.welcome.completed` or use an incognito window.
