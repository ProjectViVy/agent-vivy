# Block 8 acceptance (human view)

Boot the Studio the usual way for your host (Windows: `launch-vivy-studio.ps1`
→ `http://127.0.0.1:3090`; the skin is read at server boot, so restart after
syncing the profile copy). Hard-refresh the browser.

1. **Shell, light.** Sidebar shows the fluorite gem + `NIERLAND` wordmark; the
   welcome hero shows `寻找真心之旅` with the `STUDIO` badge. Nothing looks
   different from before this change — that is the point: the fold is
   render-identical in light.
2. **Shell, dark.** Switch appearance to dark. Surfaces stay warm ink
   (`#161310` family), the wordmark gem flips to the light fluorite variant,
   the running-turn indicator still reads `Seeking.....` in shimmering
   fluorite. No grey/blue DSH default surfaces anywhere.
3. **Plugin page, dark.** Open `插件`. Toggles and the `添加插件` button are
   fluorite (muted in dark), the `实验性` badges are fluorite wash — not the
   old grey-wash hub palette and not DSH blue.
4. **Install modal, dark.** `添加插件` opens the install dialog: title renders
   as the italic serif product name, the trust notice keeps its amber wash,
   and the disabled `安装` button is muted **fluorite**, not grey/white.
   (Before the cascade fix this button rendered DSH near-white in dark — the
   easiest regression tell.)
5. **Token coverage.** In `studio/dsh-vivy-studio`, run
   `node check-tokens.mjs` (needs the DSH reference checkout three levels up):
   it prints `ok: 117 official alias/specific tokens covered` and exits 0.
6. **Structure.** Open `studio/dsh-vivy-studio/theme.css`: the
   `--dsw-alias-*` / `--dsw-specific-*` names appear exactly once (one mapping
   block), and the dark theme block contains only `--vivy-*` overrides.
   `grep -c -- '--dsw-' theme.css` counts references, not duplicate
   declarations.
7. **Hub build.** In `studio/dsh-plugin-hub`: `npm ci && npm run build &&
   npm test` — build clean, 56/56 tests pass. The marketplace styles no longer
   contain the ad-banner / update-badge rules (sealed-fork leftovers).

Screenshots of old vs new in both themes are in `shots/`
(`old-*` = pre-change server, `new-*` = this branch).
