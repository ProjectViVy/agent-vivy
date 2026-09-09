# TUI Tool Output DEBUG Mode

## Delivered

- Added the `tui.debug` configuration, defaulting to `false`.
- In the default mode, the body of a completed tool card is limited to 8 terminal line breaks, with the number of omitted lines and the way to enable them displayed.
- `tui.debug: true` shows the complete tool result; terminal control-character cleanup and width constraints still apply.
- Standalone `vivy-code`, in-process `vivy tui`, remote `vivy tui --live`, and the packaged TUI face share the same renderer behavior.

## Boundaries

- The actual tool return, Journal data, and context sent to the model are not truncated; this is a display-only configuration.
- Tool protocols, Eino orchestration, and Studio were not modified.
- This was not a release operation, so there is no `release.md`.
