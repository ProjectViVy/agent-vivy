# Acceptance: remove chat preflight gate

How a human can tell it worked:

1. Open `http://127.0.0.1:3015` (split Vite dev) and send any chat message.
   The message starts processing immediately — the amber
   "Preflight found warnings" bar with Continue/Cancel buttons no
   longer appears between sending and the answer.
2. The empty new-session screen no longer shows the hint
   "After sending a message, Vivy runs a preflight check first."
3. Tool safety still works: a run that calls an approval-required tool (e.g.
   `write_note`) still raises the normal approval prompt at execution time —
   that gate lives in the tool adapter chain and was never part of the
   preflight.
4. `preflight/run` is gone from the control-plane RPC surface; API clients
   calling it now get an unknown-method error instead of a preview payload.
