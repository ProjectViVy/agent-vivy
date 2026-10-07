# A2A-04 Acceptance

- TaskHost read/cancel/subscribe surface implemented per design §7/§8/§10:
  scoped, receipt-owned, journal-projection-only, bounded.
- No remote-visible tool args/results/reasoning/prompts/paths/credentials/
  checkpoints (asserted by unsafe-content exclusion test).
- Submit ordinary answers route through the A2A-03 atomic answer commit.
- List totals are exact and authorized; page tokens bound + expiring.
- Stream never manufactures success from transport close.
