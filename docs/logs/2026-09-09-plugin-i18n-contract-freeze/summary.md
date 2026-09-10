# Plugin I18N contract freeze summary

Date: 2026-09-09

The approved plugin localization design is now normative across the Module,
Port, Plugin, and Assembly contracts. PLG-P1 and PLG-P2 were formally
scheduled together by the human owner; P1 is in progress in the isolated
`feat/plugin-v1-p1-p2` worktree and P2 is queued behind its compiler
foundation in the same clean-break landing unit.

The contract fixes one optional Module-owned `vivy.i18n/v1` catalog, literal
`plugin.<module-id>.*` ownership, English fallback, shared Web/TUI Host
resolution, compiler resource limits, canonical SHA-256 provenance, and
evidence-derived completeness. Localization remains a Host surface and does
not create a fifteenth public Port or a Grant.

Merge-conflict resolution is intentionally deferred until the branch reaches
the merge-ready stage, as directed by the human owner.
