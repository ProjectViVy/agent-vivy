# UI-CHAT-ACT R2 — session/fork kernel + UI edit/rewind/fork wiring

## What changed

R1 (80fe3b6) shipped the `session/rewind` kernel (truncation markers, read-time folding, controller).
R2 completes the other half according to `docs/architecture/JOURNAL-REWIND-AND-FORK.md`:
the `session/fork` kernel + all three UI buttons wired, bringing the main UI-CHAT-ACT work to closure.

### Kernel (fork = copy history to a new session; original session unchanged)

- **Tail-anchor correction (e2e offline replay exposed an R1 design defect)**: the initial folding implementation used an open interval in which "everything after cutoff
  is voided", so turns appended after rewind were folded out as well—the retry in the edit flow
  (rewind + resend) disappeared permanently, appearing on 3015 as an empty view after resending.
  Fix: add `tail_message_id` to `session_truncations` (migration020 has not landed in any
  released DB, so this is an in-place extension), and change folding to hide the **closed interval `[cutoff, tail]`**
  (the session's last message at the time the marker was recorded); turns appended after rewind are outside tail and remain visible.
  The design document §2.1/§2.2/§8-Q1 has been signed off again (unknown cutoff/tail fail-open remains unchanged).
  CN-21, rewind_service, and the control route each gained a regression assertion that turns appended after rewind remain visible; CN-21 also adds a cutoff==tail case
  (when rewinding the last message, both anchors have the same id and must be parsed independently, or folding silently fails—the second folding defect caught by the second e2e round).
- **Effective-view correction (exposed by e2e offline replay)**: fork initially copied message rows from the raw stored list
  `≤ fork point`—forking again after the edit flow (rewind + resend) resurrected the original text that rewind had folded out of the child session's context.
  Fix: locate rewind/fork cutoffs and copy for fork entirely from the **effective view** (after truncation markers are folded)—the child-session context = the visible context at the fork point in the original session;
  requesting rewind/fork for an already folded message → `ErrInvalidCutoff` (aligned with the "already behind the effective truncation
  point" wording already present in the ErrInvalidCutoff comment); `remaining_count` now likewise uses a view-relative count. Design document §3.3
  step 3 has been signed off again. Regression: `TestRewindAndForkRespectEffectiveView` (two negative cases for hidden messages + no resurrection on copy + view-relative remaining) + e2e guard assertion against child-session resurrection.
- **Union-folding correction (caught by the third e2e round)**: the view initially selected the "latest rewind/edit marker
  wins"—but the edit flow itself is rewind + new turn + possibly another rewind, and a second rewind
  (for the retry message) under latest-wins displaced the first marker, resurrecting the original text
  that had been edited out (e2e reload reproduced `hello vivy` and exposed it). Fix with **union folding**: the view =
  the stored list minus the union of all rewind/edit closed intervals `[cutoff, tail]` for the session;
  `TruncationStore` read semantics are split—`LatestSessionTruncation` (audit read, latest for any reason)
  is unchanged; add `ListViewTruncations` (view read, returns all rewind/edit markers in insertion order) +
  `storage.ApplySessionTruncations` (interval-union mask), with single-marker `ApplySessionTruncation` becoming a thin wrapper.
  Both `effectiveSessionMessages` and the `session/messages` folding point (control.go) now use the union. CN-21 adds assertions for "a late-written
  fork anchor does not enter view folding, successive rewinds accumulate, and rows appended between markers form a non-contiguous union";
  runtime adds `TestSuccessiveRewindsAccumulate`. Design document §2.1/§2.2 signed off again.
- `internal/runtime/rewind_service.go`:
  - `ForkSession(ctx, sessionID, messageID, title)`: busy gate (an active
    run in the same session → ErrSessionBusy) → `sessionViewCutoff` locates the cutoff in the effective view (missing →
    ErrInvalidCutoff) → new session (`sess_` prefix; default title = source title + " (fork)",
    localized copy passed by the UI) → copy effective view `[:cutoff+1]` (including the cutoff) to the child session →
    bidirectional provenance markers: parent session `fork` (`fork_session_id` points to the child), child session
    `forked-from` (points back to the parent, with the cutoff recorded as the child-side copy id) → via `RecordExternalRunEvent`
    record `session.truncated` (reason=fork) and `session.forked` (parent_session_id + cutoff) on a synthetic `tr_` run.
  - Design deviations (signed off in the design document): copied messages are minted with entirely new `msg_` IDs (messages.id is a
    globally unique primary key, not `(session_id,id)`); the child session's forked-from marker points to the child-side copy id for the cutoff;
    the title fallback uses the " (fork)" suffix (region-neutral), while the UI passes a localized title.
  - `RewindSession` was refactored to share the `rejectBusySession` / `sessionViewCutoff` helpers.
- `internal/storage/contracts.go`: added `TruncationFork`/`TruncationForkedFrom`
  reason constants; inverted the `ApplySessionTruncation` decision—only rewind/edit are filtered,
  fork/forked-from are pure provenance anchors and are never folded, and unknown future reasons are likewise not folded (fail-safe).
- `internal/domain/event.go`: added `EventSessionForked` (`session.forked`) to the vocabulary.
- `internal/rpc/control.go`: `session/fork` method (`{session_id, message_id,
  title?}` → `{session_id, fork_point_message_id, copied_count}`), error mapping
  consistent with rewind (busy → -32009, missing cutoff → -32004); append
  `session.fork` to capabilities; connect folding to `listMessages` (`Truncations` injected).
- `internal/app/app.go`: inject `Truncations` at both composition sites.
- Tests: `rewind_service_test.go` (full ForkSession flow: copied count / inherited
  SandboxMode+ApprovalPolicy / bidirectional markers / `session.forked` event / negative busy+missing-cutoff cases;
  regression after Rewind shared the helpers), `conformance/suite.go` CN-21 (latest-wins folding +
  fork/forked-from pass-through + stale-cutoff fail-open), `control_test.go`
  TestSessionForkRoute (end-to-end RPC including envelope assertions) + TestSessionRewindRoute,
  `domain` vocabulary guard 38.

### UI

- `ui/src/lib/api.ts`: `rewindSession`/`forkSession` fetchers + method-table registration.
- `ui/src/lib/store.ts`: `rewindSession` (refetch context + message view after the RPC),
  `forkSession` (refetch the session list after the RPC, return the new id).
- `ui/src/components/chat/MessageBubble.tsx`: enable "Edit" on user bubbles — an in-place
  Textarea with save (`chat.editSave`) and cancel; enable "Rewind to here" and "Fork from here" on assistant bubbles, both with AlertDialog confirmation (`rewindConfirm*`/`forkConfirm*`), running
  disabled as before. Remove the old `chat.pending` placeholder button.
- `ui/src/components/chat/ChatView.tsx`: `handleEdit` = rewind + `submit(new text)`
  (the kernel does not automatically resend; resend semantics live in the UI layer); `handleRewind` = after rewind,
  prefill the input box with the most recent user input still in context; `handleFork` = fork + navigate to the new session;
  action failures are shown through RecoverableError (retry only clears the error).
- `ui/src/components/chat/ChatInput.tsx`: `draftPreset` prop (write the draft and focus when seq changes,
  with a ref preventing repeated application of the same seq) carries the rewind prefill.
- `ui/src/i18n/en.ts`/`zh.ts`: bilingual `chat.editSave`/`editCancel`/`rewindConfirm*`/
  `forkConfirm*`.

## What was explicitly not done

- Rewind/fork on tool-message bubbles (buttons only appear on assistant text bubbles; tool results remain collapsed as before).
- Queued messages and actions in running sessions (the busy gate rejects them on the kernel side; the UI remains disabled as before).
- R3 polish items (localized fork-title passing, child-session trajectory empty-state copy, etc.) handled separately as budget permits.
- `initialize()` boot-window race (clicking "New session" before settlement is overwritten by the automatic tail selection,
  exposed by e2e replay): this round works around it with the spec's wait signal; a product-side fix is tracked separately as `UI-INIT-RACE`.

## Board

`docs/TODO.md` UI-CHAT-ACT row flipped to DONE (R1 kernel + R2 wiring/replay fully delivered;
edit/rewind/fork three buttons implemented), logged in §10.
