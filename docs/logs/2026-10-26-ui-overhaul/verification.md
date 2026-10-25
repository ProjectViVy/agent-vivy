# Verification — UI overhaul Block 9 (final sweep)

Host: macOS (Apple Silicon), worktree
`/Users/mastwet-macmini/pjvv/agent-vivy/.worktrees/ui-overhaul/agent-vivy`,
branch `feat/ui-overhaul`. Node v24.18.0, pnpm 12.10.1, happy-dom 20.14.0.

## Commands run

### Typecheck (clean)

```text
cd ui && npx tsc --noEmit
# no output → clean
```

Run after each of: the `switchModelTitle` removal, the `separator.tsx`
remove-then-revert, and the three-dependency removal. Clean every time. This is
the proof that none of the removed packages were imported by any TS source
(including the generated plugin modules under `ui/src/generated/ui`).

### Token ratchet + i18n (green)

```text
cd ui && npx vitest run src/theme/tokens.test.ts src/i18n
# [token-ratchet] src/palette: 0 (baseline 0); 0 files (baseline 0)
# [token-ratchet] src/textSize: 0 (baseline 0); 0 files (baseline 0)
# [token-ratchet] src/hex: 0 (baseline 0); 0 files (baseline 0)
# [token-ratchet] plugins/palette: 0 (baseline 0); 0 files (baseline 0)
# [token-ratchet] plugins/textSize: 0 (baseline 0); 0 files (baseline 0)
# [token-ratchet] plugins/hex: 0 (baseline 0); 0 files (baseline 0)
# Test Files  4 passed (4)   Tests  52 passed (52)
```

The ratchet allowlist is empty in every category for both groups — the Block 9
acceptance condition. `src/i18n/completeness.test.ts` (en/zh parity) passes
after the `switchModelTitle` deletion, confirming the dictionaries stayed
structurally identical.

### Contrast audit (green)

```text
cd ui && node scripts/audit/contrast.mjs
# contrast: 7 families, 147 pairs, all >= AA
```

All seven theme families (default/dark/pink/miku/rice/fluor/indigo) meet WCAG AA
across 147 foreground/background pairs.

### Dependency removal + lockfile consistency

```text
cd ui
# removed @supabase/supabase-js, framer-motion, zod from package.json
cp pnpm-lock.yaml /tmp/ui-lock-before.yaml
pnpm install --lockfile-only --offline      # Done in 48ms, EXIT 0
diff /tmp/ui-lock-before.yaml pnpm-lock.yaml # 119 lines removed, 0 added
pnpm install --frozen-lockfile --offline     # Done in 699ms, EXIT 0
```

The lockfile diff is deletion-only: the three importer entries plus
`@supabase/*` sub-packages, `framer-motion@11.18.2`, and `zod@3.25.76`.
`zod@4.4.3` (a transitive dependency of `@tanstack/router-plugin` and
`@tanstack/router-generator`) is correctly retained. `--frozen-lockfile`
succeeds, so `package.json`, `pnpm-lock.yaml`, and `node_modules` agree — CI's
frozen install will not break.

### Repo-wide unused-reference proof

```text
rg -l --fixed-strings "@supabase/supabase-js" . -g '!**/node_modules/**' …  # (none)
rg -l --fixed-strings "framer-motion"          . -g '!**/node_modules/**' …  # (none)
rg -l --fixed-strings "zod"                    . -g '!**/node_modules/**' …  # only studio/ libs (own deps)
```

## Pre-existing failure — NOT introduced by this work

The full `npx vitest run` reports **45 failed / 665 passed (710)**, 9 files.
Every failure is the same environmental defect:

```text
(node:…) ExperimentalWarning: localStorage is not available because
--localstorage-file was not provided.
TypeError: Cannot read properties of undefined (reading 'clear')
  ❯ window.localStorage.clear()
```

Node 24 exposes an experimental global `localStorage` that is disabled without
`--localstorage-file`; it shadows happy-dom's `window.localStorage`, which then
reads `undefined`. Classification evidence:

- The failing files are logic/store/DOM tests
  (`store`, `reveal-engine`, `session-list-view`, `ConversationSidebar`,
  `WelcomeWizard`, four settings cards), not CSS surfaces.
- My branch modified only one of them (`ConversationSidebar.test.tsx`); the
  other eight are untouched by `feat/ui-overhaul`.
- It reproduces identically on the **untouched `stable` root checkout**:
  `cd /Users/mastwet-macmini/pjvv/agent-vivy/ui && npx vitest run
  src/lib/reveal-engine.test.ts` → same `window.localStorage.clear()` failure.

This is tracked by `docs/TODO.md` §0.1 row `UI-VITEST-NODE25`, which already
records the macOS / Node v24.18.0 reproduction (45 tests, 9 files, untouched
root). It is test-infrastructure debt, out of scope for a CSS overhaul, and was
not introduced or worsened by any block. Fix direction (per that row): pin the
supported Node major, or rebind `globalThis.localStorage` from the happy-dom
window in a test setup file.

## Checks intentionally not run

- `pnpm build` (production `vite build`) — not run; the host has a known
  esbuild temp-file condition (`CI-VITE-ESBUILD-TEMP`, Windows) and the build is
  not the Block 9 acceptance gate. Typecheck + ratchet + contrast + frozen
  lockfile cover the changes made here (none of which touch bundling).
- `just studio` — the Studio submodule skin was verified in the Block 8 log; no
  Studio file changed in Block 9.
- Full `just ci` — separate platform/release gate; Block 9 changed only UI
  source, two dictionary lines, one comment, and the dependency set.
