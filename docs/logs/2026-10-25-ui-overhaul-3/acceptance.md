# Block 3 — acceptance

Open the split pair (`just run` + `cd ui && pnpm dev`) at
`http://127.0.0.1:3015` and send one long message. What a human should see:

1. **User bubble** — the 3k-character paste wraps inside a brand-tinted bubble,
   nothing is cut off at the right edge, and no horizontal scrollbar appears at
   390px or 1280px. Long unbroken paths inside the bubble wrap rather than
   overflow.
2. **Assistant answer** — reading text is 14px with 1.7 leading; while streaming,
   a 2px brand caret blinks after the last text node (no `…` marker). Fenced code
   renders as a bordered block with a language label and Copy button; long code
   lines scroll inside the block only.
3. **Tool row** — one collapsed line per call at 28px minimum height; a long tool
   name or argument wraps to at most two lines and never shows a clipped glyph.
   Expanding shows the raw body with `text-meta` sizing; a failed call is toned
   with the danger colour, and the row still announces Running / Stopped /
   Awaiting approval to assistive tech.
4. **Delivery card** — a long preview shows 12 lines and an explicit "Show all
   lines" control; nothing is hidden behind a fixed `max-h-40`.
5. **Transcript column** — measured against `--content-max` (52rem) instead of the
   old `max-w-4xl`; aligning the composer to the same column lands in Block 4.
6. **Mobile** — at 390px the whole transcript wraps; there is no horizontal page
   scroll in any of the three captured states.

Comparison images: `transcript-{1280,390}-{rich,tool,approval}.png` in this
directory. The overflow measurements behind the claim are in
`overflow-report.json`.
