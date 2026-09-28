# Gate retest — issue #8 (ORCH) + issue #12 (plan-goal)

Date: 2026-09-28. Branch: `devin/1790577560-pg-dsn-conformance`.
Head commits: `3214a63c` (postgres conformance fixes) + `8387f2b0` (internal
source digest re-pin).

## Environment

- Go 1.26.8 (`/usr/local/go/bin`), just 1.58.0, PowerShell 7.6.6, Node 24.9.0,
  pnpm 11.19.0, Docker postgres:16 on `127.0.0.1:5432`
  (`postgres://vivy:vivy@127.0.0.1:5432/vivy?sslmode=disable`).
- Dev data redirected to `~/.vivy-dev` via `VIVY_CONFIG`; repo `data/` untouched
  (ST-2).

## G0 / PG-0 / PG-1 — PostgreSQL DSN-backed conformance

`VIVY_POSTGRES_TEST_DSN=postgres://vivy:vivy@127.0.0.1:5432/vivy?sslmode=disable
go test -v -timeout 10m ./internal/storage/postgres` → **PASS (11.79s)**.

Full suite green, including `TestHistoryConformance`, `TestContinuityAtomic`
(fault_after_message/run/events/receipt), `TestContinuityRetry`,
`TestContinuityExpectations`, `TestFencedWritesReturnLeaseLost`,
`TestDualOpenExclusive`, `TestMigrateUpgradesV14InPlace`.

Two real defects found and fixed on this branch:

1. `messages.content`/`notes.content` are `BYTEA`; pgx text-encodes Go strings,
   so any content containing `\x00` failed with SQLSTATE 22021. All writers now
   pass `[]byte(m.Content)` — same pattern already used for
   `session_compactions.summary`.
   (`internal/storage/postgres/{messages,continuity,run_admission,runs,work_control,history_mutations,notes}.go`)
2. `conformance.AssertContinuityAtomic` re-opened the backend while the faulted
   engine still held the organism lease. Postgres `Open` takes an advisory
   session lock + lease row inline, so the reopen was fenced
   (`storage.ErrLeaseHeld`). Test restructured to close the faulted handle
   before reopening — matching a real process restart; no product change.
   (`internal/storage/conformance/continuity.go`)

## G0 — repository `just ci`

`just ci` → **PASS**. fmt-check; ui-ci (pnpm typecheck, 491 vitest tests,
vite build, i18n completeness + cross-face checks); `go vet ./...`;
`go test -timeout 20m ./...` all packages green (incl. `sdk/internal` 196s,
`sdk/internal/conformance` 51s after digest re-pin); headless-compile;
plugin-ci (all `plugins/*` + `faces/*` Go modules vet+test green).

Note: `conformance_results.json` internal `sourceSha256` was re-pinned
(`go run ./sdk/internal/cmd/source-hash internal ""` → `d73fa735…`, five
entries) — required maintenance after any `internal/` source change.

## G1 — integrated child lifecycle / restart

- Storage-level restart semantics: `AssertContinuityAtomic` verified on both
  backends (postgres run above + sqlite via `just ci`).
- Backend restart truthfulness exercised in browser E2E (see below).

## G3 / PG-5 / PG-6 — browser E2E at http://127.0.0.1:3015

Executed against the dev backend (journal `~/.vivy/vivy.db`, read-only
inspection; repo `data/` untouched) with a marker-routed OpenAI-compatible
mock at `127.0.0.1:11434`. Full evidence: `e2e-report.md` in this directory;
recordings not committed (>30 MB); see acceptance.md.

Issue #8 / ORCH-08 checklist — all PASS, journal-cited:

| Item | Evidence |
|---|---|
| Child delegation | `child_1f49904f8f7b22ad` + continuable `csess_23535a85dac90bef` under `run_52c58e8013650620`; model-authored `run_31b8edb2f3e8fcce`→`child_d2a91e0a5591dad9` completed |
| Follow-up / interrupt | `run_00296825f9f1ba38` cancelled; follow-up `run_814a82c6345fd3af`; mailbox `cmsg_b25a8eb0cdb39b05` |
| DAG proposal + graph | UI `workflow_8cbd9874a05bc983`; model-authored `workflow_dc247b20789364cd` (rev `939d7ddb`) rendered in Run Inspector |
| Parallel join | alpha/beta/join all `completed`; join output materialized |
| Live status | badges/counters update live (minor subscription lag) |
| Interrupt/reload + restart truth | reload + backend restart: terminal states truthful, no phantom running |
| zh locale | inspector + work-control strings localized |

Issue #12 / PG-5+PG-6 checklist — all PASS after one real fix (below):

| Item | Evidence |
|---|---|
| Create Goal (GUI) | `goal-ed2a3f0cb88bfd0a`, `goal-9fb85c00220e5c8a`; round 1 auto-admitted |
| Inspect Plan/Goal views | phase/rounds/activation/reason/evidence + PlanReview card |
| Pause / resume / cancel | `goal.paused` cancels owned run; resume admits new round; round-limit `goal.blocked` (evidence `run_b11563cab5f61c5e`) |
| Reload/restart recovery | truthful terminal states, no phantom running |
| Later automatic runs | round 2 auto-admitted without input |
| PG-6 extras | manual pause; failed-provider surfacing (`run_dabd5f1c35b84577` → "Unable to connect!"); re-planning via `plan.decided action=revise` and `action=execute_once` (both resumed+completed); Clear Goal |

### Defect found and fixed during E2E

`submit_plan` under the default Smart preset crashed on tool-approval resume:
`run.failed: runtime: Plan review resume does not match its submission`
(reproduced 2/2: `run_f2d845bd1a71134f`, `run_0f4eaa5a9c32f71f`). Root cause:
`resumePlanReview` consumed the tool-approval resume — the persisted interrupt
state was the `vivy:tool-approval:v1:` encoding, not a submission ID, and the
approval decision is not a plan decision. Fix: pass that foreign interrupt
state through to `authorizeToolDispatch` (`internal/runtime/plan_review.go`).
Regression test `TestPlanSubmissionSurvivesToolApprovalGate` fails on the old
code with the exact reported error and passes with the fix. Re-verified
in-browser under Smart: `run_dce110caac26e98e` —
tool.requested → policy.evaluated prompt → tool.approval_required →
approval_decided → plan.submitted → plan.review_suspended →
plan.decided(execute_once) → run.completed.

### Still BLOCKED

- PG-6 live-provider coding walkthrough: no live-provider credentials are
  provisioned (none in session secrets; per DEFER.MD ISSUE-47-EXTERNAL-GATES
  they were not to be filled). Blocked, not a pass.
- Minor, non-blocking: `session/work/subscribe` can lag journal commits — the
  "Refresh work state" action converges it. Cosmetic; journal correct.
