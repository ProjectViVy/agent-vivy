---
name: testing-vivy-ui
description: How to run the Vivy split dev pair and exercise run/orchestration UI surfaces in a real browser without a provider key (hanging mock endpoint trick, Run Inspector location, locale toggle, air-gap rules).
---

# Testing Vivy UI (agent-vivy)

## Dev servers

- Backend: `PATH=/usr/local/go/bin:$PATH VIVY_CONFIG=/home/ubuntu/.vivy-dev/config.yaml go run ./cmd/vivy` from repo root -> control plane `127.0.0.1:8787`. The config redirects sqlite/workspaces/settings to `~/.vivy-dev`; never let it write under the repo's `data/` (ST-2 air gap).
- UI: `cd ui && PATH=$HOME/.local/node/bin:/usr/local/go/bin:$PATH pnpm dev` -> Vite `127.0.0.1:3015`, proxies `/rpc`. **Go must be on PATH** — `pnpm dev` runs `scripts/stage-ui-assembly.mjs` which shells out to `go run ./sdk stage-ui`; without it the dev server exits (`spawnSync go ENOENT`).
- Open `http://127.0.0.1:3015`. First run shows a welcome wizard; click "Skip wizard" (its text is left of the Next button, they visually overlap — click ~x=410 of the footer).

## Getting an `active` run without a provider key

`child/*` and `workflow/*` RPCs and the Run Inspector's Validate/Start buttons require the parent run to be server-side `active` with an in-process tool ceiling (`internal/runtime/child_sessions.go` `currentChildAuthorizer`) — a failed run cannot exercise them, and a synthetic DB row fails ("child authorizer tool ceiling is unavailable").

Trick: run a hanging OpenAI-compatible endpoint and register it via the UI:

```python
# GET /models returns a stub; POST hangs forever -> the run stays 'active'
# (python3 http.server ThreadingHTTPServer on 127.0.0.1:9911)
```

Then Settings → Model → "Add custom provider": adapter OpenAI-compatible, `http://127.0.0.1:9911/v1`, any api_key, model `mock-model`; click the model to select it. A sent run goes `active` and stays there until the client's model timeout (~2-3 min) — enough to exercise child/workflow controls. Descendants hit the same mock and stay `running`.

## UI paths

- Run Inspector: sidebar → Settings → "Vivy Features" tab → "Run Inspector" card (tabs: Current Run / Background / Children / Reviews). Children tab hosts "DAG workflows" `<details>` (descriptor textarea + Validate/Start) and the "Start child run" form (one-shot vs "Keep a continuable child session").
- Locale: Settings → Language tab → 简体中文 / English cards. Settings supports `?tab=language|vivy|...` deep links — use them when translated tab labels are hard to click by coordinates.
- Workflow descriptor schema: `{schema_version:1, start_nodes:[...], nodes:[{key,task,tool_names?}], edges:[{from,to,input_key?}], outputs:[...]}` — the textarea is prefilled with a valid minimal example.

## Notes

- Continuable-child follow-up while the child's activation is still in-flight returns "child operation conflicts with current state or authority" — interrupt the activation first, then the follow-up succeeds on the same `csess_` session.
- "Messages waiting for the parent" panel lists child→parent pending mailbox messages only; a parent→child send is confirmed by the "Message <id> admitted to the child inbox" status line, not a panel row.
- After a backend restart, an interrupted descendant shows "The child worker was lost during server restart. Retry explicitly to avoid duplicate side effects." (fencing is intentional).
- Console stays clean — missing i18n keys would surface as raw `runInspector.*` strings in the UI.

## Real-provider runs (openai-completions custom provider)

- Register a provider via Settings → Model → "Add custom provider" (adapter openai-completions, base URL `https://token.sensenova.cn/v1`-style, key, model). The API key field commits **on blur** (`onBlur` in `ModelSettingsCard.tsx`) — click outside the field before assuming it saved; verify in `~/.vivy-dev/settings.yaml`.
- Reasoning models (e.g. SenseNova) emit `model.reasoning_delta` events; a single call can take 60–95s, so a parent run finishes too fast to race clicks.
- To hold a run `active` for orchestration ops: prompt "Use the write_file tool to create X.txt containing 'y', then reply DONE." → the tool call lands in `tool.approval_required`, and under the smart preset the run stays `active` for the approval window (`approval_timeout_seconds`, default 300s). Do NOT use `sleep` — sandbox denies it and the run then fails via the pause-for-approval path.
- Child lists are **run-scoped**: `Children N` under a new run shows only that run's children. To reach a previous run's child detail (history/mailbox/follow-up), that run must still be `currentRun`; after a newer run exists you cannot reach it in the UI.
- `child/message/list` (pending panel) needs the authorizer run `active` on the origin parent session with a matching policy ceiling — it shows an inline "child operation conflicts with current state or authority" otherwise. The child-history fetch in the same effect doesn't need active, but both are fetched together when a child row is expanded.
- The child detail effect keys on `[run.id, child.id, child.session_id, child_mode]` — expanding the same row again does NOT refetch; select a different child or reload the page to refresh history/pending panels.
- Child→parent `reply_parent` replies stay `pending` until a parent run calls the `child_inbox` tool (prompt e.g. "use child_inbox to check pending messages from your child sessions"); they are not auto-injected into the next run input.
- Workflow node children CAN call `ask_user`; since nobody answers it, the node fails with `run.failed` `cause_category: human_timeout` and the workflow shows "The child task did not complete successfully." Give node tasks a concrete self-contained instruction to avoid it.
- No `sqlite3` CLI; inspect the journal with `python3 -c` + sqlite3 module read-only: `sqlite3.connect('file:/home/ubuntu/.vivy/vivy.db?mode=ro', uri=True)` (the dev config still redirects workspaces/settings to `~/.vivy-dev`, but the journal lives at `~/.vivy/vivy.db`). Tables: `runs`, `run_events`, `session_work_events` (goal/plan lifecycle mutations), `child_sessions`, `child_mailbox_messages`, `workflow_revisions`. Backend also writes rotated logs to `~/.vivy-dev/logs/vivy.log.<date>`; run-level failures log `run failed ... err=` to the backend stdout log.

