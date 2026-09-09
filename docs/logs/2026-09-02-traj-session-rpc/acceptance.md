# Acceptance — UI-TRAJ / UI-TRAJECTORY-DEMO

## Manual acceptance path

1. `just run` (control plane 8787) + `cd ui; pnpm dev` (Vite 3015), open
   `http://127.0.0.1:3015`.
2. Enter "Console → Trajectory":
   - a "Session" selector appears at the top; when sessions exist, the first is selected by
     default;
   - a skeleton appears while loading; failures show a retryable error bar.
3. Select a session that has had a conversation:
   - the timeline shows three swimlane bands, and the ledger shows Session/User/ASSISTANT/TOOL
     rows;
   - clicking the right side of an ASSISTANT row opens details (Summary/Usage/Timing), with
     numbers using the same Token-statistics convention as the session (the same journal
     fact source);
   - clicking a tool row shows input/result details; an errored tool row is red.
4. Switch to another session: the ledger follows the session (it does not reuse the previous
   session's folding/search state).
5. A completely fresh store (no sessions) shows "No sessions; start a conversation to view
   trajectory."
6. A session with no run (for example, created but never used for a conversation) shows the
   "No trajectory records" empty state in the ledger.

## Kernel-observable behavior (no UI dependency)

- `trajectory/session` RPC: `{"session_id": "<id>"}` returns
  `{session_id, turns, records, requests}`; missing session_id → InvalidParams;
- record kind is a closed set (system/user/message/tool/compacted), and text fields over
  8 KiB are truncated with a `[truncated]` suffix.

## Boundary

- Token statistics and trajectory projection share a source (`run_events`), so their numbers
  should match; if they do not, treat the journal as authoritative and report a bug on the
  trajectory side.
