# Acceptance — UI-CHAT-ACT design slice

How a human can confirm it took effect:

1. Open `docs/architecture/JOURNAL-REWIND-AND-FORK.md`: §1 three-action semantics table, §2 truncation
   markers and folding, §4 RPC contract, §7 R1/R2/R3 split, and §8 three public questions are all present.
2. Design criteria (follow-up checkpoints):
   - Why not physically delete? → §1.2(1)(2): Journal append-only + messages/run_events as dual sources of truth.
   - Why can truncation save money without deleting rows? → §2.1 marker rows store only id; §2.3 truncated messages/events are retained for audit.
   - Why copy rather than reference on fork? → §3.3 the two sessions evolve independently, avoiding cross-session read-through.
   - Why not use child-run to express fork? → §1.2(5): restart without re-execution + different lifecycle structures.
3. `docs/TODO.md`: the UI-CHAT-ACT row shows the design slice delivered + implementation split, with row status OPEN (not implemented).
4. UI invariant: MessageBubble's edit/rewind/fork buttons remain disabled placeholders (zero code in this slice).
