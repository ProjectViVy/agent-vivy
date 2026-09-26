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
