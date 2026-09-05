# TUI streamed tool-call identity fix

## Changed

- Replaced the runtime mapper's last-chunk assumption for streamed tool calls with Eino's native `schema.ConcatMessages` aggregation.
- A fragmented tool call now produces one complete `tool.requested` containing the authoritative call ID, tool name, and decoded arguments.
- Eino concatenation errors fail the run explicitly instead of committing an anonymous request that the TUI can never reconcile.
- Added a regression test where tool ID/name and JSON arguments arrive across three chunks, then verified the resulting `tool.finished` has the same identity.

## Not changed

- No TUI-side matching fallback or display heuristic was added.
- Tool execution, policy, approvals, provider routing, and view styling were not changed.
