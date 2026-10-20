# WSL kernel verification and D02 acceptance

The owner requires stable as the integration branch and a Linux-filesystem WSL
checkout for main verification. This cut restores portable verification through
the existing owners, rather than repeatedly running Windows full-product CI.

- The Node pinned-Laputa bootstrap owns validation and setup; PowerShell only
  forwards. Origin, clean source, full pin, three module identities and Go
  requirements are checked before use.
- Scoped recipes separate wiring, minimal kernel, D02 protocol/Face contracts,
  affected Web, race and real PostgreSQL. Missing required tests and a missing
  disposable DSN fail closed. Readiness agrees with command venue constraints.
- Minimal Pack uses the existing compiler, overlay, sealing, Inspect and real
  health probe. Only the explicit minimal profile without UI contributions
  omits frontend compilation. Lite keeps its built-in Web shell; selected UI
  remains compiled and embedded. A cold checkout needs no prebuilt UI dist.
- Ubuntu kernel CI runs without installing frontend dependencies. The restored
  environment skill and AGENTS.md describe actual commands and their limits.

No Eino runtime, dependency, migration or D03 operation/approval contract is
introduced. Pinned Eino v0.9.21 TypedRunner and the Service.Run/Journal path
remain unchanged. This construction/boot gate does not claim D13 usable tools
or full default composition. No release or push is requested.

Related D02 prerequisite repairs and producer evidence are recorded in
`../2026-10-20-issue91-d02-acceptance/`. The issue-91 index owns Story status.
