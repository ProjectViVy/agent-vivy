# Acceptance

Backend draft writes now require explicit CAS intent, rotate to a fresh opaque ETag on every accepted write, preserve `CreatedAt`, and retain the actual `UpdatedAt` even when time repeats or moves backwards. A stale or concurrent same-author create cannot replace the winning artifact; PostgreSQL reports the same typed conflict as SQLite.

Backend CAS is committed in `64a3196c`. P3.1 remains open until the workflow editor sends explicit create/edit intent, uses the current own-draft ETag when editing a published revision, and its source hash, browser tests, typecheck and product verification are complete.
