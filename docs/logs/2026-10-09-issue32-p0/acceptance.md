# P0 acceptance and handoff

Review the [phase index](../../superpowers/plans/issue32-remediation/index.md)
and this baseline record before implementing code.

- Confirm both refreshed main SHAs match the plan, and that the DIVA consumer
  lock still points to its recorded VIVY/Laputa versions. Final consumer repins
  are owned by P7.
- Confirm DIVA's tracked `agent-diva-gui/pnpm-lock.yaml` SHA equals the value in
  `build/vivy-sources.lock.json`; P1.2 must exercise a frozen install.
- Confirm all H/C/W/R finding IDs have exactly one owning task. R2 is
  superseded by owner decision #40 / merged PR #42; no other finding is marked
  fixed. Temporary probe passes mean the defect reproduced.
- Keep platform and environment gaps visible: current verification has no
  PostgreSQL DSN, Windows runner or microphone/audio subsystem. The plan does
  not count any of these as passed.
- Start the first blocking tranche with P1.1-P1.3 and P2.1-P2.2. Use each
  task's regression as the red gate before writing its product implementation.

P0 confirms source and evidence ownership. It does not establish any product
acceptance or release status.
