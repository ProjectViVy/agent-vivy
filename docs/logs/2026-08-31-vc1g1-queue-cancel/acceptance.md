# Acceptance — VC-1g-1

How a human can confirm that message queuing + two-stage cancellation works (requires a run from a real provider):

1. Open `http://127.0.0.1:3015` and send a message that keeps the agent busy for a while (for example, ask it to read several
   files and summarize them).
2. While the run is active (before streaming output ends), type directly in the input box and press Enter:
   - the input box is **not disabled**;
   - the message does not disappear or send immediately; a "1 queued" pill appears above the composer, and the chip shows the original text.
3. Queue another message → the pill count becomes 2; click the chip's ✕ to remove one; click "Clear queue" to remove all.
4. After the current run completes normally, the first queued message is **sent automatically** (a new user bubble and new streaming output appear),
   and the pill count decreases until it disappears.
5. Two-stage cancellation: start another run and queue one message →
   - the first press of Stop (or Esc in the textarea) clears the queue while the run **continues**;
   - the second press of Stop (or Esc) cancels the run.
6. Failure path: make the run fail (for example, disconnect the network/use a bad key), confirm the queue pill remains and the message is not discarded,
   then clear it manually or wait for automatic dispatch after the next successful run.
7. Return after switching sessions: the queue is empty (it does not cross sessions).

After switching between Chinese and English, the pill and button copy follows the selected language (for example, "N queued").
