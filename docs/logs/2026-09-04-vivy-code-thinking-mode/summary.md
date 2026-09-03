# Summary

VIVY CODE now exposes the runtime's real `auto` / `on` / `off` extended-
thinking preference in every terminal face.

- Added the shared `/thinking [auto|on|off]` command and `Ctrl+T` cycle.
- The shortcut and sidebar state appear only when the authoritative
  `session/context.thinking_supported` capability is true.
- The selected draft preference is sent as `turn/start.thinking` by the
  built-in fullscreen driver, packed `faces/tui`, and line REPL.
- Queued turns snapshot the preference at enqueue time. A later toggle cannot
  retroactively change queued work.
- Loading a session whose active model does not support thinking resets a
  stale forced-on preference to `auto`. The REPL rechecks capability before a
  forced-on send and degrades visibly if support changed.
- New sessions now fetch `session/context` immediately, so supported models do
  not lose the selector after Ctrl+N.

Not done: model selection has no session-scoped authoritative RPC and remains
hidden. Image attachment, split diff, and richer token/cost controls remain in
`TUI-PARITY-2`.
