# Verification

All commands ran in worktree `../agent-vivy-chatbox-buttons` (branch
`feat/chatbox-buttons`).

| Command | Result |
|---|---|
| `just ci` (fmt-check / vet / go test / headless-compile / ui-ci) | PASS (ui-ci: 21 files, 175 tests passed; vite build succeeded; chunk-size warning is pre-existing) |
| `pnpm typecheck` / `pnpm test` / `pnpm build` (ui) | PASS |
| `just ui-e2e` (Playwright starts a Go mock backend) | 6 passed / 2 failed; both failures also occur on a clean main HEAD (see below) |

## e2e failure attribution (not introduced by this lane)

Rerun on clean HEAD (2a77258) with `git stash -u` plus a rebuild:

- `runtime.spec.ts`: clean HEAD stops at the already-invalid `Drawing` assertion (the
  button does not exist in src); this lane removed that assertion and let the spec reach
  the Settings page. The remaining failure is "Keys are managed only by the runtime
  environment" (the Settings → Model tab assertion is stale, likely relative to
  model-list-sync).
- `welcome-wizard.spec.ts`: clean HEAD likewise fails at the "Next → Configure model"
  step.

Both are recorded in `docs/TODO.md` §0.1 **E2E-STALE**.

## Real-browser smoke (real browser, mock-provider backend)

Environment: `VIVY_ADDR=127.0.0.1:18787 go run ./cmd/vivy` in the worktree (temporary
mock config.yaml, deleted afterward) + `VIVY_BACKEND_ADDR=http://127.0.0.1:18787 pnpm
exec vite --port 3016`.
All items passed:

1. Initial DOM: `Create new session` is in the chat-box top bar; `Voice` and `Open
   companion` (desktop companion) are absent; the sidebar and snapshot have no create-
   session entry; Attachments / AutoDream / More / Thinking mode buttons remain.
2. Click `Create new session`: the session is created successfully, the History drawer
   list grows from 1 → 2 sessions, the new session is selected automatically, and no
   failure prompt appears.
3. Permission dropdown: Smart → Cautious; the trigger label and description switch
   (`session/set_permission` makes a real round trip).
4. Execution mode: selecting "Ask mode" shows "Ask mode is not connected yet" and keeps
   "Agent mode" selected; selecting "Plan mode" changes the trigger to "Plan mode".
5. Send a message in Plan mode: preflight returns **"Preflight blocked this run /
   task_create is unavailable in plan mode"** — mode reaches the backend and is blocked
   by the Plan policy (the old hard-coded normal behavior would not produce this).
6. Switch back to Agent mode and resend: the mock reply `mock reply to: smoke: agent mode
   message` renders normally (the full preflight → turn/start → streaming-reply path).
7. ShieldCheck `Approval Center`: the sheet opens with the title "Approval Center" and
   an empty list ("No approvals or issue records").

## Known unverified items

- The six passing e2e cases cover session / approval / Settings / demo paths; the
  Settings → Model and welcome-wizard cases did not complete because of the existing
  E2E-STALE failures (unrelated to this lane).
- Thinking-mode dropdown (no backend capability) was checked only for button retention,
  not behavior.
