# UI overhaul Block 7 — verification

Environment: isolated evidence pair on the `feat/ui-overhaul` worktree —
backend `127.0.0.1:8796` (`VIVY_USER_HOME=/tmp/vivy-ui-overhaul`, mock provider
at `127.0.0.1:9913`), worktree Vite `127.0.0.1:3016`. The owner's daily pair
(`:3015`/`:8787`) was not used. Go-invoking UI scripts ran with
`PATH=$HOME/.local/bin:/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.4`; vitest
localStorage suites with `NODE_OPTIONS="--localstorage-file=/tmp/vitest-ls.db"`.

## Gates run

| Command | Result |
|---|---|
| `cd ui && pnpm vitest run src/theme/tokens.test.ts` | 7/7 pass. src palette/textSize/hex = 0/0/0; **plugins palette/textSize/hex = 0/0/0** (was 29/45/0). |
| `cd ui && pnpm typecheck` (stages assembly first) | Clean, no diagnostics. Staging regenerated `ui/src/generated/ui/**` from the edited originals. |
| `cd ui && pnpm test` | 709/710. The single failure is the known pre-existing `STORE-RUN-FAILED-COPY` (`src/lib/store.test.ts:568`), unchanged from Blocks 2–6 and unrelated to this diff. |
| `cd ui && pnpm vitest run ../plugins/vivy-masks-ui ../plugins/vivy-memory ../plugins/vivy-persona ../plugins/coding/session-tree ../plugins/vivy-workflow` | 11 files / 54 tests pass, including `mask-ui.integration.test.tsx` (12), persona `view.test.tsx` (10), memory `view.test.tsx` (9), `MaskPage.test.tsx` (2). Plugin tests assert textContent/roles/data-*, so the className migration is test-safe as expected. |
| `cd ui && pnpm build` | 1.56s, 2603 modules, CSS 121.63 kB. Semantic utilities used by plugins generate from the host `@theme`. |

## Browser evidence

`node .audit/capture-block7.mjs` (gitignored harness) visited the seven plugin
routes (`/persona`, `/evolution`, `/memory`, `/notebook`, `/masks`,
`/workflows`, `/session-tree`) across all seven theme families at widths
390/768/1280 = **147 combos**, measuring document overflow and element-level
offenders, and screenshotting `default`/`miku`/`pink` at 1280 (21 PNGs in
`shots/`). Roll-up in `block7-report.json`:

- **0 document-level horizontal overflow** across all 147 combos.
- **0 page errors** across all 147 combos.
- 35 combos report element offenders, all at narrow widths (390/768) and
  identical across every family, i.e. structural and pre-existing:
  - `/persona` and `/evolution`: deliberate `overflow-x-auto` scrollers (the
    IDENTITY.MD/RELATIONSHIP.MD pill strip; the Skill/Requests/Auto TabsList).
    Benign by design, same class as the Block 6 lifecycle TabsList offender.
  - `/workflows`: the editor toolbar row is ~830px wide and the frame's
    `<main>` is `overflow-hidden`, so at ≤390px the right-hand actions clip.
    Pre-existing (the Block 7 diff is colour/type-size only) and now tracked as
    `UI-WORKFLOW-MOBILE-CLIP` in `docs/TODO.md` §0.1.
- Screenshots confirm the semantic tokens resolve and adapt per family: the
  masks identity chips render as warn/info/ok/brand soft tints in `default`,
  re-hue to the Miku gold/teal palette in `miku`, and pick up Diva magenta for
  brand in `pink` — the behaviour hardcoded `*-100`/`*-500/15` literals could
  not produce.

## Checks skipped and why

- `just ci` / `just wiring-ci` / `just kernel-ci` (Go lanes): not run on this
  macOS host (no Linux/WSL checkout). Block 7 changes no Go source, so the
  kernel gates are unaffected.
- `just plugin-ci` (per-plugin `go vet`/`go test`): not run; it exercises the
  plugin Go modules and Block 7 changes only plugin `.tsx` UI sources. The
  plugin UI test lane is covered above via the `ui` vitest include globs.
- i18n gates: not re-run; Block 7 changes no i18n keys or copy. The known
  `I18N-CHECK-RED` completeness red is unchanged.
