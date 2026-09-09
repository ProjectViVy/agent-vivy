# Symmetric Chrome Below the Input: shift+h Help

## Delivered

- With empty input, `shift+h` (usually reported by the terminal as uppercase `H`) opens Help, which is the former command/mode panel.
- The footer no longer displays `TUI` / `live` / the gateway address. The gateway address moved to the right column under `host ·`.
- permission / model / provider moved to the left directly below the input; `shift+h help` and `ctrl+x shortcuts` share the right side of the same row, aligned at half-widths.
- `ctrl+x` still opens the shortcuts overlay; in the session list, `ctrl+x` still deletes.

## Boundaries

- While typing, `H` remains a character and does not steal the draft.
- In a narrow window without a right column, the gateway address still appears in the compact top bar.
- When the approval diff is open, `shift+h` remains horizontal scrolling and does not open Help.
