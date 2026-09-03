# Acceptance

From a normal interactive terminal in a project directory:

1. Run `vivy tui` with the usual Vivy config/provider environment.
2. Confirm the visible product title is `VIVY CODE`, with purple/cyan/green/yellow/rose terminal color instead of a monochrome shell.
3. Send a coding request and confirm it creates a real `face=code` run against the current project, streams assistant/reasoning output, and displays real tool calls/results.
4. For an edit result, confirm added lines are green, removed lines are rose, hunk headers are cyan, and `+N -N` counts are shown.
5. Press `Ctrl+Y` and confirm the session permission cycles cautious → smart → trusted. Send another message while a run is active and confirm it is queued; Escape first clears queued messages and then cancels the active run.
6. Confirm `vivy tui --demo` is the only route to offline fixture data.

No Studio launch or Studio modification is part of this acceptance path.
