# TUI Slash Command Palette: Crush-style Typography + Chinese Prompts + Highlighting

## Delivered

- The `/` and Ctrl+P command palette now use a one-line menu with a short `/name` and Chinese title, rather than using full English usage as the title.
- The selected row has a full-row background; only the current row expands its description and full syntax.
- The filter query applies subsequence highlighting (bold, underline, secondary color) to `/name` and the title.
- When skills/MCP are present, entries are grouped into `System` / `Skills` / `MCP` sections.
- Built-in command descriptions, the `/help` body, footer, and session/model/file/confirmation dialogs are temporarily unified in Chinese. Command names and Usage remain inputtable English identifiers.

## Boundaries

- No i18n framework or English fallback toggle.
- Descriptions provided by skill/MCP servers retain their original language.
- Sidebar section titles were not translated in this round.
- Crush Tab switching and `sahilm/fuzzy` were not introduced.
