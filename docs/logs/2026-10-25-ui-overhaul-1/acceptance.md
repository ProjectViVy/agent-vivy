# Acceptance — UI overhaul Block 1: theme foundation and families

What a human should be able to confirm by opening the app, without reading
code. Run the split pair (`just run` + `cd ui && pnpm dev`), open
`http://127.0.0.1:3015`, then Settings → Theme.

## 1. Seven families, one contract

The picker lists seven cards in this order: **Vivy Light**, **Vivy Dark**,
**Diva**, **Miku**, **Rice & Vermilion**, **Graphite & Fluor**, **Night Indigo**.
Each card shows its own canvas, raised surface, brand chip (`Aa`), three text
bars and the four status dots. Those previews are rendered with the same CSS
tokens the app uses, so they cannot drift from the real theme.

Clicking a card:

- changes the whole shell immediately — sidebar, header, chat canvas, cards,
  buttons, focus rings — not just the accent;
- moves the selection ring and the check mark to that card;
- survives a hard reload (`Cmd+Shift+R`); the card is still selected and the
  shell keeps its colours.

## 2. No flash on cold load

Hard reload in every family (light families first, then dark ones). The page
must never paint white before a dark family, and never paint dark before a
light family. The first frame is already the family's canvas; the only change
after React boots is a slight hue settle for non-Vivy families (the pre-paint
literals are neutral Vivy-tinted by design; the plan forbids family ids in
`index.html`).

## 3. Appearance, not just accent

Pick **Vivy Light** and **Vivy Dark**: same hue family, inverted lightness.
Native controls follow: text selection, scrollbars, and the browser's
form-control rendering switch with the appearance (the root carries
`color-scheme`). Diva stays light; Miku, Fluor and Indigo stay dark.

## 4. Readable, not merely tinted

- The type scale now has names: 11px micro-label (`--type-2xs`), 12px metadata
  (`--type-meta`), 13px UI (`--type-ui`), 14px reading (`--type-read`), 16px
  panel title, 20px page title, with paired line heights. In this block they are
  definitions plus the picker; components adopt them in Blocks 3–6. No screen
  should look smaller or denser than before Block 1.
- Every family passes WCAG AA: body text on canvas ≥ 4.5:1, muted text on
  surfaces ≥ 4.5:1, brand label on brand fill ≥ 4.5:1, each status colour on
  both canvas and its own soft fill. The measured table is in
  `verification.md`.

## 5. Status colour is a contract

Success, warning, danger and info now come from four token families
(`ok`, `warn`, `danger`, `info`, each with `-soft` and `-fg`). Their values are
the same in every skin on purpose: a warning must not change meaning because
the user picked Diva. In Block 1 only the token definitions and the picker use
them; call sites move in Blocks 2–6.

## 6. Module themes still win

If a plugin UI module registers a theme (variables/classes/attributes), its
inline variables still override the shell's tokens after the user switches
families. Kernel switching only writes `data-theme`, `data-appearance` and the
`.dark` class on `<html>`; it never writes inline styles, so it cannot fight a
module. This is asserted by a unit test rather than by inspection.

## 7. One source, verifiable

- `ui/src/styles.css` is the only authored colour source: it contains no hex
  literal, and no other CSS file exists under `ui/src`.
- `ui/index.html` contains exactly four colour literals (two neutral pairs),
  no theme id, no `oklch()`, and reads only `vivy.appearance` before React.
- `ui/src/hooks/use-theme.ts` contains no colour literal at all.
- A repository test (`ui/src/theme/sources.test.ts`) fails if any of that
  changes, and another (`ui/src/theme/tokens.test.ts`) fails if a new stock
  palette class, arbitrary text size or hex literal appears anywhere in
  authored UI code.

## Known limits at this block

- Components are still written against the legacy shadcn aliases
  (`bg-background`, `text-muted-foreground`, stock `emerald/amber/red`).
  They resolve through derived aliases, so the app is consistent, but the
  semantic sweep is Blocks 2–6.
- No run-state evidence (streaming, approval, queue, tool failure) in this
  block's screenshots: it needs the frozen-env provider from Block 3.
