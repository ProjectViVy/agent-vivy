# TUI Mode Cycle, Intensity, and Color Chrome

## Delivered

- With empty input, `shift+tab` cycles Smart → Plan → Read-only. Smart/Read-only change permission (smart/cautious); Plan uses the existing `turn/start.mode=plan`. trusted remains under Ctrl+Y.
- The model name displays thinking intensity: `on` → `(high)` (gold), `auto` → `(auto)`. No new effort RPC.
- Below the input on the left: `model(high) · provider  42%`; the percentage is colored by usage (green / gold / rose). No upper bound is invented for an unknown window.
- On the right: `shift+tab Smart` plus Help/Shortcuts.
- Full-screen TUI defaults to TrueColor and downgrades only for `NO_COLOR`. Contrast is strengthened between user messages and the left gutter/right-column host.

## Boundaries

- `shift+tab` does not switch modes while busy or while typing.
- When the command palette is open, `shift+tab` still acts on the previous row.
- No new high/medium/low kernel control was added.
