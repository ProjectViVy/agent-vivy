# Gate retest — issue #8 + issue #12

Re-ran the blocked formal gates for the two merged epics on branch
`devin/1790577560-pg-dsn-conformance`.

## What was previously blocked

- #8 (ORCH index): G0 needed `just ci` + PostgreSQL DSN-backed conformance;
  G3 needed a real host/browser E2E.
- #12 (plan-goal): PG-0/PG-1 needed the live PostgreSQL migration/conformance;
  PG-5 needed browser acceptance; PG-6 needed the browser walkthrough plus a
  live-provider coding run.

## What was done

1. Provisioned the toolchain (Go 1.26.8, just, PowerShell, Node/pnpm) and a
   Docker postgres:16 on 127.0.0.1:5432.
2. Ran `VIVY_POSTGRES_TEST_DSN=… go test -v ./internal/storage/postgres` —
   **two real defects** surfaced and were fixed:
   - `messages.content`/`notes.content` are `BYTEA`; string params were
     text-encoded by pgx and failed on `\x00` (SQLSTATE 22021). All writers now
     bind `[]byte(...)`, matching the existing `session_compactions.summary`
     pattern.
   - `conformance.AssertContinuityAtomic` reopened the backend while the
     faulted engine still held the organism lease; the shared test now closes
     the faulted handle before reopening (matching a real process restart; the
     exclusive-open contract is intentional per `DualOpenExclusive`).
3. `just ci` — full pass (fmt-check, ui-ci incl. 491 vitest tests + build +
   i18n, vet, `go test ./...` incl. `sdk/internal` 196s and the re-pinned
   conformance digest, headless-compile, plugin-ci over all plugin/face
   modules).
4. Browser E2E at `http://127.0.0.1:3015` with a marker-routed mock LLM —
   every ORCH-08 item (child delegation, follow-up/interrupt, DAG proposal,
   parallel join, live status, interrupt/reload, zh locale) and every
   PG-5/PG-6 browser item (goal create, views, pause/resume/cancel,
   restart-truthful recovery, automatic rounds, failed-provider surfacing,
   re-planning, Clear Goal) **passed with journal-cited evidence**.
5. The E2E exposed **one real defect**: `submit_plan` under the default Smart
   preset crashed on tool-approval resume
   (`Plan review resume does not match its submission` — the approval resume
   was fed into the plan-review resumer). Fixed in
   `internal/runtime/plan_review.go` + regression test
   `TestPlanSubmissionSurvivesToolApprovalGate`; re-verified in-browser under
   Smart (`run_dce110caac26e98e` completed through approval → plan review →
   execute_once).

## Live-provider walkthrough

PG-6 Task 2 ran on real SenseNova `sensenova-6.8-flash-lite`:
`run_924ad3c3508cb877` — write `fib.py` via approved `write_file`, run
`python3 fib.py` via approved `bash`, `exit_code:0`; independently re-run
on disk (exit 0, matching output). Journal chain in `e2e-report.md`.

## Still blocked

- G4/release — remains the owner's decision; evidence is complete for G0/G1/G3.
- Minor: work-state subscription lags journal commits (Refresh work state
  converges it); cosmetic, filed as an observation only.
