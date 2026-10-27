# Verification — Vivy palette refresh

Lane: worktree `agent-vivy/.worktrees/ui-overhaul/agent-vivy`, branch
`feat/ui-overhaul`. Backend reused from the root checkout
(`.workspace/dev/vivy-backend`, dev home `.workspace/dev/home/`, `:8787`);
worktree Vite served the UI under test at `http://127.0.0.1:3016`.

## Gates

| command (from `ui/`) | result |
| --- | --- |
| `node scripts/audit/contrast.mjs` | `7 families, 182 pairs, all >= AA` |
| `npx vitest run src/theme src/i18n` | `61 passed (61)` |
| `npx vitest run` | `45 failed | 667 passed (712)` in 9 files — the same 9 files as `22e91ace`, every failure the pre-existing `window.localStorage is undefined` happy-dom defect tracked as `UI-VITEST-NODE25`; the two added ink assertions pass |
| `GOTOOLCHAIN=go1.26.4 npx tsc --noEmit` | clean |
| `GOTOOLCHAIN=go1.26.4 node scripts/stage-ui-assembly.mjs` | restaged `ui/src/generated/**` (untracked; `assembly.ts` byte-identical) |

Vivy light now audits: `brand on canvas=3.40`, `brand on raised=3.64`,
`brand on brand-soft=3.21` (marks, ≥ 3), `brand-fill on canvas=4.44`,
`brand-fill on raised=4.75`, `brand-fg on brand-fill=4.62`,
`brand-ink on canvas=4.89 / raised=5.23 / brand-soft=4.62`.
Vivy dark: `brand on canvas=7.25`, `brand-fill on canvas=3.96`,
`brand-fg on brand-fill=4.63`, `brand-ink on canvas=7.25`.
`ok/warn/danger/info` on their soft tints: 5.91 / 6.82 / 6.30 / 6.11.

## Rendered ink sweep

`screenshots.mjs` now measures every visible text node on every audited surface
(effective background, translucent ancestors composited, canvas-resolved colour,
WCAG 1.4.3 normal text 4.5 / large text 3.0). Controls were checked first:
white-on-white reports 1.00 and `#057bac` + `#f8fdfe` reports 4.61, so the sweep
does detect failures.

| run | result |
| --- | --- |
| `node scripts/audit/screenshots.mjs --base-url http://127.0.0.1:3016 --widths 1280` (7 families × 11 surfaces) | 77 renders, **4711 text nodes, 0 pairs below AA** |
| `node scripts/audit/screenshots.mjs --base-url http://127.0.0.1:3016 --themes default,dark --widths 390` | 22 renders, 956 text nodes, 0 pairs below AA |
| same, `--themes default,dark --widths 1280` | 22 renders, 1346 text nodes, 0 pairs below AA |

## Visual evidence

Playwright (installed Chrome), 1280×900, `locale zh-CN`, `reducedMotion reduce`,
`vivy.theme` / `vivy.appearance` / welcome key seeded before load. Pixel samples
decoded from the PNGs:

| surface (x,y) | original `stable` | first cut of this refresh | final |
| --- | --- | --- | --- |
| canvas (700,620) | `#eef9fc` | `#ebfaff` | `#ebfaff` |
| sidebar (150,600) | `#ddf3fa` | `#e2f4f9` | `#e2f4f9` |
| selected row (150,344) | `#aee9fc` | `#c5eefc` | `#c5eefc` |
| user bubble, dark (870,120) | `#eef9fc`¹ | `#018fc9` + dark ink | `#057bac` + `#f8fdfe` |

¹ the original Block 0 baseline is an empty chat; the stable bubble token was
`#008fca` with white text (3.6:1, below AA).

Computed-style probes on `:3016` after the fix: the user bubble is
`oklch(0.55 0.117 236)` with `oklch(0.99 0.005 220)` text (4.62), the `在线`
chip is `oklch(0.95 0.035 152)` with `oklch(0.46 0.11 152)` text, and
`text-brand-ink` / `bg-brand-fill` / `text-brand-fg` all resolve.

Screenshots: `shots/default-1280-chat.png`, `shots/default-1280-settings.png`,
`shots/dark-1280-chat.png`, `shots/dark-1280-settings.png`, taken from the
all-family audit run above.

## Toolchain note

The system `go` on this host is 1.27.1 while `go.mod` pins `1.26.4`. With
`GOTOOLCHAIN=auto`, Go only upgrades, so `pnpm typecheck` / `predev` /
`pretest` die in `stage-ui-assembly` inside `grpc@v1.64.1`
(`undefined: http2.TrailerPrefix`). Setting `GOTOOLCHAIN=go1.26.4` (cached at
`~/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.darwin-arm64`) fixes it and
is how the restage above was run. Boarded as `DEV-GO-TOOLCHAIN-PIN`.

## Not run

- Full `just ci` / `just ui-ci`: not applicable to a token/ink change on this
  host, and the known `CI-VITE-ESBUILD-TEMP` / WSL gate conditions from the
  board still apply.
- Studio shell visuals: no Studio file changed in this delivery.
- The welcome wizard's channel/platform chips were swept in the audit
  (`welcome` surface) but not clicked through to the channel modal.
