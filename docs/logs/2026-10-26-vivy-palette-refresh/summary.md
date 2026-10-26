# Vivy palette refresh — the original azure, back under AA

Owner feedback on the Block 0–9 overhaul: "配色方面还是太重，看看能不能回到原来的
清新配色中。vivy 的主配色。" The overhaul had moved the Vivy families to a
desaturated teal (`hue 228`, accent chroma 0.095), so the ice-blue chrome read as
grey-teal and the user-message bubbles were a dark slab. This delivery restores
the original identity without dropping the accessibility gate the overhaul
introduced.

## What changed

**Hue and tint (both Vivy families).** `--tint-h` 228 → 220 and the neutral tint
chroma 0.011 → 0.0165 (light) / 0.012 → 0.018 (dark). The original palette used
`hue 220` with roughly twice the tint chroma the overhaul kept, which is why the
selected row and sidebar had gone grey. At `0.975 / 0.0165 / 220` the canvas is
`#ebfaff` against the original `#eef9fc`; the selected row is `#c5eefc` against
`#aee9fc`.

**The accent is the original colour again.** Light `--brand` is
`oklch(0.615 0.132 236)` = `#028fc9`, i.e. the original `oklch(0.58 0.2 220)`
after the browser's gamut clamp (the original token was out of sRGB and rendered
`#008fca`). Dark `--brand` is the original dark primary, `#00aee4`.

**Why a fill/ink split.** The original accent measures 3.40 (canvas), 3.64
(raised) and 3.21 (`brand-soft`) — fine for a fill or an icon (WCAG 1.4.11, 3:1)
but not for 11–14 px text. Rather than darken the identity colour a second time,
the accent now has one more role:

- `--brand` — fills, bubbles, rings, icons (≥ 3:1)
- `--brand-fg` — ink drawn *on* the fill; in light this is now near-black
  (`oklch(0.16 0.025 236)`, 5.33:1) because the fill is bright again
- `--brand-ink` — brand-coloured text and glyphs on neutral surfaces
  (`oklch(0.528 0.117 238)` = `#0073a6`, 4.89 / 5.23 / 4.62 on canvas / raised /
  soft)

`--brand-ink` defaults to `var(--brand)` in the shared ramp, so the five other
families declare nothing and keep their current accents; only Vivy light
overrides it. 46 `text-brand` call sites in 28 files became `text-brand-ink`.

**Audit.** `ui/scripts/audit/contrast.mjs` now splits the brand pairs by role:
`brand on canvas / raised / brand-soft` at 3:1 (graphic) and
`brand-ink on canvas / raised / brand-soft` at 4.5 (text). 7 families, 168 pairs.

**Pre-paint.** `ui/index.html` carries the new `--bg-app` / `--fg` pairs
(`#ebfaff` / `#1e3137` light, `#071519` / `#e9f4f8` dark) and
`ui/src/theme/sources.test.ts` asserts that exact set.

## Scope

Vivy light and Vivy dark only. Diva, Miku, Rice, Fluor and Indigo keep their
accents and their ink is the fill (their accents already pass 4.5:1), so they get
the same code path without a colour change. No layout, spacing, typography,
component or backend change. The Studio shell skin is untouched — Block 8 owns
it.

## Measured (1280 light chat, same session)

| surface | original `stable` | overhaul before | after |
| --- | --- | --- | --- |
| canvas | `#eef9fc` | `#f0f8fd` | `#ebfaff` |
| sidebar | `#ddf3fa` | `#e7f2f7` | `#e2f4f9` |
| selected row | `#aee9fc` | `#d2ebf7` | `#c5eefc` |
| user bubble | `#eef9fc`¹ | `#026a8b` | `#018fc9` |

¹ the original Block 0 baseline is an empty chat; the stable bubble token was
`#008fca` with white text (3.6:1, below AA).

## Explicitly not done

- The light accent cannot be both `#008fca` and a text colour at 4.5:1; the split
  is the resolution. Reverting to one token is a one-block change if the owner
  prefers the deeper solid accent.
- Bubble text is now dark ink on bright azure rather than white on azure. White
  on the original accent is 3.6:1 and cannot be restored without failing AA.
- `UI-VITEST-NODE25`, `I18N-CHECK-RED` and the other open board rows are
  untouched.

## Record

- Screenshots: `shots/{default,dark}-1280-{chat,settings}.png`
- Verification: [verification.md](verification.md)
- Acceptance: [acceptance.md](acceptance.md)
