# N0 — rollback

This change is a pure removal of compile-time injection plumbing plus a
digest re-pin; no schema, artifact, or data moves.

Rollback path: revert commit `refactor(notebook): remove automatic note
preamble injection`. Reverting restores `ServiceDeps.Notes`, the digest
helpers, and the App wiring exactly as they were, because nothing else in
the tree now depends on their absence. The conformance digest row must be
re-pinned again after any revert (`go run ./sdk/internal/cmd/source-hash internal ""`),
since the internal/ tree hash will change back.

No user action needed on revert: stored notes and saved messages were never
touched by this story.
