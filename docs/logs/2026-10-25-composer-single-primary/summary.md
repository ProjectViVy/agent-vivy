# Composer single primary + child transcript leak

## Problem

Two separate defects surfaced in the same chat surface.

1. The composer offered four buttons while a run was active (send-now, follow-up,
   steer, and a destructive button that meant clear-queue or cancel depending on
   queue state), and plain `Enter` defaulted to steer. The reference product has
   one primary button and one durable decision: queue the message behind the
   current turn, or steer it into the next step of the current turn. It has no
   "send now / cancel and restart" action.
2. One-shot child agents reuse the parent session id, so the activity projection
   wrote their messages into the parent session's derived store, and every
   transcript read rendered them inside the parent conversation.

## What changed

### One primary send/stop button

- `ui/src/components/chat/ChatInput.tsx`: one primary button. While a run is
  active and the draft is empty it is Stop (`chatInput.cancelRun`); otherwise it
  is Send whose label and target track come from `busy_enter` (`Queue` default,
  or `Steer`). Plain `Enter` and the primary click use the preference;
  `Cmd/Ctrl+Enter` inverts it; `Shift+Alt+Enter` no longer submits;
  `Shift+Enter` stays a newline; `Alt+Up` still dequeues; `Escape` still clears
  the queue first, then cancels. The queue pill is now the only clear-queue
  entry point.
- `ui/src/components/chat/ChatView.tsx`: passes `settings.busy_enter` and drops
  the send-now wrapper.
- `ui/src/components/settings/ComposerSettingsCard.tsx` (+ test): Queue/Steer
  radio card in Settings → General, saved through `settings/update`, disabled on
  `read_only`/`frozen`.

### Queue pill: send queued now

Requested after the first cut and implemented as a UI composition, deliberately
without restoring the backend `send_now` verb:

- `ui/src/lib/store.ts` `sendQueuedNow()`: one `queue/clear` call supplies the
  exact texts (steering lane first, then follow-ups), the active run is
  cancelled, `waitForRunSettled` waits for the store to observe the terminal
  state (so every prior `turn.dequeued` has been applied), then the messages are
  re-sent in order — the first starts a new run, the rest return to the
  follow-up lane (`kernelEligible` submissions with attachments/references go
  back to the local FIFO). If the run never settles, the collected messages go
  back on the queue instead of being lost. `suppressClearedRestore` keeps the
  `cleared` dequeue reason from restoring the texts into the composer, since
  they are immediately re-sent.
- Session-switch race: if the user selects another session mid-flush, the texts
  (already cleared server-side) are re-dispatched to the original session and
  attachment/reference items are buffered until the user returns; `selectSession`
  also resets `runBusy` so the new session's composer is not left disabled.
- `ui/src/components/chat/ChatInput.tsx`: `chatInput.sendQueuedNow` button in the
  queue pill, immediately left of Clear queue, disabled while the flush is in
  flight. The queue pill remains the only place these actions live, so it does
  not reintroduce a second primary button.
- `sdk/ui/src/module.ts` mirrors `sendQueuedNow` on `FaceClientStore`; catalogs
  and `scripts/i18n-cross-face-contract.json` carry `chatInput.sendQueuedNow`.

### Durable `busy_enter`

- `internal/app/settings/settings.go`: `busy_enter` field, `BusyEnterQueue` /
  `BusyEnterSteer`, `EffectiveBusyEnter()` (empty means queue), `IsZero`, and
  `Validate` rejecting anything else. `internal/app/settings/settings_test.go`
  covers default, round-trip and rejection.
- `internal/rpc/control.go`: `settings/get` emits the effective value;
  `settings/update` takes `busy_enter` and only overwrites when present, so
  older clients keep the stored mode, and echoes the effective value (found in
  the smoke: the response omitted the field, so the card lost its selection
  after Save). `TestSettingsGetAndUpdate` covers default, update, invalid value,
  omitted field and the echo.
- `sdk/ui/src/module.ts`: `FaceBusyEnter`, `FaceSettings.busy_enter` (optional),
  `FaceSettingsUpdate.busy_enter`.
