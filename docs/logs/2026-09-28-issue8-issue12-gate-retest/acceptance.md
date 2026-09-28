# Acceptance

Deterministic gates (reproduce on this branch):

```text
export VIVY_POSTGRES_TEST_DSN=postgres://vivy:vivy@127.0.0.1:5432/vivy?sslmode=disable
go test -v -timeout 10m ./internal/storage/postgres   # PASS 11.79s
just ci                                             # PASS (all recipes)
go test ./internal/runtime -run TestPlanSubmissionSurvivesToolApprovalGate -v
```

Browser gates: see `e2e-report.md` in this directory — every item cites the
journal run/goal/workflow ID and observed UI state. Screenshots in this
directory show the Run Inspector DAG completion, the PlanReview card, the
Smart-preset approval→review→complete fix verification, the failed-provider
surface, and the completed children list. Recordings are session artifacts
(>30 MB, not committed): `rec-bcaac0e6-dc83-4bcc-a07f-cf3a7e1ebd90-edited.mp4`
(full pass) and `rec-fix-verify-edited.mp4` (Smart-preset re-verification).

Gate status after this run:

- **G0** (PG DSN conformance + `just ci`): PASS — evidence above and in
  `verification.md`.
- **G1** (integrated child lifecycle / restart truth): PASS — continuity
  atomicity on both backends + browser interrupt/reload/restart evidence.
- **G3** (real host/browser E2E for orchestration surfaces): PASS — all
  ORCH-08 browser items verified at 127.0.0.1:3015.
- **G4** (release): NOT claimed — `just ci` and dev-UI evidence exist;
  release decision remains with the owner.
- **PG-0 / PG-1** (PostgreSQL migration + conformance): PASS.
- **PG-5** (goal/plan GUI acceptance): PASS — exercised and verified.
- **PG-6**: browser acceptance items PASS (pause, failed provider,
  re-planning, truthful restart, Clear Goal, plan review both decisions);
  the **live-provider coding walkthrough remains BLOCKED** — no credentials
  were provisioned, per the recorded deferral. Not a pass.
