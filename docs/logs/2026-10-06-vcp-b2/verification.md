# VCP B2 verification

## Focused suites

- `go test ./sdk/tui/...` — all green (command, face, live, stream, view).
- `go test ./internal/runtime/ -run 'TestDequeue|TestQueue|TestSteer|TestFollowUp' -v` — green, incl. new `TestDequeuePopsNewestFollowUp` (LIFO pop, lane tail, `turn.dequeued{reason:"dequeued", text}` markers).
- `go test ./internal/...` — green (app, domain, rpc, runtime, compaction).

## New tests

`internal/runtime/queue_test.go`:
- `TestDequeuePopsNewestFollowUp` — LIFO pop, empty-lane not-found, journal markers.

`sdk/tui/view/view_test.go`:
- `TestAltEnterSubmitsFollowUp`, `TestCtrlQSubmitsFollowUpAsAltEnterFallback`,
  `TestAltUpRestoresQueuedText`, `TestPlainEnterStillSubmitsNormally`.

`sdk/tui/live/controller_test.go` (rewrote two B1-era snapshots that pinned the
local FIFO; they now assert `turn/steer` wire params):
- `TestLiveFollowUpQueuesOnKernelWhileBusy`, `TestLiveFollowUpOnIdleStartsRun`,
  `TestLiveBusySendRoutesToSteerTrack`, `TestLiveDequeueRestoresNewestQueuedText`,
  `TestLiveQueueNoticeRefreshesLaneCounts`, `TestLiveAbortFlushRestoresDraft`,
  `TestLiveSettleAdmissionSubscribesNextRun`.

## Gates

- `gofmt -l internal/ sdk/` — clean.
- `TestCheckedInProviderConformance` — re-pinned internal-tree digest
  `34c60318…` → `a78057a2…` (queue.go / payloads.go / control.go changed).
- i18n key `vivy.tui.live.queueLanes` added to `catalog_en.go`,
  `catalog_zh.go`, and `scripts/i18n-cross-face-contract.json`.

## Acceptance (plan)

- Busy Enter → `turn/steer` wire call; steer echo rendered optimistically. ✔ (wire-level tests)
- Alt+Enter/Ctrl+Q → `turn/follow_up`; settles then admits next run. ✔
- Alt+Up → `queue/dequeue` → text restored to editor. ✔
- `/queue` shows both lanes; Esc/`/queue clear` flushes and restores newest text. ✔
- Manual in-TUI run deferred to the user's E2E gate (no provider key on this box).
