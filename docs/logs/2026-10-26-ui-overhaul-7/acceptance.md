# UI overhaul Block 7 — acceptance (human view)

Start the split pair (`just dev`, or the isolated evidence pair) and open
`http://127.0.0.1:3015` (or `:3016` for the evidence lane).

1. **Plugin surfaces follow the theme.** Open Settings → Appearance (or the
   theme picker) and switch between Vivy light, Vivy dark, Diva, Miku and the
   new families. Then visit `/masks`. The four identity chips (Just me /
   Programmer / Researcher / Writer) re-hue with each family — e.g. the Writer
   chip is violet in Vivy light, teal in Miku, magenta in Diva — because they
   are now `brand/ok/info/warn` semantic tints. Before this block they were
   hardcoded `violet-100`/`emerald-100`/`blue-100`/`amber-100` and looked the
   same in every family (only light/dark varied).
2. **Status colours are consistent with the kernel.** On `/masks`, `/memory`,
   `/persona`, `/workflows`, error banners and retry/warning notices use the
   same danger/warn soft surfaces as the kernel pages (no stray amber/emerald
   that ignores the theme). In dark families the soft tints stay readable
   instead of the old `dark:`-only overrides.
3. **Type scale is uniform.** The workflow Runs/Editor panes and node cards no
   longer show sub-11px micro-text; the smallest tier is the shared `text-2xs`
   (11px), so mono ids, badges and event rows match the kernel's density.
   Headings on `/notebook`, `/masks`, `/evolution` use the shared
   `text-title`/`text-page` scale.
4. **No layout regressions.** Resize to 390/768/1280 on each plugin route; the
   page never scrolls horizontally (document overflow stays 0). The persona
   pill strip and evolution/workflow tab bars scroll horizontally by design at
   narrow widths.
5. **Ratchet is at zero.** `cd ui && pnpm vitest run src/theme/tokens.test.ts`
   prints `plugins/palette: 0`, `plugins/textSize: 0`, `plugins/hex: 0`. Any
   future hardcoded hue or arbitrary text size under `ui/src` or
   `plugins/**/ui/**` now fails the gate instead of silently shipping.

Known remaining (tracked, not regressions): the `/workflows` editor toolbar
clips at ≤390px (`UI-WORKFLOW-MOBILE-CLIP`), and the pre-existing
`STORE-RUN-FAILED-COPY` test red.
