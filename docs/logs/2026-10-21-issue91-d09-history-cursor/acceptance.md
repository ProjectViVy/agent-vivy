# Human acceptance

1. Search a permitted source session, obtain another page, and restart the host.
   Continuing the same cursor keeps the original upper bound; later inserted
   records require a new search.
2. Revoke scope or delete/rewind the source before continuation. Removed or
   hidden content must not be returned. A changed/future/foreign cursor never
   expands the current server authority.
3. If a selection is too large, narrow the search/selected records and retry.
   The UI shows an actionable failure and retains earlier results/selections;
   it must not falsely report an empty history.
4. An old v1 cursor requires a new search. No legacy reader is retained.

These are reproducible acceptance instructions, not a claim that a human ran
them. Standard3015 interaction remains owner-deferred. D09 as a whole and D14
release/converter acceptance remain open in the sole Story index.
