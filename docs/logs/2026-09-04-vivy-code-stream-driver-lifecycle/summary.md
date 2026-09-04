# VIVY CODE durable stream driver lifecycle

## Shipped

- Unified the built-in fullscreen TUI, packed fullscreen face, and plain REPL around durable run sequence cursors, gap detection, replay, duplicate suppression, and `run/stream_error` recovery.
- Preserved and used the control-plane `subscription_id`; replacement streams retire and unsubscribe old IDs, while request epochs reject late responses and old-stream events.
- Made Close, session switching, terminal completion, and face-host return clean up stream and peer resources. Built-in TUI exit now cancels and polls an active run just like the packed face.
- Replaced the unbounded fullscreen inbox with item and UTF-8 byte bounds. Overflow discards an incomplete suffix and recovers it from the Journal; each UI tick drains a bounded batch.
- Made the REPL notification path non-blocking. Channel saturation triggers replay from the last contiguous sequence. Replay subscription retries are bounded; persistent failure cancels the accepted run and returns control instead of wedging forever.

## Explicitly not done

- `model.completed.content` compatibility remains `TUI-STREAM-N3`; this delivery does not mix a projection-contract change into lifecycle work.
- Model/provider/cost/sidebar truth fields and split diff remain tracked by their existing parity rows.
- A subscribe response that is never delivered after `Live.Close` is ultimately cleaned by the owning JSON-RPC peer closing; the face-host lifecycle now explicitly guarantees that outer ownership boundary.
