# VIVY CODE token and cost presentation

## Changed

- Completed the active-session right rail with authoritative session totals for input, output, reasoning, cached, total tokens, request count, and reference cost.
- Added Crush-style context percentage and an over-80% warning while marking byte-derived token counts with `~`.
- Added explicit runtime/RPC truth flags for estimated token counts and known model limits. Unknown model metadata no longer exposes the internal 128k fallback as an authoritative percentage.
- Kept unknown or partially priced sessions distinct from known zero-cost sessions.
- Preserved built-in and packed face parity through the shared surface/view, with adapter tests covering every usage field and the new context truth flags.

## Explicitly not done

- No prices, context windows, or token counts are inferred in the terminal.
- Reference cost is not presented as a provider invoice. Follow-up attribution/accounting gaps found during the broad audit are recorded as `TUI-USAGE-ACCOUNTING` in `docs/TODO.md`.
- No Studio source or tenant Journal was read or changed.

This is a focused source delivery, not a release; no release record is included.
