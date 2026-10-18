# Nierland brand lockup and tab identity

Date: 2026-10-18
Scope: Studio shell skin only (`studio/dsh-vivy-studio`, submodule
`ProjectViVy/vivy-studio`). No kernel, Journal, policy, or RPC change.

## What changed

The shell's product identity was rebuilt around the name the wordmark already
used (`Nierland`) instead of the leftover `Vivy Studio` / `V` pairing.

- **Mark**: the 22x22 letter tile showing `V` was replaced by the fluorite gem
  (a two-tone diamond: `#2aa8a4` body, `#fbf7f0` facet; dark theme `#5fd4cf` /
  `#1a1613`). The tile contradicted a wordmark that read `Nierland`, and its
  dark-mode pairing (light teal fill, near-white glyph) sat near 1.6:1 contrast.
- **Wordmark**: `Nierland` in a 17px italic serif became `NIERLAND` on the sans
  stack at 13.5px / weight 600 / 0.22em tracking, with the trailing tracking
  compensated. The old declaration used `--vivy-display`, which resolves to
  Times New Roman on a machine without Songti SC / STSong / Source Han Serif SC
  and produced a synthesised oblique.
- **Tab title**: `Vivy Studio` became `Nierland`.
- **Tab icon**: the DSH default 50x50 mark became the same fluorite gem as an
  inline SVG data URI (the default was illegible at 16px).
- **Single source of truth**: `index.js` holds the name once as `PRODUCT`;
  `theme.css` carries the `__VIVY_PRODUCT__` token that `index.js` substitutes
  at boot. The wordmark, the dialog title, and the two name occurrences in the
  dialog copy all consume that token; the injected client script receives the
  same value as `window.__VIVY_PRODUCT__`, so `brand.js` no longer hardcodes it.
- **Display stack**: `--vivy-display` now leads with Georgia / Palatino
  Linotype and includes Noto Serif SC, so Latin text gets real italics and CJK
  text a real serif instead of falling through to Times New Roman.
- README records the brand name, the token, and what still reads unlike it.

## Explicitly not done

- No kernel, `vivy.exe`, UI (`ui/`), lifecycle CLI, or product-contract change.
- The hero slogan (`寻找真心之旅`), the `STUDIO` preview badge, and the
  `Seeking.....` turn status were left as they were.
- The dialog copy wording was not rewritten; only the product-name occurrences
  in it were tokenised, so the text no longer contradicts the title.
- No new tests were added: this is a presentation-only change to a skin bundle
  that no Go/UI test reads. Verification is the live-server assertion set and
  the pixel check recorded in `verification.md`.

## Delivery

- `vivy-studio` submodule: `ac55322` on `main`, pushed (`3145241..ac55322`).
- Host repo: gitlink bump for `studio` in the same change as this record.
