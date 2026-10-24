# Acceptance — steer survives approvals + send now

## Defect 1: guidance is not lost when an approval fires

Setup: a running session with a model that returns an approval-gated tool call.

1. Send a prompt that leads to a tool call (e.g. ask for `note.txt`).
2. While the run is still working, type guidance in the composer and press
   **Enter** (this is steer). The composer shows `1 queued · steer · <text>`.
3. When the approval card appears, the queued steer is **still listed**.
4. Approve the tool call.
5. The tool executes, the run continues, and the guidance appears in the
   transcript as a user message; the assistant's next reply reflects it.

Before this change the queue pill vanished at step 3 and the model never saw
the guidance.

## Defect 2: direct send while a run is active

1. Start a run that stays busy (any long tool/model step).
2. Type the next message.
3. Click the amber **lightning** button next to the composer
   (tooltip: *Send now: interrupt the current run*), or press
   **Cmd+Enter / Ctrl+Enter**.
4. The current run is cancelled and the message you just typed starts a new
   run immediately — it is not queued behind the old one.
5. If the session is idle, the same action simply starts a normal run.

The other affordances are unchanged: **Send** (Enter) steers inside the run,
**Clock** (Shift+Alt+Enter) queues a follow-up, **Square** cancels.

## Notes

- A steer typed **after** the run is already suspended on an approval still
  becomes a follow-up, by design: the suspended run has no model boundary left
  to accept mid-run guidance.
- A send-now queued while a run is active is durable: it is journaled with
  `interrupt_current`, kept across the cancelled terminal, and preserved by a
  journal rebuild. If the backend dies before that terminal settles, the item
  stays queued until the next admission/settle trigger rather than draining at
  startup (`QUEUE-IDLE-RESTART-DRAIN`).
- Send-now carries text only. A submission with attachments or references falls
  back to the local FIFO (sent after the current run settles) instead of
  interrupting (`QUEUE-SENDNOW-ATTACHMENTS`).
- `turn/send_now` is available over RPC for non-UI faces; TUI exposes the field
  but not yet a key binding.
