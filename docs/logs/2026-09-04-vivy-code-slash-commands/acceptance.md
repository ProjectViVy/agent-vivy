# Acceptance

## Fullscreen face

1. Start `vivy-code` in a real terminal.
2. Enter ordinary text; it is sent unchanged.
3. Enter `//hello`; the model receives `/hello`, not a local command.
4. Enter `/help` or `/commands`; a local Bubble Tea command overlay appears
   and no raw stdout line is emitted outside the TUI.
5. Enter `/status`; the overlay reports the active session/run state.
6. Enter `/sessions`; the existing Crush-style Sessions picker opens. Session
   rows are not rendered in the right rail.
7. Enter `/new "中文 标题 🙂"`, `/session <id>`, `/rename "新标题"`,
   `/permission smart`, `/queue clear`, or `/cancel` and observe the existing
   live driver/controller path. While a run or gate is active, session-state
   mutations fail closed.
8. Enter `/delete [id]`; the picker opens a y/n confirmation and does not
   delete before confirmation. A pending approval/question still consumes
   input before slash parsing.
9. Enter an unknown `/name`; a local error appears and no model turn starts.
10. `/quit`, `/exit`, and `/q` exit the face.

## Plain REPL

Run the legacy line REPL and repeat the parser, command, unknown-command, and
delete-confirmation checks. Its output is line-oriented, while command
classification and aliases remain identical to fullscreen.
