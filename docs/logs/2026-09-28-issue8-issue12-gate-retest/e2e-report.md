# Vivy E2E acceptance report — Issue #8 (ORCH-08/G3) + Issue #12 (PG-5/PG-6)

Repo: agent-vivy, branch `devin/1790577560-pg-dsn-conformance` (first pass @ `8387f2b0`; the defect fix + re-verification landed on top)
Session under test: `sess_f40c41bbd88a1c24` at `http://127.0.0.1:3015`
Fixture: OpenAI-compatible mock LLM at `127.0.0.1:11434` (`mock-fast` tool-call markers + `mock-hang`), dev config `~/.vivy-dev/config.yaml`, journal `/home/ubuntu/.vivy/vivy.db` (read-only queries).
Recording: `rec-bcaac0e6-dc83-4bcc-a07f-cf3a7e1ebd90/rec-bcaac0e6-dc83-4bcc-a07f-cf3a7e1ebd90-edited.mp4`

## Summary

- Issue #8 checklist: all 7 items exercised and PASS.
- Issue #12 checklist: all items exercised — every control verified end-to-end, with **one real defect** (submit_plan × tool-approval) that blocks the plan-review flow under the default Smart permission preset.
- All evidence cites journal rows and observed run IDs.

## A) Issue #8 — orchestration surfaces

| # | Check | Result | Evidence |
|---|-------|--------|----------|
| A1 | Child delegation | PASS | UI "Start child run" (Run Inspector → Children): one-shot `child_1f49904f8f7b22ad` + continuable `csess_23535a85dac90bef` under parent `run_52c58e8013650620`. Model-authored child via `agent` tool_call: `run_31b8edb2f3e8fcce` → `child_d2a91e0a5591dad9` (completed). |
| A2 | Follow-up/interrupt to child | PASS | Interrupt on in-flight activation cancelled `run_00296825f9f1ba38`; follow-up re-admitted `run_814a82c6345fd3af`; mailbox message `cmsg_b25a8eb0cdb39b05` admitted to child inbox. |
| A3 | Workflow/DAG proposal + graph | PASS | UI "DAG workflows" validate→start (`workflow_8cbd9874a05bc983`, digest `4a7e472e…`); model-authored DAG via `workflow` tool_call (`workflow_dc247b20789364cd`, revision `939d7ddb680e9d9f`) rendered in inspector with node list. |
| A4 | Parallel join | PASS | `workflow_dc247b20789364cd` completed; nodes alpha/beta/join all `completed`; declared output `{"join":"MOCK-REPLY: task acknowledged."}`; 3 `workflow_child_*` rows completed in Children tab. |
| A5 | Live status surfaces | PASS | Run badges, Children counters, workflow node states update live; minor lag noted (subscription converges ~1 event late). |
| A6 | Interrupt/reload persistence | PASS | Full reload + backend restart: journal terminal states shown truthfully; no phantom "active" for cancelled/failed runs; suspended/active runs swept by Recover. |
| A7 | zh locale | PASS | Settings → Language → 简体中文 localized inspector/work-control strings; toggled back to English cleanly. |

## B) Issue #12 — Goal lifecycle + GUI controls

| # | Check | Result | Evidence |
|---|-------|--------|----------|
| B1 | Create Goal via GUI | PASS | `goal-ed2a3f0cb88bfd0a` and `goal-9fb85c00220e5c8a` created via WorkControlBar → goal.created committed, round 1 auto-admitted (`goal.round_admitted`). |
| B2 | Inspect Plan/Goal views | PASS | Work bar shows phase/rounds/activation/current_run_id/reason/evidence_run_id; PlanReview card renders submitted markdown + status. |
| B3 | Pause / resume / cancel | PASS | Pause → `goal.paused` + owned run cancelled (`run_507f106ff5b09717` etc. cancelled); Resume → `goal.resumed` + new round admitted; round-limit → `goal.blocked` ("goal round limit reached", evidence `run_b11563cab5f61c5e`). |
| B4 | Reload/restart truthful recovery | PASS | Backend killed mid-active-goal → restart + reload: goal Active/Disarmed in journal, interrupted round runs terminal — no phantom running. |
| B5 | Later automatic runs | PASS | Goal auto-admitted round 2 without user input (`goal-9fb85c00220e5c8a` rounds: `run_724a3ba4faae14a9`, `run_b11563cab5f61c5e`); blocked at max_rounds with reason+evidence. |
| B6 | PG-6 extras | PASS (with defect below) | Manual pause verified (B3). Failed-provider: killed mock endpoint → `run_dabd5f1c35b84577` failed fast, UI showed "Unable to connect! Check your provider configuration!". Re-planning: Enter Plan → submit_plan → PlanReview → **Request revision** (feedback committed, `plan.decided action=revise`, run resumed+completed) and **Execute plan once** (`plan.decided action=execute_once`, run `run_0f6cca8e091fc569` completed, plan mode exited). |

