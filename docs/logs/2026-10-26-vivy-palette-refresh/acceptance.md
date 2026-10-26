# Acceptance — Vivy palette refresh

What a human can confirm by looking at the running app, without reading code.
Start the pair (`just run` + `cd ui && pnpm dev`), open
`http://127.0.0.1:3015`, and hard-reload once (`Cmd+Shift+R`) so the new
`index.html` pre-paint colours are picked up.

## 1. The chrome is ice-blue again, not grey-teal

With **Vivy Light** selected, the chat canvas is a pale cyan, the sidebar a
slightly deeper cyan, and the selected session row clearly cyan. Sampled with a
colour picker at 1280 wide:

| surface | expected |
| --- | --- |
| canvas, empty area right of the transcript | `#ebfaff` |
| sidebar background | `#e2f4f9` |
| selected session row | `#c5eefc` |

The old overhaul values (`#f0f8fd`, `#e7f2f7`, `#d2ebf7`) mean the CSS is cached.

## 2. Blue surfaces carry white text, not black

Send one message. The user bubble is azure (`#057bac` at the sampled pixel)
with near-white text — no black or near-black ink on any blue fill. The same
holds for the send/steer buttons, the **V** wordmark, the theme badge on
Settings → Theme, the selected platform card in the welcome wizard, and the
selected platform chips in the channel wizard. Icons inside the bubble are
white at 85% opacity, still visible.

The blue is one step deeper than the accent marks (which stay the original
bright azure) — that is what makes white text hold 4.6:1.

## 3. Tinted notices are readable

Open Settings (there is a read-only notice), the MCP page, and the trajectory
ledger. Every pale-green / pale-amber / pale-red block now draws its own deep
tone as text — dark green on pale green, dark amber on pale amber, dark red on
pale red. Nothing is white-on-pale or black-on-dark any more. The connection
chip next to the header title and the mask status labels behave the same way.

## 4. Dark is the same family, deeper not muddier

Switch Appearance to **Dark**. Panels are deep sea-blue (`#071519` canvas), the
accent marks are the original bright azure, and the user bubble is the same
azure as light with white text. Toggling back and forth changes lightness, not
hue family, and the bubble never turns into bright cyan with black text.

## 5. Only Vivy moved

Settings → Theme. **Vivy Light** and **Vivy Dark** look refreshed; **Diva**,
**Miku**, **Rice & Vermilion**, **Graphite & Fluor** and **Night Indigo** keep
their accents and their inks. Diva stays rose with white ink; Miku, Fluor and
Indigo keep a bright accent with dark ink, which is deliberate for a light
accent on a dark theme and audits at 9:1 or better.

## 6. The gates agree

From `ui/`:

```
node scripts/audit/contrast.mjs
node scripts/audit/screenshots.mjs --base-url http://127.0.0.1:3015 --widths 1280
```

The first prints `contrast: 7 families, 182 pairs, all >= AA`. The second
photographs every surface in every family, measures the **rendered** text
contrast, and prints `0 ink pairs below AA` (4711 text nodes across 77
renders on this machine). Any future light-on-light or dark-on-dark pairing
fails the audit and the unit test `ui/src/theme/tokens.test.ts`.

## Known limits

- The bubble ink is near-white, so the fill had to drop to `#057bac`; a
  `#028fc9` bubble with white text is 3.6:1 and would fail AA. Raising it back
  is a one-block change if the owner accepts that trade.
- The bright `#028fc9` accent remains for marks, icons, rings and dots; it is
  not used as small text (text uses `--brand-ink`).
- No layout, typography, component or Studio-shell change is part of this
  delivery.
