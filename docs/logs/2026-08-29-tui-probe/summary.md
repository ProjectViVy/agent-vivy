# TUI probe Summary

Date: 2026-08-29
Status: complete (probe; not F3)

## Outcome

A first-cut terminal mouth exists: `vivy tui` connects to a *resident*
gateway over the same JSON-RPC WebSocket the browser uses. It can create a
session, send a turn, stream `model.delta`, and answer an approval with
y/n. It does not start a second kernel, does not listen, and does not
replace the default web face.

## Delivered

- `internal/tui` — control-plane client + line REPL (`/help`, `/sessions`,
  `/new`, `/cancel`, `/quit`).
- `cmd/vivy/tui.go` — `vivy tui [--addr host:port] [--title name]`.
- `internal/rpc.Peer.Close` so the client can tear down the transport.
- Tests for event interpretation and a JSONL-pair turn that streams `hi`.
- Board: `FACE-TUI-1` (packed `faces/tui` still open). Proposal §5 notes
  this probe is not F3.

## How to try it

```text
terminal 1: just run          # gateway :8787
terminal 2: go run ./cmd/vivy tui
```

(`go run ./cmd/vivy` currently fails on this worktree if HEAD still lacks
`ModelResolver`; use a gateway built from a tree that composes, then point
`--addr` at it.)

## Explicitly not done

- Packed `faces/tui` / FaceHost / omitting `ui/dist`.
- Bubble Tea, settings TUI, Review Center in the terminal.
- In-process TUI (no HTTP) — still F1.
- Changing the default no-arg `vivy` from the web gateway.
