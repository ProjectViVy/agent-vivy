# Verification

## Focused tests

- `go test ./sdk/tui/view ./internal/tui/view -count=1` — PASS.
- `cd faces/tui; go test ./view -count=1` — PASS.
- `go test -race ./sdk/tui/view ./internal/tui/view -count=1` — PASS.
- `cd faces/tui; go test -race ./view -count=1` — PASS.

Coverage includes `/` and `Ctrl+P` opening, draft preservation, alias filtering,
canonical selection, normal dispatch reuse, wrapped cursor movement, `//`
literal escaping, no-result rendering, backspace reset, gate/session/result
priority, asynchronous gate/result arrival, hostile ANSI/control paste,
128-rune input bounds, physical space, compact 32x10 rendering (including
footer and bottom border), and both face wrappers.

## Product gate and smoke

- Two GPT-5.6-LUNA MAX specialists reviewed the state machine and rendering.
  Their gate-priority, ANSI/control-injection, physical-space, dialog-width,
  and small-terminal findings were fixed; both final reviews reported no
  blocking findings.
- `just ci` — PASS, including UI typecheck/201 tests/build, Go vet, all
  main-module tests, headless checks, every plugin module, and packed TUI.
- `just vivy-code` — PASS.
- Real Windows PTY (`cmd.exe`, 80 columns) — PASS: footer displayed
  `^p commands`; Ctrl+P opened the palette; typing the physical-space query
  `run status` displayed that exact filter and selected `/status`; Esc closed
  it; a separately typed bare `/` reopened it; `attach` selected `/image`;
  Enter inserted `/image` into the editor; Ctrl+C exited with code 0.
