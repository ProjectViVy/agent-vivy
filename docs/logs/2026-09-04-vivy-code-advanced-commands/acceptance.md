# Acceptance

A human can verify this delivery in `vivy-code`:

1. Enter `/help`; the advanced commands appear in the shared command list.
2. Enter `/stats`; an overlay shows the RPC payload labelled as aggregate
   usage, without claiming it is the active session's cost.
3. Enter `/skills` or `/mcp`; the overlay describes catalog/configured state,
   not session-mounted skills or a persistent live MCP connection.
4. Enter `/compact`, `/fork <message_id> [title]`, or
   `/rewind <message_id>`; the TUI asks for `y/n` before mutation. Denial makes
   no RPC mutation. Acceptance refreshes the current session or switches to
   the forked session.
5. While a run, gate, session load, or mutation is active, another mutating
   command is rejected locally.
6. Enter `!anything` or `@anything`; a local unavailable message appears and
   nothing is sent to the model or executed. Enter `!!anything` or
   `@@anything`; exactly one prefix is removed and the remainder is treated as
   ordinary model text.

All JSON shown by these commands is the corresponding control-plane response;
the TUI does not synthesize provider cost, loaded-skill, or connection state.
