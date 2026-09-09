# Acceptance

1. **View the session todo list**:
   - Open the Vivy Web UI (`http://127.0.0.1:3015`) and enter a session containing todo items.
   - Click the todo icon at the top or expand `SessionTodoPanel` in the right panel; the grouped todo list should display normally (current and historical todos).

2. **Check and switch status**:
   - When the session is idle (no active Run):
     - Click the unchecked Checkbox on the left of a todo item; its status immediately updates to `completed`, moves to the history list, and its text turns gray.
     - Click the checkbox for a completed todo in the history list to uncheck it; its status returns to `pending` and it moves back to the current list.
   - Click the cancel button that appears on hover on the right of a todo item (`Ban` icon); its status changes to `cancelled` and it displays a strikethrough.
   - Click the restore button (`RotateCcw` icon) for a cancelled item; the todo returns to `pending`.

3. **Safe Execution Guard**:
   - When the session triggers a new Run (generating an answer or executing a tool) or is busy:
     - The top of the panel displays a prominent “Locked during execution” badge.
     - All todo Checkboxes and action buttons are automatically grayed out and disabled (`disabled`), preventing human-induced concurrent write conflicts while the model advances the task.
     - If the model itself advances a todo through a tool call, the UI refreshes in real time to reflect the resulting state.

4. **Error handling and rollback**:
   - If a simulated network disconnect occurs or the server returns a conflict error, the frontend optimistic update is safely rolled back to the pre-change state and an error message is recorded.
