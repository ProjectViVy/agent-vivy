# Verification — Vivy palette refresh

Lane: worktree `agent-vivy/.worktrees/ui-overhaul/agent-vivy`, branch
`feat/ui-overhaul`. Backend reused from the root checkout
(`.workspace/dev/vivy-backend`, dev home `.workspace/dev/home/`, `:8787`);
worktree Vite served the UI under test at `http://127.0.0.1:3016`.

## Gates

| command (from `ui/`) | result |
| --- | --- |
| `node scripts/audit/contrast.mjs` | `7 families, 168 pairs, all >= AA` |
| `npx tsc --noEmit` | clean |
| `npx vitest run` | `45 failed | 665 passed (710)` in 9 files — unchanged from `22e91ace`; every failure is the pre-existing `window.localStorage is undefined` happy-dom defect tracked as `UI-VITEST-NODE25` |
| `npx vitest run src/theme src/i18n` | `59 passed (59)` |
| `GOTOOLCHAIN=go1.26.4 node scripts/stage-ui-assembly.mjs` | restaged `ui/src/generated/**` (untracked; `assembly.ts` byte-identical) |

Default light now audits: `brand on canvas=3.40`, `brand on raised=3.64`,
`brand on brand-soft=3.21` (graphic, ≥3), `brand-fg on brand=5.33`,
`brand-ink on canvas=4.89`, `brand-ink on raised=5.23`,
`brand-ink on brand-soft=4.62`. Every other pair in every family is unchanged
from the Block 9 run.

## Toolchain note

The system `go` on this host is 1.27.1 while `go.mod` pins `1.26.4`. With
`GOTOOLCHAIN=auto`, Go only upgrades, so `pnpm typecheck` / `predev` /
`pretest` die in `stage-ui-assembly` inside `grpc@v1.64.1`
(`undefined: http2.TrailerPrefix`). Setting `GOTOOLCHAIN=go1.26.4` (cached at
`~/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.darwin-arm64`) fixes it and
is how the restage above was run. Boarded as `DEV-GO-TOOLCHAIN-PIN`.

## Visual evidence

Playwright (installed Chrome), 1280×900, `locale zh-CN`, `reducedMotion reduce`,
`vivy.theme` / `vivy.appearance` / welcome key seeded before load. Pixel samples
decoded from the PNGs:

| surface (x,y) | original `stable` | overhaul before | after |
| --- | --- | --- | --- |
| canvas (700,620) | `#eef9fc` | `#f0f8fd` | `#ebfaff` |
| sidebar (150,600) | `#ddf3fa` | `#e7f2f7` | `#e2f4f9` |
| selected row (150,344) | `#aee9fc` | `#d2ebf7` | `#c5eefc` |
| user bubble (870,120) | `#eef9fc` (empty baseline) | `#026a8b` | `#018fc9` |

Computed-style probes on `:3016` confirm the utilities resolve:
`text-brand-ink` → `oklch(0.528 0.117 238)`, `text-brand-fg` →
`oklch(0.16 0.025 236)`, `bg-brand` → `oklch(0.615 0.132 236)`.

Screenshots: `shots/default-1280-chat.png`, `shots/default-1280-settings.png`,
`shots/dark-1280-chat.png`, `shots/dark-1280-settings.png`. The "empty chat"
surfaces could not be captured separately: the app restores the last opened
session on load, so both chat shots show that transcript.

## Not run

- Full `just ci` / `just ui-ci`: not applicable to a token-only UI change on
  this host, and the known `CI-VITE-ESBUILD-TEMP` / WSL gate conditions from the
  board still apply.
- Studio shell visuals: no Studio file changed in this delivery.
