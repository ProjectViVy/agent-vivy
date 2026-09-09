# FACE-PACK §14 four-question decision record

## What changed

`docs/architecture/VIVY-FACE-PACK.md` §14: all four "requires human decision before
adoption" questions received final user review (2026-09-02, all taking the contract's
recommended values), clearing the F2/F3 slice prerequisites:

1. **Standalone `go.mod` for `faces/`** — gateway-generation compilation does not see TUI
   dependencies.
2. **Default is always `face: web`** — the coding generation is a separate recipe and
   does not replace the everyday double-click flow.
3. **Web and TUI do not coexist in the first cut** — one mouth per generation; multi-client
   coexistence and switching require first-writer-wins approval to be nailed down first.
4. **Approval in headless = fail exit** — do not hang waiting and do not yolo (run-level
   durable suspension + cancellation is fixed by F1 tests; the process-level rule is fail
   exit).

`docs/TODO.md`: added the decision note to the FACE-TUI-1 line + §10 ledger entry.

## Explicitly not done

- Implementation of F2 (built-in `faces/headless` organ + pack overlay) and F3
  (built-in `faces/tui` organ) — this slice only clears their decision prerequisites.
