# Acceptance — UI overhaul Block 0

Block 0 ships no visible product change. What a human should be able to confirm
is that the baseline is honest and that the ratchet enforces what the overhaul
promises.

## What to look at

1. **The baseline images** in this directory. Chat empty at 390 (Miku), 768 and
   1280 (Vivy light), 1280 (Vivy dark); settings at 390 and 1280 (Diva). They
   show the state the overhaul starts from: the composer column narrower than
   the transcript, 10/11px labels, the status colours picked per call site.
   These are the "before" frames every later block is compared against.

2. **Run the ratchet and read the numbers.** `cd ui && npx vitest run
   src/theme/tokens.test.ts` prints one line per category, e.g.
   `[token-ratchet] src/palette: 159 (baseline 159)`. The printed counts are the
   ones frozen today.

3. **See the ratchet bite.** Add `className="bg-emerald-500"` to any component
   under `ui/src` and re-run the suite: it fails with `newly violated by these
   files`. Remove it and it passes. Adding a stock palette class in a file that
   is already on the baseline still fails, because the count grows.

4. **Run the audit against a live pair.**
   `cd ui && pnpm run audit -- --base-url http://127.0.0.1:3016 --out .audit/after`
   writes `<theme>-<width>/<surface>.png` for 120 combinations and prints
   `audit: 120 screenshots, 0 failures`. Open two frames side by side with the
   committed baseline to compare a block's before/after. If the pair is not
   running it fails loudly instead of starting its own server, so a green audit
   always means a human-openable app was photographed.

## How the overhaul's contract is enforced from here

- Every later block must lower `src/palette`, `src/textSize` or `src/hex` and
  delete the files it cleaned from the baseline arrays in the same commit. A
  block that converts a file but forgets the baseline entry fails with
  `baseline is stale for these files; delete them`.
- The plugin group covers only trees containing a `ui` path segment, so
  `plugins/<module>/ui/<module>/src` is in scope and generated copies under
  `ui/src/generated` are not (they are skipped, not edited).
- The Zero at Block 7/9 is therefore checkable: an empty baseline with the same
  test file still passing.

## Known red the reader should not attribute to this block

`pnpm test` on this host reports 45 failures in 9 files from
`localStorage.clear is not a function` (Node ≥ 25/24 happy-dom shadowing,
`UI-VITEST-NODE25` in `docs/TODO.md`). It reproduces on an untouched checkout.
Everything Block 0 adds passes; the rest of the suite needs that separate fix.
