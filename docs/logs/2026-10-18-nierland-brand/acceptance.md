# Acceptance — how a human can tell it worked

Studio is running at http://127.0.0.1:3090. The skin is read at boot, so
**hard-refresh the page** (Ctrl+Shift+R) before checking; a plain refresh can
serve the cached shell.

1. **Sidebar wordmark** — the top-left lockup reads `NIERLAND` in wide-tracked
   capitals, preceded by a small teal diamond (the fluorite gem). No `V`
   letter tile, no italic serif wordmark.
2. **Tab title** — the browser tab reads `Nierland`, not `Vivy Studio`.
3. **Tab icon** — the tab shows the same teal diamond, legible at 16px instead
   of the previous detailed 50x50 mark.
4. **Dark theme** — switch the shell to dark: the gem becomes light teal with a
   dark facet (the old tile was near-white text on light teal, ~1.6:1).
5. **Welcome / onboarding dialog** — its title reads `Nierland`; the body keeps
   its wording with the name now consistent.

Anything that still reads unlike the brand is a defect against this change:
report the string and where it appears.

## What to compare against

- Decision sheet (the options that were considered):
  `data/studio-home/brand-preview/nierland-brand-options.png`
- Rendered result used for the pixel check (light / dark):
  `data/studio-home/brand-preview/lockup-light.png`,
  `data/studio-home/brand-preview/lockup-dark.png`

Both live in Studio's own scratch (`data/studio-home/`), not in version control.

## Tuning knobs

One-line changes in `studio/dsh-vivy-studio/theme.css` if the result is too
loud or too quiet: mark size (`width`/`height` on the `::before` rule, 19px),
tracking (`letter-spacing`, 0.22em), wordmark size (`font-size`, 13.5px).
