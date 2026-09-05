# TUI composer chips — acceptance

Date: 2026-09-06

A human can tell this worked in `vivy-code` (or packed TUI) without
reading the diff:

| Check | What you should see |
|---|---|
| Rounded composer | The input is a box with `╭╮╰╯` corners, not a lone `─` rule above `:::` |
| Top-left chips | Small muted `model  ·  permission  ·  thinking` on the first inner row |
| Draft | `::: ` still prefixes the typed line |
| Shortcuts | `^l` / `^y` / `^t` still change model, permission, and thinking; the chips follow |
| Attachments | `/image` metadata stays inside the box as `[image: name]`, never a data URL |
| Compact terminal | Help row (`enter send`) remains on the last line at 30 rows |

Not in this slice: the web chat input, agent/plan mode, clicking chips.
