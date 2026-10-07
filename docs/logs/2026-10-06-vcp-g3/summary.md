# G3 — TUI extras: search, prompt-jump, copy, editor, startup listing, OSC8, renderers

Story: `docs/superpowers/plans/vivy-code-parity/G3-tui-extras.md`
Commit: `feat(tui): transcript search, editor, renderers, startup listing`

## What shipped

- **Transcript search** (`search` action, `ctrl+f`): one-line query row that
  replaces the composer while open. Case-insensitive substring over message
  content + tool name/preview/result; `enter`/`tab` next match,
  `shift+tab` previous, wraps; `esc` restores the saved viewport
  (scroll + follow). Match counter `i/N` shown in the row.
- **Prompt-jump** (`prompt_prev`/`prompt_next`, `ctrl+up`/`ctrl+down`):
  viewport jumps to the previous/next user-message segment relative to the
  current top line; no-ops at the ends.
- **copy_last** (`alt+c`): keybound route into the existing `/copy`
  (OSC 52) implementation — no second clipboard path.
- **External editor** (`external_editor`, `ctrl+e`): writes the composer
  draft to a temp file, runs `tea.ExecProcess` on `$EDITOR` → `$VISUAL` →
  first of vi/nano (notepad on Windows), reloads the saved draft. Resolution
  and file round-trip are split into testable helpers; errors surface as a
  command overlay, never a panic.
- **Startup listing**: hero gains a counts line
  `N skills · N tools · N MCP · N sessions`. Only facts the sidebar
  projection reports are rendered (unknown ≠ zero). The `tools` count needed
  one narrow kernel addition: `session/sidebar` now returns
  `tools_known`/`tool_count` from the same `activeToolsFromOverlay` +
  `toolCatalog` resolution `tools/list` uses — a read-only field on an
  existing projection, flagged here as the one step outside the
  "face-internal" boundary.
- **OSC8 hyperlinks**: rendered transcript lines pass through
  `linkifyOSC8` — bare `https?://` runs become `\x1b]8;;URI\x07…\x1b]8;;\x07`.
  The URI is ANSI-stripped from the match so styled link text still yields a
  clean target; trailing `).,;` stays outside the link.
- **Tool renderer registry**: `view.RegisterToolRenderer(name, fn)` —
  case-insensitive; renderer returns the full card lines or nil to fall back
  to the default card. Deliberately a view-layer seam, not a plugin Port
  (documented; keeps the 14-Port catalog closed).
- All five actions are `keybindings.yaml` actions and appear in `/hotkeys`.

## Deviations

- `/` stays bound to the palette (pi's search key is Ctrl+F; `/` was already
  claimed locally — overriding it would break muscle memory from G2).
- "Filter" reads as locate-and-jump here: matches drive viewport jumps + a
  live match counter; per-line highlight/dimming of glamour-painted rows was
  descoped as low-signal for the cost (ANSI re-wrapping painted spans).

## Key chord additions

`search ctrl+f` · `prompt_prev ctrl+up` · `prompt_next ctrl+down` ·
`copy_last alt+c` · `external_editor ctrl+e`
