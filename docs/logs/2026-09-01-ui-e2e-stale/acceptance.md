# Acceptance: how a human verifies it

## Suite level

- Run `just ui-e2e` from the repository root and expect
  `1 skipped / 9 passed` at the end (without a real provider environment, the
  provider cases beyond the runtime no-provider branch are skipped), with exit
  code 0.
- `ui/test-results/` no longer contains screenshots of raw i18n keys (such as
  `welcome.provider`) from `welcome-wizard`; the wizard's "Configure model"
  step displays copy rather than a key name such as `welcome.provider`.

## Wizard copy (first-use path)

- Clear `data/dev-home/settings.yaml`, then open `http://127.0.0.1:3015`:
  the wizard's second-step title is "Configure model", and the body describes
  the Provider runtime bundle (openai / anthropic), Base URL, and default model,
  explicitly stating that the API Key is injected by the runtime environment and
  is not collected by the wizard.
- Switching between Chinese and English produces no missing-key rendering in the
  same form (the original defect displayed `welcome.provider` as literal text).

## Settings → Model: adding a model no longer intermittently reports internal error

- Custom provider row → Add → enter a model ID → press Enter/click Add:
  the model is applied only after the registry write completes; on success, the
  new model appears in the list and becomes the current model.
- Repeated rapid actions (double-clicking/pressing Enter) no longer produce a red
  "internal error" bar or leave the registry missing the newly added model.

## Kernel: concurrent settings-document corruption (a separate UI-independent defect)

- The concurrent-save scenario (test `TestSaveConcurrentWritersKeepDocumentValid`,
  including `-race -count=3`) passes; Windows no longer reports access denied when
  rename overwrites a file with a concurrent read handle.
- Regression command: `go test ./internal/app/settings/ -race -count=3` exits 0;
  before the fix, it reproduced `settings: commit: rename ... Access is denied`.
