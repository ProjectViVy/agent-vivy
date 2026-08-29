# Acceptance — tui-live-client

## Product checks

1. `vivy tui --demo` still opens the offline Crush-style mock (no dial).
2. `vivy tui --live` against a running gateway:
   - sidebar lists sessions (or creates one if empty)
   - enter sends a turn; assistant text streams into the chat
   - tool approval shows a center overlay; `y` / `n` calls `approval/respond`
   - `esc` while busy calls `run/cancel`
   - tab / ↑↓ (empty editor) switches sessions and loads history
   - `^n` creates a session
   - status/help footer shows `live · host` not `mock · not connected`
3. If the gateway is down, `--live` prints an error and exits non-zero
   (does not silently open demo).
4. Default no-arg `vivy` remains the web gateway; this face is opt-in argv.
5. Same Journal remains authoritative — live TUI is another control-plane
   client, not a second kernel.

## Automated stand-in

When `cmd/vivy` cannot be built on the tip, `go test ./internal/tui/...`
covers boot, streaming projection, approval RPC, run-id filtering, and
session switch message load via a fake JSON-RPC peer.
