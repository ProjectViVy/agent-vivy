# R1 verification

Host: macOS arm64. Node 26.11.0, pnpm 12.10.1, Playwright 1.62.1, Chromium 1234 (installed under `.workspace/ms-playwright` because the default browser cache is outside the sandbox). Go checks used the pinned `go1.26.4`; the browser pack used the host Go 1.27.1 with `-tags=http2legacy` (the host toolchain needs that for the pinned grpc).

## Focused unit and type checks

- `./node_modules/.bin/vitest run plugins/vivy-notebook/ui/vivy-notebook/src/view.test.tsx plugins/vivy-notebook/ui/vivy-notebook/src/reports-view.test.tsx` — **16 passed** (view 5, reports panel 11). Coverage includes first-use controls, absent reports capability, settings read failure then successful re-probe, authoritative destination in an unloaded section, dirty-editor retention, and no duplicate panel.
- `pnpm --dir ui typecheck` — pass.
- `go run ./sdk verify plugins/vivy-notebook` — `ok vivy/notebook`.
- `node scripts/check-i18n-completeness.js` — PASS (en/zh 1575 keys, 162 placeholders); cross-face tests 8 pass; cross-face projection PASS (14 units). No new catalog keys were added.
- `go vet ./...` — pass.
- `GOFLAGS= go test -run '^$' -tags 'vivy_headless,http2legacy' ./cmd/vivy ./cmd/vivy-code ./ui` — pass.
- Full UI suite (`pnpm --dir ui test`) — **93 files / 707 tests passed** with `NODE_OPTIONS=--localstorage-file=...`; the same suite without that host flag has 39 pre-existing failures in unrelated persona/settings/reveal tests because Node 26 leaves `window.localStorage` undefined. This is a host-runtime precondition, not an R1 regression.

## Real browser path

Fresh packs were built under `.workspace/notebook-e2e/{default,no-notebook,no-reports}`; the default artifact is generation `18d3c6fd4b1e6dbad8e4dd9d0489921054a87b99c9d07c6f88d539e32790fcb2`, recipe `32366ca2c0f13ae9a5bc86da2e08734bfdfdf38767987934b7eb9bd35b9e7a76`, UI dist `066a0cb6e013410e51004c091dc32d4c3d269c83d6d67792b56196a01b673493`.

`node node_modules/@playwright/test/cli.js test --config playwright.notebook.config.ts --reporter=list` from `ui/` with `VIVY_NB_E2E_BACKEND_PORT=8798` and `VIVY_NB_E2E_UI_PORT=3025`:

- 8/8 passed.
- R1 case: empty Daily section → `notebook-entries-empty` + one `notebook-empty-reports` panel → open settings, save disabled schedule → Generate → exactly one entry selected, `report-provenance` present → generated revision, human edit/save, comment, regenerate.
- Omitted reports generation: `controls vanish…` passed (no panel).
- Generic Notebook: seeded sections/editor/create/save/comment/revision/move/delete/restore/export/restart/CAS all passed. The one generic assertion that reports must never surface was updated because R1 intentionally exposes generation in the unselected detail.

The first browser attempts exposed two environment/test-harness gaps and were fixed: the Playwright Chromium cache was absent, and the first-run Persona onboarding dialog intercepted the settings click. The notebook e2e helpers now dismiss the local onboarding marker as well as the welcome marker.

## Full CI context

`just ci` passed bootstrap/fmt/ui-ci/vet and stopped at the pre-existing `sdk/codeclient` `TestClientAgainstRealVivyCode` handshake failure (reproduced unchanged in the root checkout). The remaining recipes passed separately: `diva-recall-test`, `headless-compile`, `plugin-ci` (22 modules). This lane did not introduce that failure; no full-CI green claim is made. Details are in `docs/logs/2026-10-19-issue81-toolkit-gate/verification.md`.

Not verified: Linux and native Windows browser venues. macOS Chromium exercised the real split path.
