# Acceptance

A human can tell this delivery works when:

1. A provider that emits only `model.completed` still shows its answer in both fullscreen TUI distributions and the plain REPL.
2. Normal streamed deltas followed by the same full completion render once, while an authoritative empty completion clears the fullscreen partial bubble.
3. A new model request, reasoning phase, tool, gate, or terminal event cannot cause a later completion to overwrite an earlier model round.
4. Assistant preamble text attached to a tool-call message remains visible in the durable event stream and cannot contaminate the model response after the tool.
5. Two concurrent calls of the same tool retain separate cards; each `tool.finished` or approval event updates the card with the matching `tool_call_id`.

Automated acceptance covers mapper boundaries, completed-only, streamed completion de-duplication, empty completion, completion mismatch visibility, model/reasoning round fences, history identity preservation, and built-in/packed wire paths.
