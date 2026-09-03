# VIVY CODE slash commands

## Delivered

- Added the shared `sdk/tui/command` parser and registry used by the
  built-in and packed terminal faces.
- Preserved ordinary input byte-for-byte, added `//` literal-slash escaping,
  Unicode/emoji-safe single- and double-quoted arguments, backslash escapes,
  syntax errors, aliases, and local unknown-command errors.
- Routed fullscreen Enter through the shared parser. Local help/status/errors
  render in the Bubble Tea overlay; they never write directly to stdout.
- Preserved ordinary turn text through the live drivers without trimming;
  malformed/unknown command drafts remain editable after dismissing the
  local error, and asynchronous command results are rendered rather than lost.
- Added the first command slice: `/help`, `/?`, `/commands`, `/status`,
  `/sessions`, `/new`, `/session`, `/rename`, `/delete` (confirmation),
  `/cancel`, `/queue clear`, `/permission`, `/quit`, `/exit`, and `/q`.
- Added `surface.CommandExecutor` and `CommandResultMsg`; both live drivers
  implement the same fail-closed command adapter.
- Routed the plain REPL through the same parser and added an explicit
  y/n confirmation barrier for `/delete`.

## Explicitly not included

The advanced command families (`/compact`, `/fork`, `/rewind`, `/todos`,
`/stats`, `/skills`, `/mcp`) are not exposed until each has a complete TUI
result surface and truthful session-scoped semantics. They remain tracked in
`docs/TODO.md`.
