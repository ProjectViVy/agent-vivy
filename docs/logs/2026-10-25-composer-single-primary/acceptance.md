# Acceptance — composer single primary + child transcript leak

All checks run against the split pair: `just run` (control plane on
`127.0.0.1:8787`) and `cd ui && pnpm dev`, opened at
`http://127.0.0.1:3015`. If no provider key is configured, start the run with a
hanging mock endpoint so the composer stays busy while you test delivery.

## 1. Idle composer

- Exactly one primary button, labelled Send, at the right of the composer.
- Type text and press Enter: one user turn appears and the run starts.
- `Cmd/Ctrl+Enter` does the same.

## 2. Busy composer, empty draft

- While the run is active and the draft is empty the primary button is the red
  Stop button.
- Press `Escape`: the run is cancelled (no queue to clear).
- No second send button (no lightning/send-now, no clock/follow-up) is present.

## 3. Busy composer, queue mode (default)

- Type a message. The single primary button now reads Queue.
- Press Enter (or click Queue): the message appears in the queue pill above the
  input, not in the transcript; the running turn is not interrupted.
- `Cmd/Ctrl+Enter` instead sends it as a steer: it disappears from the draft and
  lands in the running turn at the next step (queue pill shows it under Steer
  until consumed).
- In the queue pill each item can be removed individually, and Clear queue
  empties it. `Escape` while items are queued clears the queue and does not
  cancel the run; a second `Escape` cancels.

## 4. Busy composer, steer mode

- Settings → General → Send while running → Steer → Save.
- The primary button label becomes Steer immediately, and plain Enter steers.
- `Cmd/Ctrl+Enter` now queues instead.
- Reload the page: the preference is still Steer.

## 5. Child agent leak

- Ask for a task that spawns a one-shot child agent (for example a delegated
  read/search task).
- The parent timeline shows the agent tool card and its result only: no child
  user/system bubbles and no child assistant text.
- The child's own transcript is still readable from the agent tool card /
  Run Inspector / `child/history`.
- Continuable children (those with their own session) still appear in
  Run Inspector with their own reconciled transcript.

## 6. Regression checks

- A failing run still reports the backend error detail in the conversation.
- The queue pill still shows `queued` and `steer` lane badges in the active
  language, and switching zh/en relabels them.
