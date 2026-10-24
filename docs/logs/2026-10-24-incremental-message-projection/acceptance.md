# Acceptance

A user can confirm the fix by watching a run that issues several tool calls
in a row:

1. The tool row appears with its result as soon as `tool.finished` is
   published; the gap before it no longer grows with the length of the run or
   the session.
2. Tool-call latency stays flat across a long session (many earlier turns) and
   across many tool calls inside one turn.
3. The transcript after the run is unchanged: assistant preamble before a tool
   call, the tool-call row, the tool result, and the final answer appear in
   order, with no duplicate rows after a reload.
4. Cancelling a run and reloading the session shows the same persisted
   transcript as before the change.
