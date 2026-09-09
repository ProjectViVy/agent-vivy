# VC-1g-1: message queuing + two-stage cancellation

Date: 2026-08-31 · Branch: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`)

## Changes

Compared with Crush's composer behavior (see the "message queuing" entry in
`docs/research/crush-parity-code-agent-research-2026-08-31.md`), VC-1g was split into two deliveries:
this is **g1 (UI/store only, no backend changes)**; g2 (the image-attachment path) is delivered separately.

### Previous problems

- The input box was disabled while a run was active and sends were silently discarded: text entered while the agent was busy either waited or was lost.
- Cancellation had only one step: clicking/shortcut cancellation stopped the run immediately, without Crush's two-stage behavior of "clear the queue, then cancel."

### Current behavior

- **Continue typing and sending during a run**: the textarea is no longer disabled by `running`; clicking Send (or
  pressing Enter) during a run **queues** the message instead of discarding it. The queue path skips the UI preflight (the server gate still applies),
  matching Crush's queue semantics.
- **Queue pill**: the top of the composer shows "N queued" + a chip for each message (each can be removed individually) +
  "Clear queue."
- **Dispatch timing**: the queue dispatches the next item only after `run.completed` and terminal refresh (messages/context/background/approval/
  pending items) finish; `run.failed` / `run.cancelled` **retain** the queue for the user to handle.
- **Two-stage cancellation**: when a run is active and the queue is non-empty, the first press of Stop = clear the queue (the title becomes
  "Clear queue"), and the next press = cancel the run; Escape in the textarea works the same way.
- **Queue lifecycle**: the queue is cleared when switching sessions, reinitializing, or deleting a session (it belongs to the session and does not cross sessions).
- Defensive behavior: `startRun` no longer silently no-ops while busy; it queues the message (the caller's previous semantics).

### FSL compliance (Crush alignment)

Crush is FSL-1.1-MIT. This delivery provides **behavior/protocol alignment only, with zero code copying**:
the queue data structure, dispatch logic, and component implementation are all original to this project; only observable behavior is aligned
(queue while busy, queue pill, two-stage Esc behavior).

## Explicitly not done

- **g2 image attachments** (clipboard images / paste paths / @ completion / 5MB limit / SupportsImages gating /
  tool-result image workaround)—requires backend Message chunking, storage, and RPC parameter extensions, so it is delivered separately.
- The queue is not persisted (a refresh loses it, matching Crush's session-local behavior).
- The queue is not merged across sessions; Escape works only when the textarea is focused (no global shortcut).
