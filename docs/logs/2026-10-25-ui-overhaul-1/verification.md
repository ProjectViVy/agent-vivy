# Verification — UI overhaul Block 1

Host: macOS arm64. Tree: `.worktrees/ui-overhaul/agent-vivy` on
`feat/ui-overhaul`, parent `4c7f3fee` (Block 0). The root checkout carries
unrelated Go work, so this lane stays in its own worktree.

Environment followed Block 0's record: pinned `GOTOOLCHAIN=go1.26.4` for every
UI script that shells out to Go, `../laputa` symlinked for the worktree, and the
isolated dev pair on **8796 (backend) + 3016 (Vite)**. The owner's `:8787`,
`:3015` and the stray `:8798` were never touched, and no repository `data/`
path was read or written (`VIVY_USER_HOME=/tmp/vivy-ui-overhaul`).

## Gates

| Command | Result |
| --- | --- |
| `cd ui && GOTOOLCHAIN=go1.26.4 pnpm typecheck` | clean (`tsc --noEmit`, nothing after the stage step) |
| `cd ui && GOTOOLCHAIN=go1.26.4 pnpm exec vitest run src/hooks/use-theme.test.ts src/theme/sources.test.ts src/theme/tokens.test.ts src/plugins/presentation-host.test.tsx` | **58 passed** (4 files) |
| `cd ui && GOTOOLCHAIN=go1.26.4 pnpm build` | built in 1.5 s; `dist/assets/*.css` 155 kB |
| `node scripts/audit/contrast.mjs` | `contrast: 7 families, 147 pairs, all >= AA`, no out-of-sRGB tokens |
| `pnpm run audit -- --base-url http://127.0.0.1:3016 --out .audit/block1` | `231 screenshots, 0 failures` |
| pre-paint probe (`/tmp/prepaint-check.mjs`, described below) | 7/7 families correct before React boots |

### Ratchet (unchanged except hex)

```text
[token-ratchet] src/palette: 159 (baseline 159); 31 files (baseline 31)
[token-ratchet] src/textSize: 85 (baseline 85); 26 files (baseline 26)
[token-ratchet] src/hex: 5 (baseline 5); 1 files (baseline 1)
[token-ratchet] plugins/palette: 29 (baseline 29); 7 files (baseline 7)
[token-ratchet] plugins/textSize: 45 (baseline 45); 8 files (baseline 8)
[token-ratchet] plugins/hex: 0 (baseline 0); 0 files (baseline 0)
```

The hex baseline was lowered 21 → 5 and its file set 2 → 1 in this block, so the
ratchet now fails if a preview gradient or accent literal comes back. The
remaining five all live in `components/ui/chart.tsx` (deleted in Block 2).
The ratchet was re-run after every edit; `src/palette` and `src/textSize` did
not move because Block 1 migrates no component call sites.

### Build artefacts

`dist/index.html` contains exactly the two neutral pre-paint pairs and two
references to `vivy.appearance` (the header script writes one key). The built
stylesheet contains one block per family, both scopes
(`[data-theme=…]` and `[data-theme-preview=…]`), the dark appearance ramp, and
the new utilities (`text-2xs`, `bg-canvas`, `bg-brand`), so nothing in the new
token layer depends on dev-only CSS injection.

Checked with:

```text
for id in default dark pink miku rice fluor indigo; do grep -o "\[data-theme=$id\]" dist/assets/*.css | wc -l; done
# 1 1 1 1 1 1 1
grep -o 'data-theme-preview' dist/assets/*.css | wc -l   # 8 (7 families + one in the utility selector)
grep -o 'data-appearance=dark' dist/assets/*.css | wc -l # 1
```

## Contrast — measured, not asserted

`scripts/audit/contrast.mjs` parses `ui/src/styles.css`, applies the cascade in
declaration order, resolves `var()` and `calc()`, converts OKLCH → OKLab →
linear sRGB → relative luminance, and reports both WCAG ratio and sRGB gamut.
147 pairs = 21 foreground/background pairs × 7 families, each evaluated in its
own appearance.

| Family | Appearance | Canvas | `fg` on canvas | `fg-muted` on canvas | `brand` on canvas | `brand-fg` on brand |
| --- | --- | --- | --- | --- | --- | --- |
| Vivy light | light | `#f0f8fd` | 12.66 | 7.21 | 5.70 | 5.94 |
| Vivy dark | dark | `#0c1418` | 16.59 | 9.99 | 10.27 | 10.34 |
| Diva | light | `#fdf4f7` | 12.73 | 7.25 | 6.32 | 6.61 |
| Miku | dark | `#0d1414` | 16.59 | 9.99 | 10.40 | 10.48 |
| Rice & Vermilion | light | `#fcf6ed` | 12.70 | 7.23 | 6.58 | 6.87 |
| Graphite & Fluor | dark | `#121315` | 16.59 | 9.98 | 12.19 | 12.24 |
| Night Indigo | dark | `#10111e` | 16.61 | 9.98 | 9.14 | 9.22 |

Every ratio above is AA for body text (≥ 4.5). The script also checks each
status colour on the canvas and on its own `-soft` fill (lowest: `danger` on
`danger-soft` 5.92), and `fg-subtle` (lowest 5.82) — full output omitted here,
reproducible with `node scripts/audit/contrast.mjs`.

