# Human acceptance

Start any real TUI path (`vivy-code.exe` interactive terminal, or `vivy tui`):

1. **F5 tool-card expansion**: Have the agent run a tool with long output (such as listing a directory or reading a file). The tool-card body is limited to 8 lines by default, with `… N more lines · ctrl+o expand` on the last line; after pressing `ctrl+o`, the same card shows the full output and the marker disappears; press it again to restore 8 lines. Users with `tui.debug: true` are unaffected (they always see the full output).
2. **F9 reasoning collapse**: Ask a question with a reasoning-enabled model. The reasoning section is fully visible by default (┊ vertical-bar style); after pressing `ctrl+r`, each reasoning section becomes a single line `reasoning · N lines · ctrl+r expand`, and the body is no longer visible; press it again to restore it.
3. **F13 empty-session hero**: Create a new session with `ctrl+n` (or start for the first time). The chat area shows the `Vivy™ VIVY CODE` mark, “Journey to Find Your True Heart”, the current working directory (when available), and key hints; the hero disappears after the first message is entered.
4. **Shortcut panel**: Open the panel with `ctrl+x`; it shows `ctrl+o tool output` and `ctrl+r reasoning` on two lines.
5. **No interference**: When an approval/question gate is open, pressing `ctrl+o`/`ctrl+r` does not change chat-body state; inside the `ctrl+s` sessions dialog, `ctrl+r` remains rename.

Items 1–3 are presentation-only behavior; Journal data and the context sent to the model are unchanged.
