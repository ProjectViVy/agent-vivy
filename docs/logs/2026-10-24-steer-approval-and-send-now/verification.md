# Verification — steer survives approvals + send now

Venue: local macOS arm64 worktree `agent-vivy-steer-sendnow`
(branch `feat/steer-sendnow`). Toolchain pinned with
`GOTOOLCHAIN=go1.26.4`; the ambient Go (1.27.1) cannot build the pinned
`grpc@v1.64.1` (`http2.TrailerPrefix`).

## Gates

| Command | Result |
|---|---|
| `just wiring-ci` | PASS — 111 executed tests, 0 skipped |
| `just kernel-ci` | PASS — 1991 executed tests + minimal boot, 0 skipped |
| `just d02-ci` | PASS — 175 + 1 + 13 + 9 executed tests |
| `just d02-web-ci` | PASS — 107 UI tests + 2 pack tests |
| `go test ./internal/runtime -run 'Steer\|SendNow\|Approval\|Queue' -count=1` | PASS (11.8s) |
| `go test -race ./internal/runtime -run 'TestSteer…\|TestSendNow…\|TestSteerDuringSuspendedRunDemotesToFollowUp' -count=1` | PASS (18.9s, no race reports) |
| `go test ./internal/runtime -count=1` | PASS (73.8s, final tree) |
| `go test ./internal/rpc -count=1` | PASS (23.6s, final tree) |
| `go vet -tags=vivy_headless ./internal/runtime ./internal/rpc ./internal/domain ./sdk/tui/...` | PASS |
| `go test ./sdk/tui/... -count=1` | PASS (all packages) |
| `cd ui && pnpm typecheck` | PASS (after `pnpm build` staged `ui/dist`) |
| `cd sdk/ui && pnpm typecheck && pnpm test` | PASS — 29 tests |
| `gofmt -l` on changed Go files | clean |
| `git diff --check` | clean |

New kernel tests (`internal/runtime/queue_steer_sendnow_test.go`):

- `TestSteerSurvivesApprovalInterrupt` — steer stays in the lane across the
  approval interrupt, `turn.steered` fires once after the decision, and the
  resumed model input contains the steer text.
- `TestSteerCancelStillFlushes` — user cancel with a pending steer: terminal
  `run.cancelled`, no steer resume, flushed item journaled
  `turn.dequeued{reason:"aborted"}`.
- `TestSendNowInterruptsCurrentRun` — old run cancelled, flagged turn admitted
  with the send-now text, flag cleared on admission.
- `TestSendNowCancelsSuspendedRun` — send-now while the run is parked on an
  approval cancels the suspension and admits the flagged turn as the next run.
- `TestSendNowSurvivesReplay` — journal rebuild keeps the flagged item pending.

UI tests added: send-now button while running, Cmd+Enter precedence over the
steer branch, `sendNowMessage` submission shape; `sdk/ui` store/API surface
parity compiles.

## Full UI suite (pre-existing failures)

`cd ui && pnpm test`: 648 passed, 44 failed in 8 files
(`ConversationSidebar`, `session-list-view`, `WelcomeWizard`, four
`settings/*` cards, `reveal-engine`). Every failure is
`TypeError: Cannot read properties of undefined (reading 'clear'/'unmount')`
on `window.localStorage` — the ambient Node 26 global `localStorage` shadows
happy-dom. This is the open board item **UI-VITEST-NODE25**, not a regression
from this change; all files touched by this delivery pass (78 tests across
`ChatInput.queue`, `store`, `ui-sdk-face-compat`).

## Real-path smoke (split pair, not embedded)

Backend: `VIVY_ADDR=127.0.0.1:8788 VIVY_USER_HOME=/tmp/vivy-smoke-sendnow/home
VIVY_PROVIDER=deepseek VIVY_API_BASE=http://127.0.0.1:9911/v1 VIVY_MODEL=mock-fast
DEEPSEEK_API_KEY=anything go run ./cmd/vivy`; UI: Vite on `127.0.0.1:3016`
with `VIVY_BACKEND_ADDR=http://127.0.0.1:8788`. Ports 8787/3015 were already
held by an unrelated dev pair, so the smoke used 8788/3016. Fresh persona was
initialized through `/persona` in the temp home. A local OpenAI-compatible mock
held the first model call for 20 s and then emitted a `write_file` tool call.

1. Sent `VIVY-SLOW-TOOL create note.txt`; while the model call was in flight
   enqueued `STEER-MARK also mention the moon` (Enter = steer).
2. Approval appeared; the composer still showed `1 queued · steer ·
   STEER-MARK also mention the moon` (previously it was cleared).
3. Clicked **Approve**; the run wrote `note.txt`, then answered `ok`, and the
   steer appeared in the transcript as a user message.
4. Journal (`run_events`, read-only):
   `run.started(1) → turn.queued steer(3) → tool.approval_required(8) →
   tool.approval_decided(9) → turn.steered(16) → run.completed(21)`.
5. Recorded model requests: the post-approval call contains a user message
   `STEER-MARK also mention the moon` after the tool result.

Send-now:

6. Sent `VIVY-HANG hold on` (mock never answers); composer showed the running
   controls including the new `Send now: interrupt the current run
   (Cmd/Ctrl+Enter)` button.
7. Typed `SENDNOW fresh task` and clicked the Zap button. The old run reached
   `run.cancelled{reason:"user_requested"}` and the new run completed with
   `ok`; the composer returned to idle.
8. Journal: `turn.queued{track:"follow_up", text:"SENDNOW fresh task",
   interrupt_current:true}` → `run.cancelled` → new `run.started` →
   `run.completed`; the mock received `SENDNOW fresh task` in the new run's
   first request.

Scratch (`/tmp/vivy-smoke-sendnow`) and the smoke servers were removed/stopped
afterwards; the repository `data/` tree was never read or written.

## Not run

- `just ci` (full product, Windows lane) and `just vet` / `just fmt-check`
  recipes — they are PowerShell-only recipes on a Windows host; the equivalent
  scoped gates above ran on macOS.
- PostgreSQL conformance and Docker — untouched by this change.
- Full-package `-race` lane: only the new/changed queue tests ran under the race
  detector. A broader `-race` filter over every `Approval` test was stopped
  without completing on this host; the targeted lane above is the recorded
  evidence.
- TUI send-now affordance — out of scope (field decode only).