## Goal + Plan-review controls (work bar)

- The work bar (`section[aria-label="Work control"]`) hosts Create/Edit/Clear Goal, Pause/Resume Goal, Enter/Leave Plan, and the PlanReview card (renders only when `plan.submission_id` is set and shows Request revision / Execute plan once while `review_status === 'pending'`).
- Order matters: `submit_plan` requires plan mode entered FIRST (`plan.entered` committed → `plan.active`), otherwise the tool returns a clean `stale goal reference: invalid plan submission` error and the run completes normally. An Active goal blocks Enter Plan (`active Goal must be paused before Plan`); Blocked goals allow it.
- `submit_plan` under the default Smart preset (which prompts for effectful tools) used to crash the run on approval-resume — `run.failed` with `runtime: Plan review resume does not match its submission` — fixed at `internal/runtime/plan_review.go` (foreign `vivy:tool-approval:` interrupt state is no longer consumed as a plan decision). Covered by `TestPlanSubmissionSurvivesToolApprovalGate`. If it ever regresses, the workaround is adding `"submit_plan"` to `runtime.sandbox.approval.auto_approve_tools` in `~/.vivy-dev/config.yaml` + the **Trusted** preset.
- Both decisions verified: Request revision commits `plan.decided action=revise` + feedback, resumes the suspended run; Execute plan once commits `action=execute_once`, resumes + completes the run, and plan mode exits (Enter Plan returns).
- Work-bar staleness recovery: the `session/work/subscribe` push can lag journal commits (observed after plan.* events). Any commit with a stale `expected_version` surfaces "Work control error: stale work version" + a **Refresh work state** button — click it to converge (e.g., open Edit Goal → Save Goal on an unchanged form is a harmless trigger; the commit itself fails harmlessly).

## Fast mock fixture (marker-routed tool calls)

- `/home/ubuntu/mock-llm.py` (port `127.0.0.1:11434`, OpenAI-compatible) routes on markers in the LAST user message: `VIVY-HANG` or model `mock-hang` → hang; `VIVY-AGENT` → `agent` tool_call; `VIVY-FLOW` → `workflow` tool_call (alpha‖beta→join DAG); `VIVY-PLAN` → `submit_plan` tool_call (`# E2E Plan` markdown); `VIVY-GOAL` → `create_goal` tool_call; last message role `tool` → final text; else instant text. Register it as provider base URL `http://127.0.0.1:11434/v1` with models `mock-hang` + `mock-fast`.
- Only the embedded `vllm` vendor executes runs — a custom "adapter" provider fails with "no embedded vendor data". Switch models via Settings → Model → "Saved models" chips (the header model switcher dropdown may not open).
- Failed-provider check: kill the mock (port down) → a sent run fails fast with `connection refused`; UI shows "Unable to connect! Check your provider configuration!" in the chat — clean failure, not a hang.
- Mock bugs to avoid: scan only the LAST user message for markers (earlier `VIVY-HANG` in history otherwise poisons later calls), and treat only a trailing `role:"tool"` message as tool-result (an earlier tool message misroutes).

## Click reliability in this environment

- The browser viewport renders ~2090 CSS px wide while the tool coordinate space is 1024 px — raw `computer` clicks on small React buttons frequently miss. Reliable workaround via `browser_console`: dispatch a full pointer sequence on the target element —
  `for(const ty of ['pointerover','pointerdown','mousedown','pointerup','mouseup','click']) el.dispatchEvent(new (ty.startsWith('pointer')?PointerEvent:MouseEvent)(ty,{bubbles:true,cancelable:true,clientX:cx,clientY:cy,button:0,pointerId:1,pointerType:'mouse'}))`
  where `cx,cy` come from `el.getBoundingClientRect()` center. Works for tabs, permission presets (incl. the "Switch to Trusted?" confirm dialog), work-bar buttons, and the Approvals panel.
- `browser_console` return values often show `undefined` — use `console.log()` inside the script and read "Logs from your script" in the response.
- The chat textarea is often offscreen; `ta.scrollIntoView({block:'end'}); ta.focus()` before typing. For controlled inputs (feedback textarea, run-picker select), set `.value` via the native setter (`Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype,'value').set`) then dispatch `input`/`change`.
- Full reload (`F5`) runs `background/recover` which kills unsuspended active runs — only reload when all runs are terminal, or navigate in-SPA (sidebar links / `alt+Left`) to refresh views without the sweep. Suspended runs (plan-review interrupt) survive restart and are rebuilt.
