# Acceptance

- Reopening a converted channel chat returns the original session and history,
  including after its display title changes. Distinct topics and tuples with
  separator bytes cannot merge. Concurrent inbound messages use one session.
- Unbound channel history produces explicit unavailable/finite-conversion
  guidance and creates no replacement conversation.
- A converted observer cursor resumes after its stored sequence, keeps its
  version, and does not repeat completed entries after Host restart. Old cursor
  namespaces refuse startup before storage changes.
- Replaying a completed Run after memory upgrade returns the same new receipt
  and recalls the original record ID/revision once. Exact source IDs remain
  distinct; ambiguous historical sources fail. Concurrent allocations for the
  same source leave one record.
- The finite converter refuses unproved topic/provider bindings, preserves
  source copies, and publishes only a complete target 047 store.

Native owner tests exercise these behaviors on disposable stores. Final
copied-store and PostgreSQL acceptance belongs to the integrated D14 lane.
