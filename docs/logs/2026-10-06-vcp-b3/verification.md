# VCP-B3 verification

Environment: Linux VM, Go 1.26.8, Node 22 + pnpm, backend `go run ./cmd/vivy`
with `VIVY_USER_HOME=/tmp/vivy-b3` + frozen env provider
(`VIVY_PROVIDER=deepseek`, `VIVY_API_BASE=http://127.0.0.1:11434/v1`,
`VIVY_MODEL=mock-fast`), marker-routed `mock-llm.py` (hangs on `VIVY-HANG`,
instant text otherwise), UI `pnpm dev` at `127.0.0.1:3015`.

## Automated

- `go test ./internal/runtime/ ./internal/rpc/` — ok (45s / 16s).
- `go test ./sdk/internal/conformance/` — ok after digest re-pin.
- `go test ./sdk/internal/assembly/` — ok.
- `pnpm exec tsc -b --noEmit` — clean (Face compat + provenance checks).
- `pnpm exec vitest run` — 73 files / 580 tests green, incl. new
  `store.test.ts` queue block (+7) and `ChatInput.queue.test.tsx` (+5).
- `pnpm exec vitest run src/i18n` — 47 green (en/zh key parity).
- `gofmt -l` on touched Go files — clean.

## Live smoke (:3015, journal at /tmp/vivy-b3/vivy.db)

| Action | Observed | Journal |
|---|---|---|
| send `VIVY-HANG hold this run` | run active, busy buttons (follow-up / steer / cancel) | `run.started`, `model.request` |
| type + Enter | optimistic user bubble + `steer` chip | `turn.queued{track:"steer"}` |
| type + Shift+Alt+Enter | `queued` chip, 2 total | `turn.queued{track:"follow_up"}` |
| Alt+Up | newest chip gone, text in composer | `turn.dequeued{reason:"dequeued"}` |
| chip × | steer chip removed | `turn.dequeued{reason:"dequeued"}` |
| Clear queue | pill empty, text restored | `turn.dequeued{reason:"cleared"}` |
| Square ×2 (clear → cancel) | `Vivy cancelled`, draft restored | `run.cancelled` |
| send `hello` | `MOCK-REPLY`, run completed | `run.completed` |

Reload persistence: `session/get` now embeds `queue`; pill re-hydrates via
`selectSession → refreshQueue` (unit-tested path; live reload avoided —
recover sweep kills the hanging run by design).

## Known limits

- Steer injection at a turn boundary was not exercised live (mock hangs the
  in-flight call; `CancelAfterChatModel` waits for it to return). Queue-state
  truth for the steer lane was verified; the boundary-cancel mechanism itself
  is covered by B1 kernel tests and B2 TUI smoke.
- Attachment-bearing submissions stay on the local FIFO (text-only kernel
  queue) — verified by unit test; kernel queue is text-only by design.
