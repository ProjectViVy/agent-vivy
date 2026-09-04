# VIVY CODE command palette

## Changed

- Added a filterable fullscreen command palette backed directly by the shared
  slash-command registry. Bare `/` and `Ctrl+P` open it; command names,
  aliases, usage, and descriptions all participate in Unicode-safe fuzzy
  filtering.
- Added wrapped keyboard navigation (`Up`/`Down`, `Ctrl+P`/`Ctrl+N`), bounded
  scrolling, a no-results state, and `Enter`/`Ctrl+Y` selection.
- Treats the filter as untrusted terminal data: ANSI/control sequences are
  removed, input is bounded to 128 runes, physical `KeySpace` is supported,
  and terminals below 16 rows use a compact single-line list that keeps the
  navigation footer visible.
- Selection inserts the canonical command into the editor and deliberately
  reuses the existing parser, validator, permission/busy gates, confirmations,
  and dispatcher on the next Enter. Parameterized or destructive commands do
  not gain a bypass path.
- Preserved `//text` as the existing literal-slash escape even though the first
  slash now opens the palette.
- Kept interaction priority explicit: approval/question gates, the sessions
  picker, and command confirmation/result overlays cannot be preempted. An
  asynchronously arriving gate or command result closes the palette instead of
  being hidden behind it.
- Implemented the feature once in `sdk/tui/view`; built-in and packed faces use
  the same model and have wrapper smoke coverage.

## Explicitly not done

- The registry currently contains authoritative built-in commands only.
  Dynamically user-invocable skills and MCP prompts need a typed catalog before
  they can safely become command rows; that follow-up is tracked in
  `TUI-CMD-N3`.
- The line-oriented legacy REPL cannot provide a key-driven popup. Its slash
  parsing and `//` behavior remain unchanged.
