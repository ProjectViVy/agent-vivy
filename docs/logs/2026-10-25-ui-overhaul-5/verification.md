# UI overhaul Block 5 — verification

Environment: macOS arm64 worktree
`/Users/mastwet-macmini/pjvv/agent-vivy/.worktrees/ui-overhaul/agent-vivy`
(branch `feat/ui-overhaul`). UI gates run with the pinned toolchain
(`PATH=$HOME/.local/bin:/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.4`);
localStorage suites with `NODE_OPTIONS=--localstorage-file=/tmp/vitest-ls.db`.

## Static gates

| Command | Result |
| --- | --- |
| `pnpm typecheck` (stages assembly first) | clean, `tsc --noEmit` no errors |
| `pnpm test` (88 files) | 709/710 passed; the single red is the pre-existing `STORE-RUN-FAILED-COPY` (`src/lib/store.test.ts:568`), unchanged from stable |
| `pnpm vitest run src/components/layout/WelcomeWizard.test.tsx src/components/layout/welcome-model.test.ts src/hooks/use-welcome.test.ts` | 38/38 passed after the split |
| `pnpm vitest run src/theme/tokens.test.ts` | 7/7; src palette 137 (baseline 137), src textSize 35 (35), hex 0; plugins 29/45/0 untouched |
| `pnpm build` | built in 1.78 s |
| `node scripts/check-i18n-cross-face.js` | PASS: 13 shared semantic units |
| `node scripts/check-i18n-cross-face.test.js` | 8 tests, 8 pass |
| `node scripts/check-i18n-completeness.js` | exit 1, unchanged pre-existing red: three `Uncatalogued Han` rows at `ui/src/lib/failure.ts:56-58` (tracked in `docs/TODO.md` I18N-CHECK-RED) |
| `git status ui/src/generated` | empty — assembly regeneration produced no diff |

## Browser evidence

Isolated pair only: backend `127.0.0.1:8796` with
`VIVY_USER_HOME=/tmp/vivy-ui-overhaul` and the mock provider on
`127.0.0.1:9913`; worktree Vite on `127.0.0.1:3016`. The owner's daily pair
(`:3015`/`:8787`) was not touched. Capture script: `ui/.audit/capture-block5.mjs`
(gitignored); artifacts in this directory: `block5-report.json` plus 13 PNGs
(`nav-open/nav-toggled/settings/long-title` at 1280 and 390, `wizard-intro`
and `wizard-model` at 390 and 1280).

Measurements from `block5-report.json`:

| State | Document client/scroll | Notes |
| --- | --- | --- |
| nav open / toggled / settings / long-title @1280 | 1280/1280 | no horizontal document overflow |
| nav open / toggled / settings / long-title @390 | 390/390 | same |
| wizard intro / model @390 | 390/390 | card 358 px wide inside 390 viewport |
| wizard intro / model @1280 | 1280/1280 | card 512 px |
| settings tab list @390 | 358 client / 358 scroll, height 100 | wraps to three rows instead of clipping |
| settings tab list @1280 | 832/832 | single row |
| mobile nav sheet @390 | overlay 390 wide, rail inside | `nav-toggled-390.png` shows the 288 px rail over the scrim |
| collapsed rail @1280 | wrapper width 0 | the 12 non-`sr-only` "offenders" in `navToggled` are children clipped inside the intentional `w-0 overflow-hidden` wrapper; document does not scroll |
| long session titles | `clientH 39 < scrollH 59` | two-line clamp active with `title` tooltip (live probe of `span.line-clamp-2[title]`) |

Remaining offenders in every state are `sr-only` spans (screen-reader text,
`clientWidth 1`), which do not affect layout.

## Not run

- `just ci` / `just wiring-ci` / `just kernel-ci`: no Linux/WSL checkout on
  this macOS host; Block 5 is UI-only and the UI gates above are the
  affected surface. Recorded per the validation policy.
- Embedded-UI e2e (`pnpm e2e`): known red on stable (EMBEDDED-E2E-LOCALE),
  not re-run.
