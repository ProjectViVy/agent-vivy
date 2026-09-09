# Restore the Wide-Screen Right Sidebar

## Delivered

- The right-pane display threshold was lowered from width 120 to 100, so common Windows Terminal sizes show the Crush-style right session pane again instead of collapsing it into top-bar compact mode.
- Overly wide rows using `padHorizontal` are truncated instead of pushing the right pane out of the terminal; the chat column adds `MaxWidth`.

## Boundaries

- Height must still be ≥30 to show the right pane. Shorter windows continue to use top-bar compact mode.
- Sidebar content and scrolling logic were not changed.
