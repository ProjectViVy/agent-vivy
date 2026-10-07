# A2A-00 acceptance notes

Story outcome (E0 / R7): evidence package plus adopted G0 contract.

- SDK seam proven with the official client on the pinned version; the
  forbidden paths (`NewHandler`, `AgentExecutor`, `TaskStore`, second
  queue) were never used and remain guard-checked.
- Native candidate-session contract is pinned to verified symbols;
  workspace loser cleanup reuses proven code, persona cleanup is a
  specified minimal laputa seam awaiting an owner-approved pin bump.
- G0 record: design section 1.1 table is now the authoritative adoption
  record; the exact issue #2 amendment text is in design section 13.

Open owner items carried forward:

1. Laputa pin bump for `personactx.Store.DiscardSession`/`ListFrozenSessions`
   and the `agentapi`/`cognitivecontract` discard surface (A2A-02.2
   dependency; the host-side raw-SQL alternative stays rejected).
2. G1 scheduling for A2A-01..06 — nothing functional is authorized yet.
3. Applying the amended reconnect criterion text to issue #2 itself
   (draft in design section 13; not yet posted).
