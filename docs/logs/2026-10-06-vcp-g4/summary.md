# G4 — Inline terminal images — summary

**Story:** `docs/superpowers/plans/vivy-code-parity/G4-inline-images.md`
**Commit:** `feat(tui): inline image display for supporting terminals`

## What landed

- `sdk/tui/view/images.go` — protocol probe (`detectImageProtocol`): kitty via
  `KITTY_WINDOW_ID`/`TERM`, iTerm2 via `TERM_PROGRAM=iTerm.app`, WezTerm maps
  to the kitty protocol. `resolveImageProtocol` maps the `images` option:
  `auto` (default, detect only), `on` (force kitty when undetected), `off`
  (chips only, unchanged behavior).
- kitty path: image bytes transmit **once** per attachment
  (`a=t,f=100,t=d,q=2`, 4096-char base64 chunks) via `imageTransmitCmds()`
  called from `Update`; each frame emits only the cheap `a=p` placement
  sequence plus blank rows reserving `imageCellRows` cells (cell aspect 2:1,
  capped at 18 rows).
- iTerm2 path: single OSC 1337 `File=inline=1` sequence embedded in the
  frame; payloads capped at 768 KiB (re-transmits per repaint); sequence
  cached per path.
- `renderImageAttachment` hooks into `renderMessageWithOptions` after the
  attachment chips; any failure (missing file, undecodable header, oversized
  iTerm2 payload, failed upload) returns nil → chip-only fallback.
- Flag wiring: `tui.images` in config.yaml → `face.Options.Images` →
  `view.Options.Images`; defaults resolved in codeface launch like theme.
- webp decode added (`golang.org/x/image/webp`); png/jpeg/gif via stdlib.

## Boundary kept

No mermaid, no sixel. No new Port — pure face-internal view behavior.
Image uploads go through `tea.Println` (renderer channel), so transmit
sequences never interleave mid-frame.

## Deviations / notes

- iTerm2 has no re-placement protocol: its full File= sequence rides inside
  every frame. Accepted per risk-flag; size cap limits repaint cost.
- kitty placement lands at the cursor cell (after the chat bar) — a 2-col
  offset, harmless.
- Visual verification in a real kitty/iTerm2 terminal is not possible on
  this VM; unit tests cover sequence generation, probe, sizing, transmit
  dedup, and fallback. Recorded as untested-with-reason.
