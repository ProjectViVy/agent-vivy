# Verification

Commands run from the worktree root (`agent-vivy-vc0`, branch `feat/vc1a-bash-tool`):

| Command | Result |
|---|---|
| `just ci` | Green. UI `tsc --noEmit` clean; `vitest run` 24 files / **195 passed** (including 9 zh/en key-equivalence checks in `src/i18n/index.test.ts` and 1 section-uniqueness check in `diva-preview-data.test.ts`); `vite build` succeeded (the chunk-size warning is pre-existing). |
| `just ui-e2e` | **10 passed / 1 skipped** (21.4s, workers=1, real-kernel webServer at 127.0.0.1:8799). |

The e2e specs sensitive to this change all passed:

- `network-tools-setting.spec.ts` — asserts the network-tab trigger copy
  "Network tools"; this item changed the `tabs.network` value from "Network" to
  "Network tools", and the assertion hit the real rendered text.
- `language-setting.spec.ts` — the main language-switch persistence path, covering
  bilingual rendering of all new keys in this item.
- `compaction-setting.spec.ts` — the compaction-card regression is unaffected by
  this `DivaSettingsPreview` change.
- `runtime.spec.ts` / `welcome-wizard.spec.ts` / `genparams-advanced.spec.ts` /
  `sandbox-setting.spec.ts` / `mcp-settings.spec.ts` / `model-refresh.spec.ts` /
  `files-panel.spec.ts` — all green.

Smoke-for-user-visible-change note: the user-visible behavior in this item is the
rendered copy for each Settings tab and preview section in Chinese and English.
The `just ui-e2e` webServer uses the real `go run ./cmd/vivy` kernel and the real
built UI artifact; the specs use Playwright to open the Settings page and assert
the rendered localized text (including the language-switch spec), which is
equivalent to opening each section in the split Vite instance on 3015. A separate
3015 session was not started.

Skip item: `cron-tasks.spec.ts` has a pre-existing skip (requires a real
provider), unrelated to this item.

## Notes

- In `git status`, `ui/src/routeTree.gen.ts` is generator churn and was not
  included in the commit.
- `src/i18n/index.test.ts` gates zh/en key equivalence; the new top-level
  `divaPreview` section is synchronized on both sides and does not break that
  test.
