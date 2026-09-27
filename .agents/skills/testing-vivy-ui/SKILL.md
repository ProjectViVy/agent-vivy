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
- No `sqlite3` CLI; inspect `~/.vivy-dev/vivy.db` with `python3 -c` + sqlite3 module (tables: messages, runs, run_events, child_mailbox_messages). Backend also writes rotated logs to `~/.vivy-dev/logs/vivy.log.<date>`.
