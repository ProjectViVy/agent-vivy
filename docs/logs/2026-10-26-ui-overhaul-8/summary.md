# Block 8 — Studio shell skin alignment

## What changed

In the `studio/` submodule (branch `ui/skin-alignment`, off `ac55322`):

- `dsh-vivy-studio/theme.css` collapsed onto one semantic layer. Every colour
  decision now lives in a `--vivy-*` variable (light values in `:root`, dark
  values in the `body[data-ds-dark-theme]` override). The official
  `--dsw-alias-*` / `--dsw-specific-*` sheet is derived from that layer exactly
  once; the old file re-declared all ~90 aliases per theme (187 declarations),
  the new file declares them once (117 official tokens + constants).
- Token coverage closed: `check-tokens.mjs` was already red at `ac55322`
  (29 official tokens added upstream since the skin was written — diff
  markers, illustration ramp, shimmer/turn-trigger, switch/thumb, tooltip-key
  and friends). All 29 are now covered with fluorite-palette equivalents; the
  check reports `ok: 117 official alias/specific tokens covered`.
- Type scale adopted in the skin overrides: wordmark 13.5px → `--vivy-text-ui`
  (13px), slogan 34px → `--vivy-text-hero`, badge 11px → `--vivy-text-2xs`,
  dialog title 20px → `--vivy-text-page`, copy 14px → `--vivy-text-read`,
  turn clock 13px → `--vivy-text-ui`. The scale mirrors the kernel's
  2xs/meta/ui/read/title/page steps plus one display hero.
- Cascade bug fixed while folding: DSH's own sheet declares the aliases on
  `body[data-ds-dark-theme]` (specificity 0,1,1). The old skin won in dark only
  because it repeated every alias inside its dark block. A single `html, body`
  mapping block (0,0,1) loses that fight, so dark silently fell back to DSH
  defaults (verified: install button rendered DSH near-white in dark). The
  mapping block now also lists `body[data-ds-dark-theme]` — equal specificity,
  later document order, Vivy sheet wins in both themes.
- `dsh-plugin-hub/src/client/styles/tokens.module.css` remapped onto the same
  sheet names: dead token references (`--dsw-alias-state-danger-primary`,
  `--dsw-alias-state-warning-primary`, `--dsw-alias-input-border`,
  `--dsw-alias-label-disabled`) replaced with the real sheet names; the
  off-palette purple family collapsed onto brand (single-accent family), the
  deep-amber "warning" family onto "warn"; derived tints/borders now use
  `color-mix` on the resolved alias so both themes adapt without a second
  declaration; the dark block's grey-wash overrides were dropped so hub dark
  follows the Vivy dark palette (danger keeps its red in both themes).
- Dead CSS-module classes deleted in the hub: 21 classes / 28 selector
  occurrences across `CustomInstallView`, `Header`, `InstalledView`, `List`,
  `Modal` module stylesheets (ad banner, update badges, hub-update modal
  leftovers of the sealed fork). Selector-aware removal kept compound
  selectors intact.

## Scope notes / what was explicitly not done

- The plan estimated "40 unreferenced CSS-module classes". The verifiable dead
  set is 21 classes / 28 selectors, all in the hub. The four sidebar candidates
  (`sessionRow`, `itemLabel`, `loadingRow`) exist only inside comments — no
  rules to delete. The eleven `LogsView` badge/category classes are referenced
  dynamically (`view[`badge${e.level}`]`, `view[`cat${e.category}`]` in
  `LogsModal.tsx`) and were kept.
- `dsh-better-sidebar` is not installed in the Vivy Studio profile, so its
  styles are not part of the running shell; nothing was changed there.
- Kernel shadcn primitives stay on the compat shim (Block 9).
- No push of either the submodule branch or the host branch.
- Hub `lib/` + `client/` build outputs remain gitignored in the submodule; a
  checkout that serves the hub must run `npm ci && npm run build` in
  `studio/dsh-plugin-hub` (the root tree carries untracked build outputs today).

## Commits

- Submodule `vivy-studio` branch `ui/skin-alignment`: skin + hub style commit.
- Host branch `feat/ui-overhaul`: gitlink bump + this log (separate commit).
