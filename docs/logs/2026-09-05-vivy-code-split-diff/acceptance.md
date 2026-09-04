# Acceptance

1. Trigger a governed `write_file`, `patch`, `multiedit`, Skills mutation, or symbol replacement that requires approval.
2. Confirm the approval shows the server preview and risk metadata, not serialized tool arguments.
3. On a terminal wide enough for the computed dialog itself to reach 140 columns (normally about 175 terminal columns), confirm the preview opens as two aligned columns with old/new line numbers. On a narrower terminal, confirm it defaults to unified diff without overflow; `t` must still switch modes.
4. Press `t` to switch views and `f` to enter/leave fullscreen. Use arrows/Page Up/Page Down and Shift+H/L (or Shift+Left/Right) to scroll.
5. Confirm Enter, Ctrl+Y, or `y` approves; Esc or `n` denies; while submission is pending, other input does not reach the editor.
6. Trigger a non-file approval whose preview happens to contain diff-like text and confirm no split/unified controls appear.
7. Repeat with both the built-in `vivy-code.exe` face and a packed TUI face; the surface and interaction must match.
8. Put a credential-shaped canary in a proposed diff/target/risk and confirm the Approval row, Journal event, replayed TUI, and plain REPL contain only the redaction marker.
