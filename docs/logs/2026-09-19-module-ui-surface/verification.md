# Verification

## Commands and results

| Command | Result |
| --- | --- |
| `go run ./sdk verify plugins/vivy-{persona,evolution,memory,notebook}` | `ok` for all four after rebinding the source digests |
| `go run ./sdk stage-ui --recipe recipes/default.vivy.yml --out ui/src/generated` | exit 0; four extensions staged |
| `go run ./sdk/internal/cmd/generate-default --repo . --output internal/generated/assembly/zz_default.go` | exit 0 |
| `gofmt -l plugins/... sdk/internal sdk/ui` | empty |
| `git diff --check` | exit 0 |
| `cd ui; pnpm typecheck` | exit 0 |
| `cd ui; pnpm test` | 48 files, 392 tests passed |
| `cd ui; pnpm build` | exit 0, `dist/assets/index-*.js` 1,425.90 kB |
| `just ci` | see below |
| Playwright geometry audit (embedded build, 1440×900) | every Module page slot = 844 px = `main`, one `h1`, one banner, no clipping |
| Playwright dev-loop smoke (Vite 3016 + backend 8787) | 1 passed |
| `pnpm exec vitest run src/plugins/presentation-host.test.tsx src/plugins/host-icons.test.ts` | 31 passed |

## Geometry audit (before → after)

The first run of the audit drove this change: the slot was a bare `<div>` and
each page's `h-full` collapsed to its content height.

```
/persona    slot 345px -> 844px, h1 1 -> 1,  clipped false
/evolution  slot 530px -> 844px, h1 1 -> 1,  clipped false
/memory     slot 320px -> 844px, h1 0 -> 1,  clipped false
/notebook   slot 269px -> 844px, h1 0 -> 1,  clipped false
/masks      slot    -     844px, h1 1 -> 1,  clipped false   (core page, unchanged)
```

After: `slotClass = "flex h-full min-h-0 flex-col"`, exactly one
`[data-vivy-presentation-page-header] h1` per page (the host's), one demo
banner, and `main.scrollHeight === main.clientHeight` (nothing clipped).

The final embedded run after the catalog copy was added reports the same
geometry for all four pages — `slot 844 = main 844`, `header 82`,
`content 728`, one `h1`, one header icon, `clipped false` — with resolved page
titles and zero console/page errors.

## Dev-loop smoke (the required real path)

Temporary Playwright config against `vite --port 3016` with the running backend
on 8787 (the long-running lane servers on 3015/8787 were never touched; only
HTTP was used). Asserted, in Chinese chrome:

- sidebar VIVY group = 人格 / 面具 / 进化 / 记忆 / 记事本 in that order, one icon
  per entry;
- for `/persona`, `/evolution`, `/memory`, `/notebook`: exactly one
  `div[data-vivy-presentation-route]`, a header whose `h1` is the page title and
  which carries one icon, a `[data-vivy-presentation-page-content]` region,
  `slot.clientHeight === main.clientHeight`, the content region accounting for
  the frame minus header and banner (≤2 px of separators), no clipping, and the
  sidebar still present with its VIVY group;
- zero console errors and zero page errors.

Temporary artifacts (`ui/e2e/tmp-layout-shots.spec.ts`,
`ui/e2e/tmp-dev-loop-smoke.spec.ts`, `ui/playwright.dev-tmp.config.ts`,
`ui/test-results/`) were removed before committing. Screenshots from the audit
are outside the repository in `.workspace/ui-shots/`.

## Two defects found and fixed while verifying

1. **Stale SDK copy.** `defineUIRoute` was `undefined` inside the test run while
   typecheck (also through `node_modules`) rejected it: both resolved
   `ui/node_modules/@vivy/ui-sdk`, a pnpm copy of `sdk/ui` made at install time,
   not the source the app builds from. Fixed by aliasing `@vivy/ui-sdk` to
   `../sdk/ui/src` in `ui/vitest.config.ts` and `ui/tsconfig.json` (with React
   deduped) so dev, build, test, and typecheck tell one story.
2. **Missing catalog copy.** `/memory` and `/notebook` rendered the surface
   header before their catalogs had `title`/`subtitle` units, which the dev-loop
   smoke caught as `[missing translation: plugin.vivy/memory.title]`. Both
   catalogs gained the units (English + Chinese) and the digests were rebound.

## Skipped

- `pnpm e2e`: pre-existing locale expectations (recorded as
  `EMBEDDED-E2E-LOCALE` in `docs/TODO.md` §0.1); unchanged by this iteration and
  not part of `just ci`.
- No Go behaviour changed, so no new Go test was added; the module digests and
  the default generation baseline were re-verified through `just ci`.