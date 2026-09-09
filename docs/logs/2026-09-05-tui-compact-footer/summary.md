# Simplified Footer: shift+tab Mode / ctrl+x Shortcuts

## Delivered

- The footer no longer lays out a row of `^s ^p enter y/n…`. By default, only `shift+tab mode`, `ctrl+x shortcuts`, and the status remain.
- `shift+tab` enters command mode (the command palette).
- `ctrl+x` opens the shortcuts overlay, listing the original shortcuts; `esc` / pressing `ctrl+x` again closes it.
- During approval, the footer changes to `y/n approve`; in the session list, `ctrl+x` still deletes.

## Boundaries

- `/` and `ctrl+p` can still open the command palette.
- It was not made into a prefix chord (press ctrl+x, then a single key to execute).
