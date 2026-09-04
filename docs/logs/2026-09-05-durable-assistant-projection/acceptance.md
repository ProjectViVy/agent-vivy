# Acceptance

1. Stream a long Unicode assistant answer: it renders continuously, survives reload byte-for-byte, and no durable event exceeds the configured payload ceiling.
2. Produce assistant text before a tool call: after the tool completes and after app restart, history shows the preamble, tool call/result, and final answer in order.
3. Start the next turn: the model context contains those same projected rows without duplicates.
4. Replay the run repeatedly or read `session/messages` after an interrupted projection: history converges to one copy of every row.
5. Replay an old v1 `model.completed` journal: its completion-only text remains visible.
6. Delete a session while its model is still running: deletion returns without any later event/message recreation, and that deleted id cannot accept another run in the same process.
7. Corrupt a v2 completion digest or byte count: TUI and both headless faces surface a protocol failure instead of reporting a successful answer.
