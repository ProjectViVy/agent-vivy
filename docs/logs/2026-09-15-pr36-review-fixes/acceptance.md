# Acceptance — PR #36 review fixes

A human can accept this change when all of the following hold:

1. A channel Provider exposing outbound media through its capability target
   sends one media batch through the durable reply pipeline.
2. Starting with a pending delivery for an uninstalled or disabled channel does
   not panic, consume retries, or delete the row.
3. A run that completes before its start call returns still sends exactly one
   reply and leaves no open delivery row.
4. Deleting a session removes both pending and failed channel-delivery rows on
   SQLite and PostgreSQL.
5. The PR remains conflict-free with `main`; workspace and channel schema
   migrations both exist with distinct monotonic versions.
6. Plugin packages import neither `internal/*` nor Eino runtime types.
7. GitHub Actions reports success for backend CI, UI CI, the packed full-UI
   browser smoke, and the aggregate `just ci` job.