## Follow-up: defect RE-VERIFIED fixed under Smart preset

Fix `internal/runtime/plan_review.go` (persisted `vivy:tool-approval:v1:` interrupt state now returns not-handled so `authorizeToolDispatch` consumes the resume; plan-review interrupt then suspends for the real decision) — re-tested in-browser with the `auto_approve_tools` workaround REMOVED from `~/.vivy-dev/config.yaml` and backend rebuilt+restarted (pid 118573, log `/tmp/vivy-backend-fix.log`).

`run_dce110caac26e98e` (mock-fast, Smart preset, policy gated):
```
seq 3 tool.requested(submit_plan, "# E2E Plan") → 4 policy.evaluated "prompt"
→ 5 tool.approval_required(apr_92d0e8b14efbba56) → 6 tool.approval_decided(approved)
→ 7 policy.evaluated → 8 tool.started → 9 tool.finished → 12 run.completed
work events: 25 plan.entered → 26 plan.submitted → 27 plan.review_suspended → 28 plan.decided(execute_once)
```
UI: Approvals panel approved → PlanReview card "Review pending / # E2E Plan" (after F5 rehydrate) → **Execute plan once** → `run.completed` ("MOCK-REPLY: delegated work finished."), plan mode exited, header "Vivy completed". Backend log clean — no `run failed`/`does not match`. **PASS** (previously `run.failed internal_error`).

Recording: `rec-fix-verify/rec-fix-verify-edited.mp4`
Screenshots: `ss_55d288d1.png` (approval detail Approved, run id visible), `ss_f0afa6d6.png` (PlanReview card pending after rehydrate), `ss_a4e00aa2.png` (completed, Enter Plan returned).

Note: the work-subscription lag recurred — PlanReview card did not appear until F5 rehydrated the work view (suspended run survives reload via Recover). Cosmetic only; journal was correct throughout.

## Defect found (BLOCKING for plan review under default preset) — FIXED + re-verified

**`submit_plan` crashes the run when gated by tool approval (Smart preset).**
- Repro 2/2: `run_f2d845bd1a71134f`, `run_0f4eaa5a9c32f71f` — events: tool.requested → policy.evaluated "prompt" → tool.approval_required → tool.approval_decided(approved) → run.failed.
- Backend error: `failed to stream tool call call_plan_1: runtime: Plan review resume does not match its submission` (`internal/runtime/plan_review.go:86`).
- Root cause: the tool-approval resume value is delivered to `resumePlanReview`, which expects a plan-review decision JSON carrying the matching submission ID — approval decision ≠ plan decision → run-level `internal_error`, not a graceful tool error.
- User impact: Smart is the default composer preset and submit_plan is classified effectful → plan review is unreachable for a default user. Only worked under **Trusted** preset + `runtime.sandbox.approval.auto_approve_tools` containing `submit_plan` (added to `~/.vivy-dev/config.yaml`, a dev-config fixture, not repo code).
- Second-order: when plan mode is NOT active, submit_plan tool returns a clean error `stale goal reference: invalid plan submission` (correct validation, run completes).

## Minor issue (non-blocking)

- Work-bar subscription lag: `plan.submitted`/`plan.review_suspended` pushes didn't refresh the card; a stale-version commit surfaced "Refresh work state" which converged it. Same pattern seen earlier for goal events.

## Constraints honored

- Never touched repo `data/`; all dev state under `~/.vivy-dev` and `~/.vivy` journal read-only.
- First pass modified no repo code; the defect fix then landed in `internal/runtime/plan_review.go` (see section above). Dev-config `auto_approve_tools` and mock fixture `/home/ubuntu/mock-llm.py` are test-side artifacts.
- zh locale toggled back to English; mock LLM left running on 11434; backend running as `/tmp/vivy-build` (pid 106656, log `/tmp/vivy-backend-plan.log`).

## Out of scope / declared gaps

- No live provider credentials — real-model paths untested (mock fixture only).
- PG-6 live coding walkthrough not exercised.
- `submit_plan` plan-execution semantics ("execute once" spawns no separate run — it resumes the originating run only; verified via journal `plan.decided action=execute_once` + run completion).
