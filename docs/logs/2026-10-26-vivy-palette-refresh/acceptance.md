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

Any of those reading as grey or teal (`#f0f8fd`, `#e7f2f7`, `#d2ebf7` are the
old overhaul values) means the old CSS is still cached.

## 2. Bubbles are the original bright azure

Send one message. The user bubble is the identity azure (`#018fc9` at the
sampled pixel, token `#028fc9`), not the previous deep teal slab (`#026a8b`).
The text inside it is dark navy, not white — white on this brightness is
3.6:1 and would fail AA; the dark ink is 5.33:1.

Assistant replies stay on the neutral raised surface with normal body ink.

## 3. Brand-coloured text is still readable

Anything written in the accent at small size — command hints in the composer,
model names in the switcher, active nav labels, chips, badges, permission-menu
icons and labels — is a deeper azure (`#0073a6`-ish) rather than the bright
fill. Zoom to 100% and read a chip label and an active sidebar row: both must
be as legible as ordinary body text. Bright `#028fc9` never appears as small
text; it appears as fills, bubbles, rings and icons.

## 4. Dark is the same family, deeper not muddier

Switch Appearance to **Dark**. Panels are deep sea-blue (`#071519` canvas),
the accent is the original bright dark-mode azure, and the pale text keeps the
same freshness. Toggling back and forth must not change hue family — only
lightness.

## 5. Only Vivy moved

Settings → Theme. **Vivy Light** and **Vivy Dark** look refreshed; **Diva**,
**Miku**, **Rice & Vermilion**, **Graphite & Fluor** and **Night Indigo** look
exactly as they did after the overhaul — their accents are their own and their
brand text already carried 4.5:1. Switching among them still recolours the
whole shell immediately and survives a reload.

## 6. The gate agrees

From `ui/`:

```
node scripts/audit/contrast.mjs
```

prints `contrast: 7 families, 168 pairs, all >= AA`. The Vivy light rows are
`brand on canvas=3.40`, `brand on raised=3.64`, `brand on brand-soft=3.21`
(graphic, ≥ 3), `brand-fg on brand=5.33`, and
`brand-ink on canvas/raised/brand-soft = 4.89 / 5.23 / 4.62`.

## Known limits

- The bubble ink is dark navy instead of white. White on the original accent is
  below AA, so this is the price of restoring the original brightness.
- The light accent cannot serve as both fill and small text at 4.5:1; `--brand`
  (fill) and `--brand-ink` (text) are the two roles. Reverting to a single token
  means going back to the deeper accent — a one-block change if preferred.
- No layout, typography, component or Studio-shell change is part of this
  delivery; only colour tokens, the four pre-paint literals, the audit pairs and
  the `text-brand` → `text-brand-ink` call sites moved.
