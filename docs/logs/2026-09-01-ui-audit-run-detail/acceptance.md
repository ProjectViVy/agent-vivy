# Acceptance

## Manual acceptance

1. Open `http://127.0.0.1:3015`, start or open a run with events, and expand
   Run Inspector on the chat side → "Current run": each event-list row (`#seq` +
   type) is clickable.
2. Click any event row: formatted JSON details expand below the row (indented,
   wrappable, and internally scrollable when too tall); click again to collapse;
   only one row is expanded at a time.
3. Reach an event row with Tab and press Enter/Space to expand/collapse it as
   well (native button); a screen reader can read the state from
   `aria-expanded`.
4. Event rows no longer have hover tooltips (`title` was removed); expanding an
   event with an empty payload displays `null`.

## Acceptance criteria

- `just ci` is green (tsc/eslint/vitest/build).
- The full `just ui-e2e` suite is green (the real-control-plane spec covers the
  chat page containing Inspector).
