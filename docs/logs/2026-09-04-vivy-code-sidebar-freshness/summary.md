# VIVY CODE sidebar freshness

## Changed

- Refresh the active session context after every terminal run in both the
  built-in and packed TUI drivers. Token, message, and compaction facts no
  longer remain at their boot-time snapshot after a turn completes.
- Fence asynchronous context refreshes by request generation and session ID,
  so a late response from the previous session cannot overwrite the newly
  selected session.
- Fence permission mutations by their originating session and a monotonic
  request generation; late or out-of-order permission responses cannot mutate
  whichever session happens to be active when they arrive.
- Refresh context after the fail-safe cancellation used when an initial stream
  subscription cannot be established. A failed context refresh hides the stale
  snapshot and reports the problem instead of presenting old facts as current.
- Continue queued turns only after `run.completed`; failed and cancelled runs
  keep their follow-up turns queued for explicit user action. Queue counts in
  the active-session rail exclude turns belonging to other sessions.
- Keep the active sidebar session projection synchronized after permission
  updates, and preserve the pre-existing session list when `--prompt` creates
  a fresh boot session.
- Render only existing authoritative facts: active run ID, queued turn count,
  feed/total message counts, compaction threshold, and summary availability.
  The right rail remains about the active session; the session collection stays
  in its dedicated picker.

## Explicitly not done

- No process cwd, creation timestamp, or aggregate/global statistic is relabeled
  as session/workspace truth.
- Updated time, effective provider/model, per-session cost, workspace git state,
  modified files, LSP health, and mounted MCP/skill state still require new
  authoritative control-plane projections and remain tracked by
  `TUI-SIDEBAR-N1`.