- `ui/src/lib/api.ts` / `store.ts`: `BusyEnter` aliases the SDK type,
  `Settings.busy_enter?`, `SettingsUpdate.busy_enter?`, and
  `settingsUpdateFrom` always emits the current or default value.

### `send_now` removed end to end

- Runtime: `Service.SendNow`/`SendNowWithOptions` deleted;
  `enqueueTurn` loses its `interruptCurrent` parameter and the front-of-lane
  insertion branch. RPC `turn/send_now` and `sendNowTurn` deleted.
- UI/SDK: `api.sendNowTurn`, `store.sendNowMessage`, `FaceClientAPI.sendNowTurn`,
  `FaceClientStore.sendNowMessage`, the Zap button, the follow-up Clock button,
  and the `chatInput.sendNow` / `chatInput.sendAsFollowUp` catalog keys removed;
  `chatInput.steer` is now `Steer`/`插话` and the queue lanes read
  `steered`/`queued` (`插话`/`排队`).
- `InterruptCurrent` stays as a decode-only field: `domain.QueuedTurn`,
  `payloadTurnQueued`, the cancelled-terminal survivor drain (`service.go`),
  queue-request sanitization, the SDK wire field and the TUI decode field remain
  so a legacy journal replays its flagged item instead of dropping it. No
  producer sets the flag. `TestLegacySendNowJournalReplaysAsFollowUp` pins that
  boundary; the two producer tests were deleted.

### Child transcript leak

- `internal/runtime/message_projector.go`: reconciliation skips
  `RunKindChild` runs whose `EffectiveChildMode()` is one-shot (empty mode reads
  as one-shot). Continuable children keep their own session and are reconciled
  there.
- The child-id selection was extracted into `childRunIDs(ctx, sessionID,
  oneShotOnly)` plus `dropRunMessages`, shared with `parentFeedMessages` (which
  continues to drop every child row from the model feed).
- `Service.SessionViewMessages` lists stored messages and drops one-shot child
  runs, failing open on run-listing errors like the existing filter.
- New call sites: RPC `session/get` and `session/messages`
  (`control.go` `sessionViewMessages`), `session_copy.go` export, and both
  `rewind_service.go` read points (fork and cutoff). Per-run views
  (`trajectory.go`), title/reference/deliverable/channel/cron paths are
  unchanged: they are run-keyed and already omit child user rows.
- Tests: `TestMessageProjectorSkipsOneShotChildRuns`,
  `TestSessionViewMessagesDropsOnlyOneShotChildRows`,
  `TestControlTranscriptReadsHideOneShotChildRows`, and the existing
  `child_feed_test.go` assertions now distinguish one-shot from continuable.

## Explicitly not done

- No backfill or deletion of historic one-shot child rows. The derived store is
  append-only and has no delete API; those rows are now invisible to every
  transcript read but still occupy storage. Boarded as
  `CHILD-ONESHOT-ROW-BACKFILL` in `docs/TODO.md`.
- No per-item "steer this queued message" action, and no empty-draft
  "steer the whole queue" gesture. "Send queued now" flushes the whole queue;
  it does not pick one item.
- The backend `send_now` producer stays deleted: the flush is a UI composition
  over `queue/clear` + `run/cancel` + `turn/start`/`turn/follow_up`, so the
  kernel keeps one admission path.
- No change to the approval-policy work delivered earlier in the same tree.

## Incidental fixes required by the gates

- `ui/src/lib/store.test.ts` expected the old localized `run.failed` line, but
  `f0b9f1ff` made the backend detail authoritative; the stale expectation is
  updated to the message the runtime now emits.
- `scripts/check-i18n-completeness.js`: the three model-service status markers
  added in `f0b9f1ff` are protocol recognition tokens; they join the existing
  `failure.ts` exact-value allowlist.
- `ui/node_modules/@vivy/ui-sdk` is a pnpm file-dependency copy; `pnpm install`
  is required after editing `sdk/ui/src` or the UI typechecks against stale SDK
  types.
