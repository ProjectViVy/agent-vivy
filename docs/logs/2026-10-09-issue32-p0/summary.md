# Issue #32 execution baseline

Date: 2026-10-09. Scope: P0 source/version/prerequisite and finding-traceability
gate before code tasks.

The currently reviewed GitHub main heads are VIVY
`017ec8cc37970b291e04c619990aed00d5403116` and DIVA
`518a33ef09858ee1bb190579dd7529aceaa15dd6`. The implementation branches are
isolated: VIVY `feat/issue32-remediation` in
`agent-vivy/.worktrees/issue32`; DIVA `feat/issue32-desktop-gates` is based on
the reviewed commit under `/workspace/work/issue32/agent-diva`. DIVA's lock was
RELEASED at baseline and is now held for the P1.1-H1/P1.2-H2/P1.3-H3 scope.

The 28-row index still maps 27 repairs and R2's owner-approved supersession.
No product finding is marked fixed. C1/R1/R3/R4 temporary probes are classified
as reproductions, and W5 migration subcases remain distinct from workflow W5/W6.

P1 preflight correction: the reviewed DIVA commit *does* track
`agent-diva-gui/pnpm-lock.yaml`. Its SHA-256
`01ef0ea82f41b54be2103a8bc7cf54a407a2be6b39a48a6137010a8acecea249` matches the
single recorded frontend-lock owner in `build/vivy-sources.lock.json`. The
planned P1.0 lock-creation prerequisite was erroneous and has been removed.
P1 now starts with H1 (`P1.1`); P1.2 tests the existing frozen build input.
The P1.1 source path audit also corrected repository ownership: DIVA tracks
`internal/desktop/runtime_service.go` and its tests; VIVY has no such package.

This baseline is a source/documentation gate. It does not claim passing product
tests, native builds, PostgreSQL, Windows, microphone/voice, final artifact
acceptance or release readiness.
