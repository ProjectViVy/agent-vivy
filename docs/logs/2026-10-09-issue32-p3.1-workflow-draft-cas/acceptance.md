# Acceptance

Backend draft writes now require explicit CAS intent, rotate to a fresh opaque ETag on every accepted write, preserve `CreatedAt`, and retain the actual `UpdatedAt` even when time repeats or moves backwards. A stale or concurrent same-author create cannot replace the winning artifact; PostgreSQL reports the same typed conflict as SQLite.

Backend CAS is committed in `64a3196c`; the browser/editor continuation is committed separately. P3.1 is engineering-verified locally: explicit create/edit intent, current own-draft ETag when editing published revision content, same-author collision protection, passing Module source tests, reproducible source hash and staged assembly typecheck are recorded in `verification.md`.

The owner-level candidate browser acceptance, aggregate `just ci`, SDK/Port conformance and native product path remain P7 gates. Therefore P3.1 is implementation-complete but does not by itself close issue #32 or establish release acceptance.
