# Acceptance — reference-project license review

How a human confirms it is effective:

1. Open `docs/research/license-review-2026-09-02.md`: §1 should quote the upstream
   claude-code LICENSE verbatim (proprietary ruling), and §2 should give the key points of
   rig's full MIT license plus the correction of the earlier misreading.
2. In §6 of `docs/research/REFERENCE-INDEX.md`, RI-OQ-1 / RI-OQ-2 should be marked
   RESOLVED and point to that document.
3. In §0.1 of `docs/TODO.md`, the P3-1 and P3-2 lines should show DONE 2026-09-02 and
   include filing links.
4. Behavioral invariant: grep should find no source code from claude-code / rig in the
   repository — this slice makes no code changes, and anyone considering reuse will first
   encounter the license-review conclusion.
