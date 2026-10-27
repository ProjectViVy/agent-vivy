# Vivy palette refresh — fresh azure chrome, white ink on the fill

Owner feedback on the Block 0–9 overhaul, in two rounds: "配色还是太重，回到原来的
清新配色，vivy 的主配色", then "你有一些蓝底黑字。这样的类型还有很多，全面排查".
This delivery restores the original ice-blue chrome and azure accent, and fixes
the whole ink/surface contract underneath it: every coloured surface now takes
the ink that belongs to it.

## The two defects

1. **Dark ink on the brand fill.** The first cut of this refresh restored the
   bright identity azure but had to put near-black ink on it to reach 4.5:1. That
   reads as a black-text highlight, not as the original blue bubble.
2. **`-fg` ink used on `-soft` tints.** About thirty call sites paired
   `bg-ok-soft` / `bg-warn-soft` / `bg-danger-soft` / `bg-info-soft` with
   `text-*-fg`, where `-fg` is the ink for the *solid* tone. In light that is
   white on a pale tint; in dark it is near-black on a dark tint — about 1.2:1
   in both. Status badges, read-only notices, MCP warnings, the trajectory
   ledger, the connection chip and the mask status labels were effectively
   unreadable. Three more sites used a white `-fg` with no coloured surface at
   all, and `text-primary` was used as text while `primary` resolved to the
   bright accent.

The colour audit did not catch (2) because it checks tokens, not pairings; the
screenshots did not catch it either because "invisible" and "deliberately
subtle" look alike at thumbnail size.

## The ink roles

| token | role | light | dark |
| --- | --- | --- | --- |
| `--brand` | accent marks: icons, rings, borders, dots, progress, caret (≥ 3:1) | `#028fc9` | `#00aee4` |
| `--brand-fill` | brand surface that carries ink (bubbles, send button, filled chips) | `#057bac` | `#057bac` |
| `--brand-fg` | ink drawn on `--brand-fill` | `#f8fdfe` (4.62) | `#f8fcfe` (4.63) |
| `--brand-ink` | brand-coloured text/glyphs on neutral surfaces | `#0073a6` (4.89) | `#00aee4` (7.25 canvas) |
| `--brand-soft` | pale tint; ink is `--brand-ink` | unchanged | unchanged |

`--brand-fill` defaults to `var(--brand)` in the shared ramp, so the five other
families keep one accent and their current ink. Only Vivy splits the roles.
`--color-primary` / `--sidebar-primary` now alias `--brand-fill`, so the
vendored shadcn button, badge, checkbox and switch carry white ink on the
deeper fill instead of on the bright mark.

Status tones keep `-fg` strictly for the solid tone (`bg-ok` + `text-ok-fg`,
`bg-danger` + `text-danger-fg`); a soft tint takes the tone itself
(`bg-warn-soft` + `text-warn`).

## Call sites

- 7 `bg-brand` + `text-brand-fg` surfaces → `bg-brand-fill` (bubble, send/steer,
  V wordmark, theme badge, welcome platform card, two channel platform chips).
- 30 pairs across 18 files: soft tint + `-fg` → soft tint + tone.
- 11 files: `text-primary` → `text-brand-ink` (it was text, not an icon, and in
  dark the fill value is not legible as text).
- Message-bubble action icons `text-brand-fg/70` → `/85` (2.96 → 3.7:1, the
  graphic threshold); a platform caption `text-brand-fg/90` → full ink.

## Ratchets

- `ui/scripts/audit/contrast.mjs`: 7 families, **182 pairs**, including
  `brand-fg on brand-fill` at 4.5, `brand-fill` on both neutral surfaces at 3,
  and the four `tone on tone-soft` pairs at 4.5.
- `ui/src/theme/tokens.test.ts`: new source rule — a soft tint may never be
  paired with the same tone's `-fg`, and `text-brand-fg` may never sit on
  `bg-brand` (it must be `bg-brand-fill`).
- `ui/scripts/audit/screenshots.mjs`: now measures **rendered** contrast on
  every audited surface (effective background with translucent ancestors
  composited, WCAG 1.4.3 / 1.4.11), so a wrong pairing fails the audit rather
  than surviving to a screenshot.

## Measured

Light bubble is `#057bac` with `#f8fdfe` ink (4.62:1); the marks and the
selection row keep the original azure (`#028fc9`) and ice-blue tint. Status
chips now render dark green on pale green (5.91), dark amber on pale amber
(6.82) and dark red on pale red (6.30).

## Explicitly not done

- The light accent cannot also be small text at 4.5:1; `--brand` stays the mark
  and `--brand-ink` carries text. Reverting to one token means a darker accent.
- The three dark families with a light accent (Miku, Fluor, Indigo) still draw
  dark ink on their bright accent, as does Diva-style contrast in reverse; their
  accents are their identity and white would not reach 3:1 on them.
- No layout, typography, component or Studio-shell change; only tokens, the
  ink/surface pairings, the audit and the two ratchets.

## Record

- Screenshots: `shots/{default,dark}-1280-{chat,settings}.png`
- Verification: [verification.md](verification.md)
- Acceptance: [acceptance.md](acceptance.md)
