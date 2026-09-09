# Summary — reference-project license review (P3-1 claude-code / P3-2 rig)

## What changed

- `docs/research/license-review-2026-09-02.md` (new): first-party verification and
  rulings for two upstream licenses.
  - **P3-1 claude-code**: the full upstream `anthropics/claude-code` LICENSE.md is
    "© Anthropic PBC. All rights reserved. Use is subject to Anthropic's Commercial Terms
    of Service."; GitHub license detection is None. The ruling is proprietary, so any
    source/asset reuse is prohibited; the Defer position moves from "inferred from absence"
    to "confirmed by an explicit upstream statement."
  - **P3-2 rig**: upstream `0xPlaygrounds/rig` LICENSE is standard MIT; on 2026-08-06,
    the standard MIT copyright line was misread as a "custom license", resolving the doubt.
    Vivy intent remains Drop (Rust; Eino covers the same seam), and it is a license-safe
    candidate for a SystemV probe.
- `docs/research/REFERENCE-INDEX.md`: rewrote the §3.3 / §3.15 entries (upstream
  verification evidence + the fact that local copies were cleaned up); marked RI-OQ-1 and
  RI-OQ-2 RESOLVED.
- `docs/research/OPEN-ITEMS.md`: moved the P3-1 / P3-2 lines to DONE.
- `docs/TODO.md`: moved the P3-1 / P3-2 lines to DONE + §10 record.

## What was explicitly not done

- No code changes; no reference/reuse behavior changes (neither project was reused to begin
  with).
- P3-3 (Defer projects vs. Crush reassessment) and P3-4 (deep FSL-1.1-MIT reuse,
  DEFERRED) are outside this slice.

## Scope

Docs-only. On-site verification confirmed that `.workspace/` has no local claude-code / rig
copies; the review now relies entirely on upstream evidence.
