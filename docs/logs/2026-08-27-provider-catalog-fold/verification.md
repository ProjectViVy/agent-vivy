# Verification record — 2026-08-27 provider-catalog folding port

## Automated gates

| Command | Result |
|---|---|
| `cd ui; pnpm typecheck` | ✅ no errors |
| `cd ui; pnpm test` (vitest) | ✅ 13 files / 68 tests all passed (including 13 new `provider-catalog.test.ts` cases; i18n zh/en structure-sync tests passed) |
| `just ci` (repository root, = fmt-check + vet + go test + headless-compile + ui-ci[install/typecheck/test/build]) | ✅ all green, ui build `✓ built in 3.20s` (only the existing chunk>500kB warning) |

## Browser smoke test (split pair: `just run` :8787 + `cd ui; pnpm dev` :3015)

Validated each item at `http://127.0.0.1:3015/settings?tab=model` (built-in browser +
DOM snapshot assertions):

1. **Catalog rendering**: 27 common providers displayed flat (OpenRouter…Mimo); the
   Mock row matching the current configuration (mock) had a 「Current」 badge and was
   selected; the folded row 「More providers 20」 existed (count = 20, matching the diva list).
2. **Expand folding**: clicking the folded row revealed folded providers such as CherryIN /
   Together AI / Yi (01.AI) / PPIO / Cerebras.
3. **Search bypasses folding**: search "yi" → only CherryIN and Yi (01.AI) displayed flat
   (substring matches); the folded row was hidden, and DeepSeek/Mock were filtered out.
4. **Select provider (key mapping)**: search deepseek → click the DeepSeek row → the right
   side showed DeepSeek / https://api.deepseek.com/v1 / bundle label `openai` and 5 models;
   the form was filled with Provider=openai, default model=deepseek-v4-pro, Base
   URL=https://api.deepseek.com/v1.
5. **Select model**: clicking deepseek-chat updated the default-model input to deepseek-chat.
6. **Persist save**: click 「Save real settings」 → no error; the top-bar switcher became
   "DeepSeek | deepseek-chat"; the DeepSeek row gained the 「Current」 badge;
   `data/settings.yaml` actually contained
   `provider: openai / default_model: deepseek-chat / base_url: https://api.deepseek.com/v1`
   (full UI→RPC→settings.Save path verified).
7. **Top-bar switcher**: the dropdown showed 「Current configuration DeepSeek | deepseek-chat」
   and 「DeepSeek available models」 (deepseek-v4-pro / v4-flash / coder / reasoner, with the
   current model deduplicated).
8. **Environment restoration**: after smoke testing, restored `data/settings.yaml` to the
   mock triple and refreshed; the top bar returned to "Mock | mock", and the Mock row regained
   the 「Current」 badge.

## Tool note

The built-in browser (IAB) used for smoke testing timed out when locator-clicking elements
inside a scroll container (an actionability-detection defect), so DOM-node clicks were used
for all interactions. This is a browser-tool quirk, not a product defect—the elements respond
to real user clicks normally (keyboard/coordinate/node paths all triggered the same React
handler).

## Conclusion

`just ci` green + real-path smoke passed, satisfying the `smoke-for-user-visible-change`
and `just-ci-is-the-gate` rules.
