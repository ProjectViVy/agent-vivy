# Crush-style TUI mock skeleton

Date: 2026-08-29
Status: complete (display-layer probe; not F3)

## Outcome

`vivy tui --demo` opens a fullscreen Bubble Tea shell with Crush-like regions
(header, session sidebar, chat, editor, status, approval overlay) driven only
by deterministic mock data. No gateway dial, no second kernel, no packed
`faces/tui`.

## Delivered

- `internal/tui/demo` — three-session script (过夜 / 审批中 / 空), y/n
  approval mutation, scripted enter reply `（demo：未接控制面）`.
- `internal/tui/view` — single Bubble Tea model + lipgloss layout/render.
- `cmd/vivy/tui.go` — `--demo` (default) / `--plain` / `--addr`.
- Charm deps: `bubbletea`, `lipgloss` (no Crush / `charm.land` fork).
- Board: `FACE-TUI-2` for wiring the real `Client` next.

## Geometry pass (same day)

Aligned the shell chrome with Crush chat layout (not a source port):

- **Wide:** chat+editor left, **sidebar right** (width 32), logo in sidebar —
  no top banner strip.
- **Compact:** `Vivy™ VIVY` + `╱` diagonals + session title header; no sidebar.
- **Editor:** Crush `::: ` success prompt; pending gate uses ` ! ` chip.
- **Messages:** left gutter bar `┃` (user blue / assistant violet).
- **Help:** bottom key+desc row (`tab` `enter` `y/n` `^n` `^c`).
- **Tools:** normal-border cards, pending/done/denied icons.

## How to try

```text
go run ./cmd/vivy tui --demo
```

If `cmd/vivy` still fails to compose on this tree because of unrelated
`ModelResolver` gaps, the demo path only needs packages under
`internal/tui` and the `tui` argv branch — build with a tree that compiles
`cmd/vivy`, or `go test ./internal/tui/...` for the skeleton contract.

## Explicitly not done

- Live control-plane data in the fullscreen shell (`FACE-TUI-2`).
- Packed `faces/tui` / FaceHost / omitting web (`FACE-TUI-1` / F3).
- LSP, MCP, model picker, attachments, Ultraviolet screen buffer.
