# Durable assistant projection

## Shipped

- `model.completed` v2 is a bounded commit record (`content_sha256`, `byte_len`); canonical assistant text is the preceding lossless `model.delta` sequence.
- Legacy v1 completion payloads remain readable and authoritative.
- The minimum event payload budget is now 128 bytes so the fixed v2 digest envelope always fits; supervised child workers use the same bounded delta/v2 contract and external events are size-checked.
- Journal replay now derives assistant preambles, tool calls/results, and final answers into MessageStore with deterministic identities and strict collision detection.
- Projection runs before live boundary publication, survives cancelled run contexts, repairs before model context construction, and repairs before `session/messages` returns history.
- Control-plane session deletion seals active producers before removing storage and is serialized with model, shell, child-worker, review/question, synthetic-compaction Journal writes, and projection. It cancels app-owned live child processes, retains fail-closed tombstones on partial deletion failures, and removes synthetic compaction events with their session.
- Built-in and packed TUI/headless consumers plus trajectory projection distinguish v1 and v2 explicitly and reject missing or mismatched v2 length/digest metadata.
- Live and crash-replayed model/child delta chunks follow one budget rule: transport framing does not consume semantic event slots, while completion still does.

## Explicitly not done

- The unrelated Studio submodule working-tree change was not touched.
- Browser support for historical v1 completion-only producers remains tracked as `UI-STREAM-N1`; current kernel v2 always emits canonical deltas.
