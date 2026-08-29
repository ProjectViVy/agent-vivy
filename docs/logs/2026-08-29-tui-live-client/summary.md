# TUI live client (FACE-TUI-2)

Date: 2026-08-29
Status: complete (display-layer live wire; not F3)

## Outcome

`vivy tui --live` opens the same Crush-style fullscreen shell as `--demo`,
but the data plane is a real control-plane `Client` against a resident
gateway. Sessions, turns, streaming deltas, tool cards, approval/question
overlays, and cancel all go through existing JSON-RPC methods.

## Delivered

- `internal/tui/surface` — shared `Driver` + DTO types (session / message /
  tool card / gate / meta).
- `internal/tui/demo.Store` — implements `surface.Driver` (mock path unchanged
  in product feel).
- `internal/tui.Live` — live driver: boot list/create, move/new session,
  send turn + subscribe, event pump → projection, approval/question/cancel.
- `internal/tui/view` — depends only on `surface.Driver`; live status line,
  streaming cursor, esc cancel, ctrl+j newline, question overlay answer.
- `cmd/vivy/tui.go` — `--live [--addr]`; dial failure exits non-zero (no
  silent demo fallback).
- Board: `FACE-TUI-2` DONE; follow-ups noted (scroll viewport, diff highlight,
  `/` command strip).
- Architecture probe blurb: `VIVY-FACE-PACK.md` documents `--live`.

## Explicitly not done

- Packed `faces/tui` / FaceHost / omit web (`FACE-TUI-1` / F3).
- Crush Ultraviolet, allow-for-session, model picker, completions, attachments,
  Glamour markdown, todo pills, sidebar files/LSP/MCP.
- Fixing pre-existing `cmd/vivy` compose gaps on this tip (`ModelResolver`,
  empty `ui/dist`) — those are main WIP outside this lane.
