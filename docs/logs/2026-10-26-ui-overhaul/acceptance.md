# Acceptance — UI overhaul (Blocks 0–9)

How a human confirms the overhaul worked, from the product/user view.

## Whole-track acceptance

1. **Start the split pair** and open the app:

   ```text
   just dev          # or: just run  +  (cd ui && pnpm dev)
   # → http://127.0.0.1:3015
   ```

2. **Theme families render correctly.** Open Settings → Appearance. Seven
   families are offered — Vivy light, Vivy dark, Diva, Miku, Rice & Vermilion,
   Graphite & Fluor, Night Indigo. Each preview tile shows the *real* tokens
   (not a hand-drawn gradient), and selecting one repaints the whole app
   (sidebar, chat, composer, dialogs) consistently. The original Vivy / Diva /
   Miku identities are recognisable and deepened, not replaced.

3. **No clipping or overflow.** The chat transcript, composer, navigation,
   wizard, and every kernel page render without cut-off text or broken layout at
   390 / 768 / 1280 px. Long titles, rich markdown, tool/reasoning rows, approval
   cards, and queued/pasted composer input all stay legible.

4. **Dark appearance is correct everywhere.** Surfaces, text, lines, and status
   colours all read from the token ramp; there are no leftover hardcoded
   `bg-white` / `bg-black` panels that break under dark.

5. **Contrast is accessible.** `cd ui && node scripts/audit/contrast.mjs` prints
   `contrast: 7 families, 147 pairs, all >= AA`.

6. **The contract cannot silently regress.** `cd ui && npx vitest run
   src/theme/tokens.test.ts` prints `0` for all six ratchet categories
   (src/plugins × palette/textSize/hex). Adding any stock palette class,
   arbitrary `text-[Npx]`, or hex literal to authored source fails the test.

7. **Studio shell matches.** With the Studio skin installed (Block 8), the shell
   at `http://127.0.0.1:3090` uses the same one-layer `--vivy-*` semantics; light
   and dark both resolve (the dark primary button is teal `#5fd4cf`, not grey).

## Block 9 acceptance (final sweep)

- `cd ui && npx tsc --noEmit` → clean (proves the removed deps and the deleted
  i18n key were unreferenced).
- `cd ui && pnpm install --frozen-lockfile` → succeeds (package.json ↔ lockfile
  ↔ node_modules agree after removing `@supabase/supabase-js`, `framer-motion`,
  `zod`).
- `cd ui && npx vitest run src/i18n` → green (en/zh parity holds after the
  `switchModelTitle` deletion).
- Ratchet allowlist empty (item 6 above).
- No `ui/src/generated/**` path appears in the branch diff:
  `git diff --name-only stable...HEAD | rg 'ui/src/generated'` → empty.
- Every block has a log directory: `docs/logs/2026-10-25-ui-overhaul-0` …
  `docs/logs/2026-10-26-ui-overhaul-8`, plus this capstone
  `docs/logs/2026-10-26-ui-overhaul/`.

## Known limitation (pre-existing, not from this work)

`npx vitest run` (full suite) shows 45 failures in 9 files, all
`window.localStorage.clear()` on Node 24 without `--localstorage-file`. This
reproduces on an untouched `stable` checkout and is tracked by `docs/TODO.md`
§0.1 `UI-VITEST-NODE25`. It does not affect the running app or any visual
acceptance above; it is a test-runner environment defect. To see the suite green,
run under a Node major whose experimental `localStorage` is absent or provide a
happy-dom `localStorage` rebind in test setup.
