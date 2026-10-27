# Summary — UI overhaul Block 0: baseline evidence and token ratchet

Block 0 of the staged UI overhaul (`feat/ui-overhaul`). It changes no product
code. It makes every later claim checkable by freezing today's visual baseline
and turning the "no stock palette class / no arbitrary text size" rule into a
test that can only shrink.

## What changed

1. **Token ratchet** — `ui/src/theme/tokens.test.ts`. Scans authored `.ts/.tsx`
   under `ui/src` and under any `plugins/*/ui` module, skips
   `node_modules`/`dist`/`generated`/`test-results`/`coverage` and `*.test.*`,
   and counts three categories: stock Tailwind palette classes
   (`bg|text|border|ring|from|to|via|fill|stroke|divide|outline|shadow|accent|caret|decoration|placeholder` ×
   a stock family with a numeric shade, or `white`/`black`), arbitrary text
   sizes (`text-[Npx|rem|em]`), and hex literals. A per-group baseline records
   both the count and the set of files that violate. A block may reduce either;
   any new violation — a larger count, or a file that was not already in the
   baseline — fails. A baseline file that
   stopped violating must be removed from the baseline in the same change, so
   the allowlist cannot quietly widen.

   Measured today (printed by the test):

   | Group | Palette | `text-[…]` | Hex |
   | --- | --- | --- | --- |
   | `ui/src` | 159 in 31 files | 85 in 26 files | 21 in 2 files |
   | `plugins/*/ui` | 29 in 7 files | 45 in 8 files | 0 |

   This reconciles with the plan's exploratory 158 / 31 for `ui/src`. The
   plan's matcher counted shades only; this one also counts `white`/`black`,
   which is the escape hatch that matters — `bg-white` and `bg-black/80` are
   exactly the hardcoded surfaces that break under a dark theme. The five extra
   files are `MessageBubble.tsx` and the `alert-dialog`, `dialog`, `drawer` and
   `sheet` primitives (overlay scrims).

2. **Screenshot audit** — `ui/scripts/audit/screenshots.mjs`, exposed as
   `pnpm run audit`. Playwright, 10 surfaces × 4 persisted themes × 3 widths
   (390/768/1280). It never starts servers: it expects the split pair to be
   running and fails loudly instead of inventing a private server, so the
   evidence is always of an app a human can open. Bundled Chromium first, then
   `channel: 'chrome'` when the Playwright browser cache is absent (true on this
   host). `locale zh-CN`, `reducedMotion: reduce`, welcome overlay suppressed
   through the profile's own `localStorage` key, one `manifest.json` recording
   `baseUrl`, every shot and every failure; non-zero exit if any surface throws.

3. **Housekeeping** — `ui/package.json` gained the `audit` script;
   `ui/.gitignore` ignores `.audit/`.

## Scope and what was explicitly not done

- No stylesheet, component, route, store or plugin source was edited. Block 1
  owns the first visual change.
- Baseline shots cover the empty/initial state of each surface plus the nav
  sheet and welcome wizard. Run states (streaming, approval, queue, tool
  failure) need a provider; they are captured from Block 3 onward with the
  frozen-env provider described in `testing-vivy-ui`.
- `chart.tsx` carries five hex literals (part of the 21) and is one of the 13
  zero-import primitives slated for deletion in Block 2, so those entries
  disappear there; `use-theme.ts` carries the other 16 and loses them in Block 1.
- `pnpm test` cannot be green on this host for reasons unrelated to this
  change; see `verification.md` and `UI-VITEST-NODE25` in `docs/TODO.md`.