Twenty-eight token values had to be de-saturated to stay inside sRGB
(`bg-surface` .99 → .985, `ok` C .125 → .11, `warn` .1 → .085, `danger-soft`
and `info-soft` to `.945/.024`, light `*-fg` to pure white, dark `danger`
.16 → .135, pink/rice `brand-soft`, and Vivy-light `brand` .135 → .095, where
the maximum in-gamut chroma at L .49 h 228 is .096). The script fails on a clip,
so these values are the ones that actually render.

## Screenshot audit

```text
cd ui && pnpm run audit -- --base-url http://127.0.0.1:3016 --out .audit/block1
# audit: 231 screenshots, 0 failures -> ui/.audit/block1
```

231 = 11 surfaces × 7 families × 3 widths (390 / 768 / 1280). The family list
is read from `src/hooks/use-theme.ts`, so the audit cannot photograph a stale
skin set. Each context seeds both `vivy.theme` and `vivy.appearance`; if it
seeded only the theme id, a flash could be masked by the 600 ms settle before
the shot.

Committed evidence (`.audit/` is gitignored):

- `chat-empty-<family>-1280.png` — the shell in all seven families;
- `theme-picker-default-1280.png` — all seven preview tiles, Vivy Light selected;
- `theme-picker-dark-1280.png` — the same page in a dark family (preview tiles
  keep their own appearance);
- `theme-picker-rice-390.png` — one column at 390, warm family, selection ring.
- `prepaint/*.png` + `report.json` — see below.

Observed and pre-existing (not caused here, owned by later blocks): the
composer placeholder wraps to two lines at 390 px (identical in the Block 0
baseline), and the empty-chat title sits in the upper third of a 900 px viewport.
Block 4 and Block 3 own those.

## Pre-paint probe

`/tmp/prepaint-check.mjs` (one-off, not committed): for each family it seeds
both storage keys, then blocks `**/src/main.tsx` so neither React nor its CSS
import can arrive, waits for `<body>`, waits 500 ms, and screenshots. What is
captured is exactly what the browser paints from `index.html` alone.

| Family | `data-appearance` | `.dark` | root background | React mounted |
| --- | --- | --- | --- | --- |
| default | light | no | `rgb(240, 248, 253)` | no |
| dark | dark | yes | `rgb(12, 20, 24)` | no |
| pink | light | no | `rgb(240, 248, 253)` | no |
| miku | dark | yes | `rgb(12, 20, 24)` | no |
| rice | light | no | `rgb(240, 248, 253)` | no |
| fluor | dark | yes | `rgb(12, 20, 24)` | no |
| indigo | dark | yes | `rgb(12, 20, 24)` | no |

`data-theme` is still `null` at that point by design (React owns it); the
pre-paint path only promises the right *appearance* plus the neutral Vivy
literal, which is the plan's constraint. So a dark family never flashes white
and a light family never flashes dark; the only settle after hydration is the
hue for non-Vivy families.

## i18n

```text
node scripts/check-i18n-completeness.js      # exit 1, 3 errors — pre-existing
node --test scripts/check-i18n-cross-face.test.js   # 8 passed
node scripts/check-i18n-cross-face.js        # exit 1, 1 error — pre-existing
```

Both failures were verified to exist **at the parent commit**, by temporarily
restoring `ThemePicker.tsx` from `HEAD` and re-running:

- completeness: the three `Uncatalogued Han` rows are Chinese provider-failure
  markers in `ui/src/lib/failure.ts:56-58`, present at `HEAD` and untouched by
  this block. The first run of this gate also flagged the picker's `Aa` sample
  glyph; that string was moved to `t('settings.themeSwatchSample')`
  (`Aa` / `字`) so this block adds no new finding.
- cross-face: `chatInput.sendNow` is classified nowhere in
  `scripts/i18n-cross-face-contract.json` at `HEAD` either. This block added its
  own new keys to the contract (`themes.{rice,fluor,indigo}.{label,description}`
  and `settings.themeSwatchSample`), so the only remaining error is the
  pre-existing one.

No new translation key is missing in either locale, and no placeholder changed.

## Full suite

```text
cd ui && GOTOOLCHAIN=go1.26.4 pnpm test
# Test Files  9 failed | 79 passed (88)
# Tests       45 failed | 666 passed (711)
# [ELIFECYCLE] Test failed.
```

Block 0 established that this host cannot run the suite green for reasons
unrelated to the overhaul: Node ≥ 25 exposes a global `localStorage` that
shadows the per-file happy-dom window, producing
`TypeError: localStorage.clear is not a function` (or `Cannot read properties of
undefined (reading 'clear')`) in exactly these 45 tests across 9 files — the
same count as the untouched parent commit (open row `UI-VITEST-NODE25` in
`docs/TODO.md`). No file this block touches appears in the failure list; the
four suites listed under Gates pass in isolation and in the full run.

This is reported as a known pre-existing red, not as a pass.

## Not run, and why

- `just ci`, `just ui-ci`, `just wiring-ci`, `just kernel-ci`: they need an
  independent Linux-filesystem checkout; unavailable on this macOS host. The
  individual pieces they would run that are runnable here are listed above
  (`typecheck`, `build`, `i18n-check` parts, tests).
- Studio (`just studio`): Block 8, untouched here; the submodule is not part of
  this worktree.
- Real-provider streaming states: no provider key in this lane; they arrive with
  Block 3's frozen-env provider.
