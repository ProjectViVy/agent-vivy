# Summary — UI overhaul capstone (Blocks 0–9)

This is the roll-up record for the staged UI overhaul on `feat/ui-overhaul`.
Each block has its own directory (`docs/logs/2026-10-25-ui-overhaul-0` …
`docs/logs/2026-10-26-ui-overhaul-8`); this directory closes the track with the
end-to-end before/after and the Block 9 final sweep.

## Goal

One professional, usable, attractive UI that inherits and extends the original
visual style, eliminates the theme chaos / dead code / clipping / inconsistent
colour that had accumulated, and respects the Vivy plugin development rules.
The user chose to keep and deepen the Vivy / Diva / Miku themes and add new
families, to cover kernel + plugins + the Studio shell, and to delete dead
controls and pseudo-operations rather than keep them.

## End-to-end result

The overhaul replaced a scattered, hardcoded colour surface with a single
authored token contract and migrated every surface onto it.

| Measure | Before (Block 0 baseline) | After (Block 9) |
| --- | --- | --- |
| Stock palette classes, `ui/src` | 159 in 31 files | 0 |
| Arbitrary `text-[…]` sizes, `ui/src` | 85 in 26 files | 0 |
| Hex literals, `ui/src` | 21 in 2 files | 0 |
| Stock palette classes, `plugins/*/ui` | 29 in 7 files | 0 |
| Arbitrary `text-[…]` sizes, `plugins/*/ui` | 45 in 8 files | 0 |
| Hex literals, `plugins/*/ui` | 0 | 0 |
| Theme families with a contrast guarantee | none asserted | 7 families, 147 pairs, all ≥ AA |
| Authored colour sources | many (inline styles, per-component literals) | one (`ui/src/styles.css`) |

339 raw colour/size/hex literals across the kernel and plugin UI modules went to
zero, and the token ratchet (`ui/src/theme/tokens.test.ts`) now fails any new
literal, so the contract cannot silently regress.

## What each block delivered

- **Block 0** — froze the visual baseline (120 screenshots) and added the token
  ratchet. No product code.
- **Block 1** — rebuilt themes on one authored token source: a neutral ramp per
  appearance plus per-family delta blocks, seven families (Vivy light/dark,
  Diva, Miku, Rice & Vermilion, Graphite & Fluor, Night Indigo), preview tiles
  rendering real tokens.
- **Block 2** — dropped 13 zero-import shadcn primitives (~2,444 lines) and 22
  unused dependencies; shared one session row.
- **Block 3** — chat transcript renders without clipping across widths.
- **Block 4** — composer split into single-owner parts.
- **Block 5** — wizard split and navigation aligned to tokens.
- **Block 6** — remaining kernel pages swept onto the token contract.
- **Block 7** — plugin UI modules migrated onto the semantic token contract.
- **Block 8** — Studio shell skin folded onto one `--vivy-*` semantic layer
  (submodule `ui/skin-alignment` at `7664754`, host gitlink bump `ba55a805`).
- **Block 9** — final sweep (this block, see below) and the capstone record.

## Block 9 — final sweep

Small, verified cleanups; no visual change.

1. **Dead i18n key removed.** `modelSwitcher.switchModelTitle` had no literal
   reference and no dynamic `modelSwitcher.*` reach; deleted from both `zh.ts`
   and `en.ts` (parity preserved). Every other `modelSwitcher` key is live. An
   exhaustive unused-key purge was deliberately not attempted: the codebase
   builds many keys through dynamic prefixes (`t(\`themes.${id}.label\`)`,
   `t(\`approvals.status.${status}\`)`, …), so a literal grep cannot prove those
   dead, and the plan does not require it.
2. **Three unused dependencies removed.** `@supabase/supabase-js`,
   `framer-motion`, and `zod` had zero imports repo-wide (the only `zod` hits are
   Studio submodule libraries and `@tanstack/router-plugin`'s own transitive
   `zod@4.4.3`, which is untouched). These were shadcn-template leftovers, not
   deps the overhaul orphaned. `ui/pnpm-lock.yaml` was regenerated offline and
   `pnpm install --frozen-lockfile` passes.
3. **Stale comment corrected.** `ui/src/styles.css` described the shadcn alias
   block as a pending deletion ("blocks 2-6 … delete entries here"). The aliases
   are a permanent *derived* boundary: the vendored `components/ui` primitives
   and the generated plugin modules name their tokens `background` /
   `muted-foreground` / `primary`, and those names resolve through the semantic
   set, so there is no second source of truth. The comment now says so.

### Dead CSS / primitive check

No dead kernel CSS remains: every custom class in `styles.css`
(`prose-inherit`, `streaming-caret`, `row-sweep`,
`vivy-presentation-diagnostic*`, `welcome-float`) is referenced, and no
`components/ui` primitive is orphaned. One candidate, `separator.tsx`, has no
*host* importer but is consumed by the generated plugin modules
(`vivy-memory`, `vivy-workflow`); `tsc` proves the dependency, so it stays. The
Block 8 hub dead-class removal (21 classes) is recorded in the Block 8 log.

## Scope discipline / what was not done

- No bulk rename of the ~444 shadcn-name call sites onto semantic utilities.
  The aliases resolve identically, so this is cosmetic purity, not correctness;
  forcing it in the final block would be high-churn and zero-visual-change.
- The pre-existing UI test-environment failure was not fixed here (see
  verification). It is infrastructure debt tracked by `UI-VITEST-NODE25`,
  reproduces on an untouched `stable` checkout, and is unrelated to the CSS work.
- No hand edits of `ui/src/generated/**`.
- No pushes; the owner's Studio instance at `:3090` was never restarted.
