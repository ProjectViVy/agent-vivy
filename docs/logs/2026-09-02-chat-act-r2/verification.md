# UI-CHAT-ACT R2 — verification

## Commands and results

| Command | Result |
|---|---|
| `gofmt -l internal/` | clean (rechecked after each round) |
| `go build ./...` / `go build ./internal/...` | OK |
| `go vet ./internal/storage/... ./internal/runtime ./internal/rpc` | OK |
| `go test ./internal/storage/... ./internal/runtime ./internal/rpc -count=1` (initial kernel) | sqlite 40.8s ok · postgres ok · runtime 117.4s ok · rpc 40.2s ok |
| `go test ./internal/runtime -run 'TestRewind\|TestFork' -count=1` (after effective-view fix) | ok |
| `go test ./internal/runtime -count=1` (full package after effective-view fix) | First run `TestServiceGrepToolEndToEnd` FAIL (order-dependent package flake); standalone `-count=5` ok; full package rerun ok (129.4s). Unrelated to this slice's diff (grep paths unchanged), same class as board item TEST-2; noted in §0.1 |
| `go test ./internal/storage/sqlite ./internal/storage/postgres -count=1` (after LatestViewTruncation, CN-21 includes new assertion) | sqlite 28.1s ok · postgres ok |
| `go test ./internal/rpc -run 'TestSession' -count=1` | ok |
| `just ui-e2e` | E2E-EXIT:0 (20 passed / 1 skipped, chat-act 3.9s; see below) |
| `just ci` | CI-EXIT:0 (see below) |

## Three rounds of offline e2e iteration (replay is the test)

The offline spec `ui/e2e/chat-act.spec.ts` (no provider → the turn fails but the user
message is recorded) exposed one real kernel defect in each of three rounds; each was
fixed and rerun:

1. **R1 open-ended folding defect**: the turn appended after rewind was permanently
   folded (the edit flow's own retry disappeared) → tail-anchor closed-interval
   `[cutoff, tail]` semantics (`migration020` adds `tail_message_id` in place).
2. **cutoff==tail fail-open**: when rewinding the last message, both anchors had the
   same ID, and the Go `switch` matched only one branch → folding silently failed → two
   independent `if` guards + a CN-21 case.
3. **fork resurrection + latest-wins resurrection**: fork copied from the stored list,
   bringing folded original text into the child session (`copied_count` 2 vs 1 exposed
   it) → changed to copy the effective view; then a second rewind under "latest marker
   wins" replaced the first rewind marker and resurrected text that had already been
   edited out (`remaining_count` 1 vs 0 after reload, with `hello vivy` reappearing) →
   the view now folds the **union** of all rewind/edit closed intervals
   `[cutoff, tail]` (`ListViewTruncations` + `ApplySessionTruncations`), and fork-anchor
   rows do not enter view folding.

Evidence for a fully green round after the fixes appears in the table above and the e2e/CI
entries below.

There was also one kernel-unrelated failure: the spec clicked "New session" before
`initialize()` settled; the automatic selection at the end of `initialize` overwrote the
new-session selection, and sending was silently swallowed (the DOM remained in its
pre-send state). This was a product-side boot-window race (<100ms, nearly impossible to
trigger manually). This round hardened the spec around it (wait for the first-session
selected signal before creating a session); the kernel was unchanged, and the product-side
fix has its own `UI-INIT-RACE` item.

## E2E

Final `just ui-e2e` run (run alone in the background, with no concurrent load):

```
Running 21 tests using 1 worker
  ✓ 3 e2e\chat-act.spec.ts:37:1 › edit reruns via rewind, rewind folds the view,
      fork copies history to a new session (3.9s)
  ✓ 8 e2e\files-panel.spec.ts:3:1 › files panel opens and shows the no-run empty state (1.3s)
  ✓ 17 e2e\runtime.spec.ts:5:1 › real control plane conversation, reload, review,
      settings and demos (2.9s)
  1 skipped
  20 passed (38.9s)
E2E-EXIT:0
```

Spec coverage chain: create session → send → banner (turn fails but message is recorded) →
`data-message-id` and other `^msg_` IDs (optimistic local IDs settled) → edit and save to rerun
 (old text folded, new text visible) → RPC assertion of the `session/messages` view → `session/fork`
 (copied_count=1, no discarded-history resurrection in the child session) → `session/rewind` (remaining_count=0)
→ folded view persists after reload → the history sidebar's "Fork branch" navigates to the new session without resurrecting the original text.

Two spec-side hardenings (not kernel changes): a settle wait (`article.first().or(开始新的对话)`
`toBeVisible`) avoids UI-INIT-RACE; the files-panel spec creates its own session to avoid
`currentRun` restoration from another session in the shared backend store.

## just ci

```
CI-EXIT:0
```

(fmt-check · ui-ci · vet · all Go package tests · headless-compile · plugin-ci
all green; log `/tmp/ci-chatact-r2.log`)

## Real-path smoke

The offline e2e is this round's real-path smoke test: a real backend (`go run ./cmd/vivy`,
8799) plus a real Vite-built UI; the full edit UI flow, rewind folding/forking driven
directly through kernel RPC, and session-list navigation were all browser-asserted. The
3015 development split was not repeated this round (same stack and code path).
