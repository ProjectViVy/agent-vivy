# TUI Markdown Typography (Crush Alignment)

## Delivered

- Assistant/user bubbles in full-screen chat no longer wrap Markdown as plain text. `sdk/tui/view` uses `github.com/charmbracelet/glamour` v1 (Charm v1 / lipgloss v1, not `charm.land/glamour/v2`) to render structure using the Vivy palette.
- The typography language aligns with Crush’s visual contract rather than being a source port: H1 pill, H2+ retaining the `##` prefix, `•` lists, `│ ` quotes, inline code chips, fenced chroma, link hierarchy, and QuietMarkdown + `┊` gutter for thinking blocks.
- The document layer removes Glamour’s default large blank spaces; completed messages are cached by session/width; the streaming bubble still fully renders the current message every frame, with cursor `▌` appended to the rendered final line.
- The model body still strips ANSI / control characters / bidi; if glamour fails, it falls back to the existing `wrapText`. Attachments and `@file` chips still display metadata only and do not enter Markdown parsing.
- Built-in `vivy tui`, standalone `vivy-code`, and packed `faces/tui` share this render path.

## Boundaries

- Crush’s streaming stable-prefix cache (`TUI-MD-STREAM-CACHE`) was not ported.
- Tool cards remain compact wrap + unified diff coloring (`TUI-MD-TOOL-RESULTS`).
- There is no Charm v2, custom chroma formatter, or theme switcher.
- `surface.Message`, the control plane, and Studio were not changed.
- This was not a release operation, so there is no `release.md`.
