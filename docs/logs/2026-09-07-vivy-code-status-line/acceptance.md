# Acceptance (human verification steps)

Prerequisite: from the repository root, run `just tui` (build and start
vivy-code.exe, then enter the VIVY CODE terminal for the current project).

1. **F1 spinner + elapsed time**: Send a message that triggers an agent run.
   On the status line (below the input), the left side should show a braille
   spinner (⠋⠙⠹… cycle) + `run` + a stopwatch advancing by the second (e.g.
   `⠸ run 23s`; after one minute it becomes the `1m03s` form). The spinner
   disappears when the run ends.
2. **F12 queue count**: Send another message while the first is running so it
   enters the queue. The right side of the status line should be right-aligned
   as `queued 1` (changing with the queue count); the text disappears when the
   queue is empty.
3. **F6 scroll indicator + end to bottom**: Scroll the chat area upward with
   PgUp / the mouse wheel. The right side of the status line shows
   `↓ <percentage>% · end to bottom`; the percentage decreases as you scroll up. After
   pressing the `End` key to return to the bottom, the indicator disappears;
   it is always hidden at the bottom.
4. **Regression**: When an error occurs, the status line still starts with
   `err · …` (higher priority than the spinner); the top title bar and the
   right sidebar's `queue · N` row look unchanged; when the window is resized,
   the status line remains one line and does not squeeze the chat area.

All conditions must be met for acceptance.
