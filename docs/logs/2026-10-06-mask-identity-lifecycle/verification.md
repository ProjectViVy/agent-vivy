# Verification

## Environment and source

Validation runs in `/workspace/agent-vivy` using Go 1.26.4, the repository's pnpm UI dependency lock and PowerShell/just toolchain. Commands below use the task toolchain on PATH and `GOPROXY=https://proxy.golang.org,direct`. Browser checks use installed Chromium at `/usr/bin/chromium`, through a temporary Playwright config extending `playwright.masks.config.ts` with `launchOptions.executablePath` and `--no-sandbox`. The CLI `agent-browser` was unavailable; the repository's installed Playwright exercised the actual split application instead.

Source lane: authoritative Module files under `plugins/vivy-masks-ui/ui/vivy-masks/src`, host shell/UI-kit changes under `ui/src`, no hand edits under generated sources. SDK staging refreshed the tracked `ui/src/generated/assembly.ts` projection. Source pin: `2f99200d1d7820b7428f893625a0115e5c9a0429b77a766d510f2fa8d059e2ca`.

The required `vivy-plugin` and `vivy-kernel-ci` paths were followed. Its referenced `oil-frontend` sub-skill was not available after filesystem/catalog search; repository `testing-vivy-ui` and the available browser/React skills supplied the UI verification workflow. No permission gate was inferred from that missing reference.

## Checks

| Command | Result |
|---|---|
| `pnpm --dir ui exec vitest run ../plugins/vivy-masks-ui/ui/vivy-masks/src` | PASS: 6 files, 26 tests. Covers late reads, draft safety, shared selection, CAS recovery, create idempotency, definitive rejection correction, reload failure/retry and a slow post-save catalog refresh. |
| `go run ./sdk verify plugins/vivy-masks-ui` | PASS: `ok vivy/masks-ui`. |
| `go run ./sdk pack --recipe recipes/masks-selected.vivy.yml --output /tmp/vivy-mask-identity-final` | PASS: sealed executable and manifest produced from the final source pin. |
| `go run ./sdk inspect-artifact /tmp/vivy-mask-identity-final` | PASS: artifact parsed statically; `vivy/masks-ui` T1 source identity and `std/ui-extension@v1` contribution match the descriptor. |
| `VIVY_MASKS_E2E_BINARY=/tmp/vivy-mask-identity-final/vivy pnpm --dir ui exec playwright test --config .playwright-mask-identity.config.ts --timeout 30000` | PASS: 11/11 real browser stories, no retries or skipped stories (1.1 minutes). |
| `just ci` | PASS, exit 0: bootstrap, Go format, UI typecheck, 72 UI test files / 568 tests, production build, en/zh and cross-face audits, Go vet/tests, headless compilation and all independent plugin/face gates. |
| `git diff --check` | PASS before final delivery review. |

Inspected generation ID: `20aa0d1fface5fd0dde9ab4e7c6e19854c0937071ff2d95e73b2a0b499ebb8e4`. Internal Provider source/conformance bindings are unchanged; the UI Module source pin was rebound using the SDK's normalized source-tree hash with its current declared digest.

SDK artifact tests passed (`sdk/internal`: 410.535s); assembly tests passed (10.764s); executed conformance reproduction passed (73.997s). The full gate includes the SDK generation pressure matrix and normal Host/Port suites. Final read-only review found no remaining material defects after the failure-path corrections.

The first complete CI attempt passed typecheck and all 568 UI tests, then caught four new unclassified Web language keys. Those keys were added to `scripts/i18n-cross-face-contract.json`; the direct Web/TUI en/zh contract check passed, and the complete gate was restarted. Earlier browser checks exposed and corrected an implicit textarea label, menu-visibility timing in the test helper, animation-time bounds checks and toolbar shrink behavior. The final browser fixture also creates an independent Chinese session and uses the existing vendor alias through `settings/model/select`, rather than treating an undeclared registry display name as an embedded vendor. The active-reply story verifies the immutable-reply hint after navigation as well as during selection. Final results above replace those development failures.

## Real product path

The browser suite starts the sealed masks-selected backend on `127.0.0.1:8787`, a deterministic hanging provider on `127.0.0.1:9911`, and Vite on `127.0.0.1:3015`. Sessions are created through the application's peer-owned UI. The fixture uses disposable `.e2e-masks-workdir` storage and the normal backend full-auto policy profile for effectful mask actions; backend authorization and reference checks remain active. No tenant `data/vivy.db`, `data/demo` or `data/workspaces` reads/writes occur. The browser fixture is SQLite-only; no live PostgreSQL DSN was supplied, and no schema or backend persistence behavior changed.

Eleven browser stories cover default identity; preview/use separation; page/header synchronization and persistence; create/edit/conflict/deletion; built-in duplication and guarded cancel; Code/Life placement; active-reply switching; four viewport widths; and fully visible Chinese default identity at 320 pixels. The fixture uses a mock model only to keep an admitted reply active. It does not fake the mask catalog, storage, selection, revision or delete behavior.

Temporary configs, disposable backend state, traces and packed scratch artifacts are removed before delivery. No separate release.md is needed: this delivers source on main, not an external binary release. Figma reference gaps and the unreproduced earlier runtime-config network failure are explicitly limited in summary.md; they are not reported as verified fixes.
